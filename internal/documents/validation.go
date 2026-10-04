package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"society.local/portal/internal/database"
)

// Check originals, never render them into the portal or extract/OCR their contents.
// qpdf is an optional system dependency: unavailable PDF checks fail closed.
func ValidateOriginal(ctx context.Context, filename string, data []byte) (string, string) {
	if len(data) == 0 || len(data) > database.MaxUploadBytes {
		return "", "FILE_TOO_LARGE"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".pdf" {
		if len(data) > 4*1024*1024 || !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(data), []byte("%%EOF")) {
			return "", "INVALID_PDF"
		}
		if code := validatePDF(ctx, data); code != "" {
			return "", code
		}
		return "application/pdf", ""
	}
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return "", "UNSUPPORTED_TYPE"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "INVALID_IMAGE"
	}
	if (ext == ".png" && format != "png") || (ext != ".png" && format != "jpeg") {
		return "", "TYPE_MISMATCH"
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 8192 || cfg.Height > 8192 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return "", "IMAGE_TOO_LARGE"
	}
	if ctx.Err() != nil {
		return "", "CHECK_CANCELLED"
	}
	// Header-only recognition is insufficient: a truncated image cannot be available.
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", "INVALID_IMAGE"
	}
	if ctx.Err() != nil {
		return "", "CHECK_CANCELLED"
	}
	if format == "png" {
		return "image/png", ""
	}
	return "image/jpeg", ""
}

type cappedOutput struct {
	buf   bytes.Buffer
	limit int
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	if len(p) > w.limit-w.buf.Len() {
		return 0, errors.New("validation output limit")
	}
	return w.buf.Write(p)
}
func validatePDF(ctx context.Context, data []byte) string {
	executable, err := exec.LookPath("qpdf")
	if err != nil {
		return "CHECKS_UNAVAILABLE"
	}
	folder, err := os.MkdirTemp("", "society-pdf-check-")
	if err != nil {
		return "CHECKS_UNAVAILABLE"
	}
	defer os.RemoveAll(folder)
	path := filepath.Join(folder, "original.pdf")
	if err = os.WriteFile(path, data, 0600); err != nil {
		return "CHECKS_UNAVAILABLE"
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, executable, "--global", "--parser-max-nesting=64", "--parser-max-errors=1", "--parser-max-container-size=5000", "--parser-max-container-size-damaged=5000", "--max-stream-filters=3", "--", "--suppress-recovery", "--json=2", "--decode-level=none", "--json-stream-data=none", "--json-key=qpdf", "--json-key=pages", "--json-key=encrypt", path)
	out := &cappedOutput{limit: 8 * 1024 * 1024}
	stderr := &cappedOutput{limit: 8192}
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second
	if err = cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return "PDF_CHECK_LIMIT"
		}
		return "INVALID_PDF"
	}
	var result struct {
		Pages   []json.RawMessage `json:"pages"`
		Encrypt struct {
			Encrypted bool `json:"encrypted"`
		} `json:"encrypt"`
		Objects []any `json:"qpdf"`
	}
	if err = json.Unmarshal(out.buf.Bytes(), &result); err != nil || len(result.Pages) < 1 {
		return "INVALID_PDF"
	}
	if result.Encrypt.Encrypted {
		return "ENCRYPTED_PDF"
	}
	if len(result.Pages) > 250 {
		return "PDF_CHECK_LIMIT"
	}
	nodes := 0
	var walk func(any, int) bool
	forbidden := map[string]bool{"/JS": true, "/JavaScript": true, "/Launch": true, "/OpenAction": true, "/AA": true, "/AcroForm": true, "/XFA": true, "/EmbeddedFiles": true, "/EmbeddedFile": true, "/Filespec": true, "/EF": true, "/RichMedia": true, "/3D": true, "/Movie": true, "/Sound": true, "/GoToR": true, "/SubmitForm": true, "/ImportData": true, "/URI": true, "/Encrypt": true}
	walk = func(value any, depth int) bool {
		nodes++
		if nodes > 50000 || depth > 64 {
			return false
		}
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if forbidden[key] || !walk(item, depth+1) {
					return false
				}
			}
			if v["/Subtype"] == "/Image" {
				w, _ := v["/Width"].(float64)
				h, _ := v["/Height"].(float64)
				if w < 1 || h < 1 || w > 8192 || h > 8192 || w*h > 8000000 {
					return false
				}
			}
		case []any:
			for _, item := range v {
				if !walk(item, depth+1) {
					return false
				}
			}
		case string:
			if forbidden[v] {
				return false
			}
		}
		return true
	}
	if !walk(result.Objects, 0) {
		return "UNSUPPORTED_PDF_CONTENT"
	}
	return ""
}
func RunValidation(ctx context.Context, db *database.Store, logger *slog.Logger) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := db.ClaimDocumentCheck(ctx, time.Now())
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("document_check_claim_failed")
				}
				continue
			}
			sum := sha256.Sum256(job.Bytes)
			contentType, code := "", "CHECKSUM_MISMATCH"
			if hex.EncodeToString(sum[:]) == job.SHA256 {
				contentType, code = ValidateOriginal(ctx, job.Filename, job.Bytes)
			}
			if ctx.Err() != nil {
				return
			}
			if err = db.FinishDocumentCheck(ctx, job, contentType, code, time.Now()); err != nil {
				logger.Warn("document_check_completion_failed")
			}
		}
	}
}

// Compile-time interface assertion documents that output is bounded.
var _ io.Writer = (*cappedOutput)(nil)
