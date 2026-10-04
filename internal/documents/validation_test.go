package documents

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os/exec"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestOriginalValidationInspectsActualContentAndRejectsTruncationAndOversizedPixels(t *testing.T) {
	ctx := context.Background()
	var out bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 9, 9))
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	valid := append([]byte(nil), out.Bytes()...)
	if kind, code := ValidateOriginal(ctx, "अभिलेख.png", valid); kind != "image/png" || code != "" {
		t.Fatal("valid image", kind, code)
	}
	if _, code := ValidateOriginal(ctx, "fake.jpg", valid); code != "TYPE_MISMATCH" {
		t.Fatal("extension defeated actual type", code)
	}
	if _, code := ValidateOriginal(ctx, "broken.png", valid[:len(valid)/2]); code != "INVALID_IMAGE" {
		t.Fatal("truncated image", code)
	}
	if _, code := ValidateOriginal(ctx, "report.svg", []byte(`<svg onload="alert(1)"></svg>`)); code != "UNSUPPORTED_TYPE" {
		t.Fatal("active image type allowed", code)
	}
	huge := image.NewGray(image.Rect(0, 0, 8193, 1))
	out.Reset()
	if err := png.Encode(&out, huge); err != nil {
		t.Fatal(err)
	}
	if _, code := ValidateOriginal(ctx, "too-wide.png", out.Bytes()); code != "IMAGE_TOO_LARGE" {
		t.Fatal("pixel cap", code)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, code := ValidateOriginal(canceled, "cancel.png", valid); code != "CHECK_CANCELLED" {
		t.Fatal("cancellation", code)
	}
}

func TestPDFValidationUsesRealQpdfStructureAndActiveContentInspection(t *testing.T) {
	if _, err := exec.LookPath("qpdf"); err != nil {
		t.Fatal("Install qpdf 12.4+ to run original PDF validation tests")
	}
	ctx := context.Background()
	data, err := Render(database.ReceiptSnapshot{Number: "TEST-2026-000001", Home: "A-101", AmountPaise: 10000, Description: "Fictional document validation sample", Payer: "Demo Resident", Method: "CASH", Date: "2026-10-04", Operator: "Demo Operator"})
	if err != nil {
		t.Fatal(err)
	}
	if kind, code := ValidateOriginal(ctx, "fictional-record.pdf", data); kind != "application/pdf" || code != "" {
		t.Fatal("real PDF rejected", kind, code)
	}
	if _, code := ValidateOriginal(ctx, "fake.pdf", []byte("%PDF-1.7\nnot a PDF\n%%EOF")); code != "INVALID_PDF" {
		t.Fatal("magic alone accepted", code)
	}
	if _, code := ValidateOriginal(ctx, "unfinished.pdf", data[:len(data)-100]); code != "INVALID_PDF" {
		t.Fatal("truncated PDF", code)
	}
	// Generate a structurally valid PDF with an escaped JavaScript name, not a mere text marker.
	objects := []string{"<< /Type /Catalog /Pages 2 0 R /OpenAction 4 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << >> >>", "<< /S /J#61vaScript /J#53 (app.alert\\(1\\)) >>"}
	active := minimalPDF(objects)
	if _, code := ValidateOriginal(ctx, "active.pdf", active); code != "UNSUPPORTED_PDF_CONTENT" {
		t.Fatal("escaped active dictionary accepted", code)
	}
	t.Setenv("PATH", t.TempDir())
	if _, code := ValidateOriginal(ctx, "unavailable.pdf", data); code != "CHECKS_UNAVAILABLE" {
		t.Fatal("unavailable checks allowed PDF", code)
	}
}
func minimalPDF(objects []string) []byte {
	var out strings.Builder
	out.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	start := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return []byte(out.String())
}
