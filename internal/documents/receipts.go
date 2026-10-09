package documents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/signintech/gopdf"
	"society.local/portal/internal/database"
)

// DM Sans, Google Fonts; redistribution license is alongside the embedded font.
//
//go:embed assets/DMSans-Regular.ttf
var receiptFont []byte

type Store struct{ root *os.Root }

var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const maxDocument = 2 * 1024 * 1024

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("document root must be a real private directory")
	}
	if err = os.Chmod(path, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}
func (s *Store) Close() error { return s.root.Close() }
func (s *Store) Read(hash string) ([]byte, error) {
	if !hashPattern.MatchString(hash) {
		return nil, errors.New("invalid document identity")
	}
	file, err := s.root.Open(hash + ".pdf")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxDocument {
		return nil, errors.New("invalid private document")
	}
	bytes, err := io.ReadAll(io.LimitReader(file, maxDocument+1))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(bytes)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, errors.New("document integrity mismatch")
	}
	return bytes, nil
}
func (s *Store) Put(bytes []byte) (string, error) {
	if len(bytes) > maxDocument || len(bytes) < 5 || string(bytes[:5]) != "%PDF-" {
		return "", errors.New("invalid receipt PDF")
	}
	sum := sha256.Sum256(bytes)
	hash := hex.EncodeToString(sum[:])
	if _, err := s.Read(hash); err == nil {
		return hash, nil
	}
	name := fmt.Sprintf(".receipt-%d-%s.tmp", time.Now().UnixNano(), hash[:12])
	file, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer s.root.Remove(name)
	if _, err = file.Write(bytes); err != nil {
		file.Close()
		return "", err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	if err = s.root.Rename(name, hash+".pdf"); err != nil {
		return "", err
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return "", err
	}
	defer directory.Close()
	if err = directory.Sync(); err != nil {
		return "", err
	}
	return hash, nil
}

func Render(r database.ReceiptSnapshot) ([]byte, error) {
	if r.Issuer != nil && (r.Issuer.FormatVersion != 1 || utf8.RuneCountInString(r.Issuer.Name) < 2 || utf8.RuneCountInString(r.Issuer.Name) > 120 || strings.ContainsAny(r.Issuer.Name, "\r\n\t") || (r.Issuer.Mode != "FICTIONAL_REHEARSAL" && r.Issuer.Mode != "LOCAL_WORKSPACE")) {
		return nil, errors.New("unsupported receipt issuer snapshot")
	}
	var pdf gopdf.GoPdf
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := pdf.AddTTFFontData("DM", receiptFont); err != nil {
		return nil, err
	}
	pdf.AddPage()
	pdf.SetFillColor(248, 247, 240)
	if err := pdf.Rectangle(0, 0, 595.28, 841.89, "F", 0, 0); err != nil {
		return nil, err
	}
	pdf.SetFillColor(230, 234, 219)
	if err := pdf.Rectangle(38, 180, 557, 288, "F", 12, 10); err != nil {
		return nil, err
	}
	pdf.SetTextColor(29, 61, 50)
	var renderErr error
	text := func(x, y, size float64, value string) {
		if renderErr != nil {
			return
		}
		if err := pdf.SetFont("DM", "", size); err != nil {
			renderErr = err
			return
		}
		pdf.SetXY(x, y)
		renderErr = pdf.CellWithOption(&gopdf.Rect{W: 510, H: size + 7}, value, gopdf.CellOption{})
	}
	var issuerLines []string
	var issuerSize float64
	if r.Issuer == nil {
		text(40, 40, 29, "society.")
		text(40, 89, 10, "FICTIONAL COMMUNITY  /  LOCAL PREVIEW")
	} else {
		// Wrap the frozen name within the reserved header, including maximum-length words.
		var lines []string
		var size float64
		for _, candidate := range []float64{22, 18, 14, 12} {
			if err := pdf.SetFont("DM", "", candidate); err != nil {
				return nil, err
			}
			var err error
			lines, err = pdf.SplitTextWithOption(r.Issuer.Name, 510, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
			if err != nil {
				return nil, err
			}
			if float64(len(lines))*(candidate+6) <= 66 {
				size = candidate
				break
			}
		}
		if size == 0 {
			return nil, errors.New("receipt issuer does not fit the supported header")
		}
		issuerLines, issuerSize = lines, size
		for i, line := range lines {
			text(40, 30+float64(i)*(size+6), size, line)
		}
		marker := "LOCAL COMMUNITY WORKSPACE"
		if r.Issuer.Mode == "FICTIONAL_REHEARSAL" {
			marker = "FICTIONAL REHEARSAL  /  SUPPLIED SAMPLE REGISTER"
		}
		text(40, 99, 9, marker)
	}
	text(40, 124, 24, "Receipt of money received")
	text(55, 196, 10, "AMOUNT ALREADY RECEIVED")
	text(55, 224, 34, fmt.Sprintf("₹%d.%02d", r.AmountPaise/100, r.AmountPaise%100))
	text(40, 315, 10, "RECEIPT NUMBER")
	text(40, 336, 14, r.Number)
	text(345, 315, 10, "HOME")
	text(345, 336, 14, r.Home)
	footer := func() {
		pdf.SetStrokeColor(211, 218, 202)
		pdf.SetLineWidth(.6)
		pdf.Line(40, 773, 555, 773)
		text(40, 785, 9, "Generated from a confirmed manual entry. Payments happen outside this portal.")
		text(40, 803, 9, "Preview format. Confirm the society's receipt policy before production use.")
	}
	y := 390.0
	field := func(label, value string) {
		if renderErr != nil {
			return
		}
		if err := pdf.SetFont("DM", "", 12); err != nil {
			renderErr = err
			return
		}
		lines, err := pdf.SplitTextWithOption(value, 510, &gopdf.BreakOption{Mode: gopdf.BreakModeIndicatorSensitive, BreakIndicator: ' '})
		if err != nil {
			renderErr = err
			return
		}
		height := 36 + float64(len(lines))*20
		if y+height > 750 {
			footer()
			pdf.AddPage()
			pdf.SetFillColor(248, 247, 240)
			if err = pdf.Rectangle(0, 0, 595.28, 841.89, "F", 0, 0); err != nil {
				renderErr = err
				return
			}
			if r.Issuer == nil {
				text(40, 40, 29, "society.")
				text(40, 90, 14, r.Number+" · details continued")
			} else {
				for i, line := range issuerLines {
					text(40, 30+float64(i)*(issuerSize+6), issuerSize, line)
				}
				text(40, 110, 14, r.Number+" · details continued")
			}
			y = 150
		}
		text(40, y, 10, label)
		y += 22
		for _, line := range lines {
			text(40, y, 12, line)
			y += 20
		}
		y += 14
	}
	field("RECEIVED FROM", r.Payer)
	field("FOR", r.Description)
	method := map[string]string{"CASH": "Cash", "BANK_TRANSFER": "Bank transfer", "UPI": "UPI", "CHEQUE": "Cheque"}[r.Method]
	field("DATE  /  METHOD", r.Date+"  ·  "+method)
	if r.Reference != "" {
		field("REFERENCE", r.Reference)
	}
	if r.SourceNote != "" {
		field("SOURCE / EVIDENCE NOTE", r.SourceNote)
	}
	field("RECORDED BY", r.Operator)
	footer()
	pdf.SetInfo(gopdf.PdfInfo{Title: r.Number, Creator: "Society OS", CreationDate: time.Unix(r.IssuedAt, 0).UTC()})
	if renderErr != nil {
		return nil, renderErr
	}
	return pdf.GetBytesPdfReturnErr()
}

// A missing/corrupt PDF can be regenerated from its immutable database snapshot.
// Database snapshots therefore retain the receipt identity even without derived files.
func (s *Store) Reconcile(ctx context.Context, db *database.Store) error {
	rows, err := db.DB.QueryContext(ctx, "SELECT receipt_id,file_hash FROM receipt_jobs WHERE state='READY'")
	if err != nil {
		return err
	}
	type missing struct{ id, hash string }
	items := []missing{}
	for rows.Next() {
		var m missing
		if err = rows.Scan(&m.id, &m.hash); err != nil {
			rows.Close()
			return err
		}
		items = append(items, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, m := range items {
		if _, err = s.Read(m.hash); err != nil {
			if _, err = db.DB.ExecContext(ctx, "UPDATE receipt_jobs SET state='PENDING',attempts=0,available_at=?,file_hash='' WHERE receipt_id=? AND state='READY' AND file_hash=?", time.Now().Unix(), m.id, m.hash); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) Run(ctx context.Context, db *database.Store, logger *slog.Logger) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := db.ClaimReceipt(ctx, time.Now())
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				if ctx.Err() == nil {
					logger.Error("receipt_claim_failed")
				}
				continue
			}
			bytes, err := Render(job.Snapshot)
			hash := ""
			if err == nil {
				hash, err = s.Put(bytes)
			}
			if finishErr := db.FinishReceipt(ctx, job, hash, err == nil, time.Now()); finishErr != nil && ctx.Err() == nil {
				logger.Error("receipt_completion_failed")
			}
			if err != nil && ctx.Err() == nil {
				logger.Error("receipt_render_failed")
			}
		}
	}
}
func DefaultPath(dbPath string) string { return filepath.Join(filepath.Dir(dbPath), "documents") }
