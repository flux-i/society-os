package documents

import (
	"bytes"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"society.local/portal/internal/database"
)

func TestReceiptPDFUnicodeAmountPrivateFilesIntegrityAndTraversal(t *testing.T) {
	receipt := database.ReceiptSnapshot{Number: "SOS-2026-000001", Home: "A-101", Payer: "Demo Owner A-101", AmountPaise: 40001, Date: "2026-01-01", Description: "Fictional maintenance already paid", Method: "BANK_TRANSFER", Reference: "DEMO-101", Operator: "Demo Registry Officer", IssuedAt: 1767225600}
	pdf, err := Render(receipt)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("PDF render", err)
	}
	root := filepath.Join(t.TempDir(), "documents")
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := store.Put(pdf)
	if err != nil {
		t.Fatal(err)
	}
	read, err := store.Read(hash)
	if err != nil || !bytes.Equal(read, pdf) {
		t.Fatal("private read", err)
	}
	for _, path := range []string{root, filepath.Join(root, hash+".pdf")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if info.IsDir() {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Fatal("public private file", info.Mode())
		}
	}
	for _, value := range []string{"../society.db", "../../keys/mfa.key", "", strings.Repeat("g", 64)} {
		if _, err := store.Read(value); err == nil {
			t.Fatal("accepted traversal", value)
		}
	}
	if err = os.WriteFile(filepath.Join(root, hash+".pdf"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(hash); err == nil {
		t.Fatal("tampered file accepted")
	}
	// Regeneration replaces only the derived blob, preserving the immutable receipt identity.
	if _, err = store.Put(pdf); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Read(hash); err != nil {
		t.Fatal(err)
	}
	receipt.Payer = strings.Repeat("W", 120)
	receipt.Description = strings.Repeat("W", 300)
	receipt.Reference = strings.Repeat("W", 120)
	receipt.Operator = strings.Repeat("W", 120)
	long, err := Render(receipt)
	if err != nil || len(long) == 0 {
		t.Fatal("maximum-length fields require safe wrapping/pages", err)
	}
}

func TestConfiguredReceiptMaximumFieldsPreserveWholeIssuerOnEveryPhysicalPage(t *testing.T) {
	extractor, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("PDF geometry extraction needs the developer's optional pdftotext tool")
	}
	r := database.ReceiptSnapshot{Issuer: &database.ReceiptIssuer{FormatVersion: 1, Name: strings.Repeat("W", 120), Mode: "FICTIONAL_REHEARSAL"}, Number: "SOS-2026-000001", Home: "A-101", Payer: strings.Repeat("W", 120), AmountPaise: 40000, Date: "2026-10-09", Description: strings.Repeat("W", 300), Method: "BANK_TRANSFER", Reference: strings.Repeat("W", 120), SourceNote: strings.Repeat("W", 300), Operator: strings.Repeat("W", 120), IssuedAt: 1791520000}
	bytes, err := Render(r)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if value := os.Getenv("SOCIETY_PDF_REVIEW_DIR"); value != "" {
		if !strings.Contains(value, "reports/local/") {
			t.Fatal("Private PDF review directory required")
		}
		dir = value
		if err = os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "maximum-configured-receipt.pdf")
	if err = os.WriteFile(path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	xmlBytes, err := exec.Command(extractor, "-bbox", path, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Pages []struct {
			Width  float64 `xml:"width,attr"`
			Height float64 `xml:"height,attr"`
			Words  []struct {
				XMin float64 `xml:"xMin,attr"`
				XMax float64 `xml:"xMax,attr"`
				YMin float64 `xml:"yMin,attr"`
				YMax float64 `xml:"yMax,attr"`
				Text string  `xml:",chardata"`
			} `xml:"word"`
		} `xml:"body>doc>page"`
	}
	if err = xml.Unmarshal(xmlBytes, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Pages) < 2 {
		t.Fatal("Maximum fields did not exercise continuation pages")
	}
	for i, page := range document.Pages {
		var name strings.Builder
		for _, word := range page.Words {
			if word.XMin < 0 || word.XMax > page.Width || word.YMin < 0 || word.YMax > page.Height {
				t.Fatal("PDF word exceeds its physical page", i, word)
			}
			if word.YMin < 90 && strings.Trim(word.Text, "W") == "" {
				name.WriteString(word.Text)
			}
		}
		if name.String() != r.Issuer.Name {
			t.Fatal("Whole frozen issuer lost on continuation page", i, len(name.String()))
		}
	}
}

func TestReceiptIssuerActualPDFTextModesWrappingAndLegacyFormat(t *testing.T) {
	extractor, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("PDF text extraction needs the developer's optional pdftotext tool")
	}
	base := database.ReceiptSnapshot{Number: "SOS-2026-000001", Home: "A-101", Payer: "Sample Owner A 101", AmountPaise: 40000, Date: "2026-10-09", Description: "Supplied money already received", Method: "BANK_TRANSFER", Operator: "Sample Treasury Officer", IssuedAt: 1791520000}
	for _, tc := range []struct {
		name             string
		issuer           *database.ReceiptIssuer
		required, absent string
	}{
		{"legacy", nil, "FICTIONAL COMMUNITY / LOCAL PREVIEW", "SUPPLIED SAMPLE REGISTER"},
		{"rehearsal", &database.ReceiptIssuer{FormatVersion: 1, Name: "The Neighbourhood Rehearsal", Mode: "FICTIONAL_REHEARSAL"}, "FICTIONAL REHEARSAL / SUPPLIED SAMPLE REGISTER", "FICTIONAL COMMUNITY"},
		{"local", &database.ReceiptIssuer{FormatVersion: 1, Name: "Verified Community Workspace", Mode: "LOCAL_WORKSPACE"}, "LOCAL COMMUNITY WORKSPACE", "FICTIONAL"},
		{"maximum", &database.ReceiptIssuer{FormatVersion: 1, Name: strings.Repeat("W", 120), Mode: "FICTIONAL_REHEARSAL"}, "FICTIONAL REHEARSAL / SUPPLIED SAMPLE REGISTER", "FICTIONAL COMMUNITY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := base
			r.Issuer = tc.issuer
			pdf, err := Render(r)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "receipt.pdf")
			if err = os.WriteFile(path, pdf, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(extractor, "-layout", path, "-").Output()
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(strings.Fields(string(out)), " ")
			for _, want := range []string{tc.required, "₹400.00", "SOS-2026-000001", "A-101", "Sample Owner A 101"} {
				if !strings.Contains(text, want) {
					t.Fatal("independent PDF text missing", want, text)
				}
			}
			if strings.Contains(text, tc.absent) {
				t.Fatal("incorrect issuer marker", text)
			}
			if tc.issuer != nil && !strings.Contains(strings.Join(strings.Fields(text), ""), strings.ReplaceAll(tc.issuer.Name, " ", "")) {
				t.Fatal("frozen issuer name missing", text)
			}
		})
	}
	r := base
	r.Issuer = &database.ReceiptIssuer{FormatVersion: 2, Name: "Wrong format", Mode: "FICTIONAL_REHEARSAL"}
	if _, err := Render(r); err == nil {
		t.Fatal("unsupported frozen version silently relabelled")
	}
}
