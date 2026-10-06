package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"image"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"time"
)

const MaxIncidentPictureBytes = 5 * 1024 * 1024

type IncidentPicture struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	MediaType     string `json:"media_type"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	SHA256        string `json:"sha256"`
	PreviewSHA256 string `json:"preview_sha256"`
	CreatedAt     int64  `json:"created_at"`
	Bytes         []byte `json:"-"`
}

func incidentPictureBytes(filename string, data []byte) (IncidentPicture, []byte, error) {
	var x IncidentPicture
	if len(data) == 0 || len(data) > MaxIncidentPictureBytes || !validText(filename, 1, 160) || strings.ContainsAny(filename, "/\\") {
		return x, nil, invalid("Choose a PNG or JPEG picture no larger than 5 MiB.")
	}
	cfg, kind, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (kind != "png" && kind != "jpeg") {
		return x, nil, invalid("This picture could not be read. Choose an original PNG or JPEG file.")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if (kind == "png" && ext != ".png") || (kind == "jpeg" && ext != ".jpg" && ext != ".jpeg") {
		return x, nil, invalid("The picture's actual type must match its PNG or JPEG filename.")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 8000000 {
		return x, nil, invalid("Use a picture within 4,096 pixels per side and eight million pixels. Resize it and try again.")
	}
	decoded, actual, e := image.Decode(bytes.NewReader(data))
	if e != nil || actual != kind || decoded.Bounds().Dx() != cfg.Width || decoded.Bounds().Dy() != cfg.Height {
		return x, nil, invalid("This picture is damaged. Replace it with a readable original.")
	}
	w, h := cfg.Width, cfg.Height
	if w > 1280 || h > 1280 {
		if w >= h {
			h = h * 1280 / w
			w = 1280
		} else {
			w = w * 1280 / h
			h = 1280
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
	}
	preview := image.NewNRGBA(image.Rect(0, 0, w, h))
	bounds := decoded.Bounds()
	for y := 0; y < h; y++ {
		for px := 0; px < w; px++ {
			preview.Set(px, y, decoded.At(bounds.Min.X+px*cfg.Width/w, bounds.Min.Y+y*cfg.Height/h))
		}
	}
	var b bytes.Buffer
	if kind == "png" {
		e = png.Encode(&b, preview)
		x.MediaType = "image/png"
	} else {
		e = jpeg.Encode(&b, preview, &jpeg.Options{Quality: 85})
		x.MediaType = "image/jpeg"
	}
	if e != nil {
		return x, nil, e
	}
	hash := sha256.Sum256(data)
	previewHash := sha256.Sum256(b.Bytes())
	x.Filename = filename
	x.Width = cfg.Width
	x.Height = cfg.Height
	x.SHA256 = hex.EncodeToString(hash[:])
	x.PreviewSHA256 = hex.EncodeToString(previewHash[:])
	return x, b.Bytes(), nil
}
func documentStorageUsage(ctx context.Context, q identityReader, actor string) (int64, int64, error) {
	var own, total int64
	e := q.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN uploaded_by=? THEN expected_size_bytes ELSE 0 END),0),COALESCE(SUM(expected_size_bytes),0) FROM library_documents WHERE uploaded_at IS NOT NULL OR (review_state='PENDING' AND validation_status='PENDING' AND expires_at>?)`, actor, time.Now().Unix()).Scan(&own, &total)
	if e != nil {
		return 0, 0, e
	}
	var exists bool
	if e = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='incident_pictures')`).Scan(&exists); e != nil {
		return 0, 0, e
	}
	if exists {
		var picOwn, picTotal int64
		e = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN uploader_id=? THEN length(original_bytes)+length(preview_bytes) ELSE 0 END),0),COALESCE(SUM(length(original_bytes)+length(preview_bytes)),0) FROM incident_pictures`, actor).Scan(&picOwn, &picTotal)
		own += picOwn
		total += picTotal
	}
	if e != nil {
		return 0, 0, e
	}
	if e = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='statement_files')`).Scan(&exists); e != nil {
		return 0, 0, e
	}
	if exists {
		var statementOwn, statementTotal int64
		e = q.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN uploaded_by=? THEN size_bytes ELSE 0 END),0),COALESCE(SUM(size_bytes),0) FROM statement_files WHERE uploaded_at>0 OR (state='PENDING' AND validation='PENDING' AND expires_at>?)`, actor, time.Now().Unix()).Scan(&statementOwn, &statementTotal)
		own += statementOwn
		total += statementTotal
	}
	return own, total, e
}
func (s *Store) UploadIncidentPicture(ctx context.Context, token, key, filename string, data []byte) (IncidentPicture, error) {
	var out IncidentPicture
	read, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	yes, e := incidentParticipant(ctx, read, p)
	read.Rollback()
	if e != nil {
		return out, e
	}
	if !yes {
		return out, ErrForbidden
	}
	meta, preview, e := incidentPictureBytes(filename, data)
	if e != nil {
		return out, e
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	yes, e = incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	if !yes {
		return out, ErrForbidden
	}
	input := struct{ Filename, SHA256 string }{filename, meta.SHA256}
	result, hash, e := replayOperation(ctx, tx, p, key, "INCIDENT_PICTURE", input)
	if e != nil {
		return out, e
	}
	if result != "" {
		out, e = incidentPictureMetadata(ctx, tx, result)
		if e != nil {
			return out, e
		}
		return out, tx.Commit()
	}
	own, total, e := documentStorageUsage(ctx, tx, p.ID)
	if e != nil {
		return out, e
	}
	size := int64(len(data) + len(preview))
	if own+size > LocalUserDocumentQuota || total+size > LocalSocietyDocumentQuota {
		return out, invalid("The private evidence storage allowance is full. Ask the records custodian to review retention.")
	}
	meta.ID = randomToken()
	meta.CreatedAt = time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO incident_pictures VALUES(?,?,?,?,?,?,?,?,?,?,?)`, meta.ID, p.ID, filename, meta.MediaType, meta.Width, meta.Height, meta.SHA256, data, meta.PreviewSHA256, preview, meta.CreatedAt)
	if e != nil {
		return out, e
	}
	if e = appendAudit(ctx, tx, p.ID, "", "INCIDENT_PICTURE", "Private validated picture retained.", nil, map[string]any{"id": meta.ID, "sha256": meta.SHA256}); e != nil {
		return out, e
	}
	if e = saveOperation(ctx, tx, p, key, hash, meta.ID); e != nil {
		return out, e
	}
	return meta, tx.Commit()
}
func incidentPictureMetadata(ctx context.Context, q identityReader, id string) (IncidentPicture, error) {
	var x IncidentPicture
	e := q.QueryRowContext(ctx, `SELECT id,filename,media_type,width,height,original_sha256,preview_sha256,created_at FROM incident_pictures WHERE id=?`, id).Scan(&x.ID, &x.Filename, &x.MediaType, &x.Width, &x.Height, &x.SHA256, &x.PreviewSHA256, &x.CreatedAt)
	return x, e
}
func (s *Store) IncidentPictureFor(ctx context.Context, token, id string, original bool) (IncidentPicture, error) {
	var out IncidentPicture
	// Reserve the writer before checking current scope so the access record and
	// authorised bytes belong to the same transaction.
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope := "1=1"
	args := []any{}
	if !p.CanHandleComplaints {
		yes, e := incidentParticipant(ctx, tx, p)
		if e != nil {
			return out, e
		}
		if !yes {
			return out, sql.ErrNoRows
		}
		scope = "x.uploader_id=?"
		args = append(args, p.ID)
		if !original {
			scope += ` OR EXISTS(SELECT 1 FROM incident_notices n JOIN incidents c ON c.notice_id=n.id JOIN flat_memberships m ON m.flat_id=n.flat_id WHERE n.picture_id=x.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
			args = append(args, p.ResidentID, today(), today())
		}
	}
	column := "preview_bytes"
	if original {
		column = "original_bytes"
	}
	e = tx.QueryRowContext(ctx, `SELECT x.id,x.filename,x.media_type,x.width,x.height,x.original_sha256,x.preview_sha256,x.created_at,x.`+column+` FROM incident_pictures x WHERE x.id=? AND (`+scope+")", append([]any{id}, args...)...).Scan(&out.ID, &out.Filename, &out.MediaType, &out.Width, &out.Height, &out.SHA256, &out.PreviewSHA256, &out.CreatedAt, &out.Bytes)
	if e != nil {
		return out, e
	}
	want := out.PreviewSHA256
	if original {
		want = out.SHA256
	}
	sum := sha256.Sum256(out.Bytes)
	if hex.EncodeToString(sum[:]) != want {
		return IncidentPicture{}, ErrConflict
	}
	action, reason := "INCIDENT_PICTURE_PREVIEW_READ", "Permitted picture preview opened."
	if original {
		action, reason = "INCIDENT_PICTURE_ORIGINAL_READ", "Private original opened."
	}
	if e = appendAudit(ctx, tx, p.ID, "", action, reason, nil, map[string]any{"id": id, "sha256": want}); e != nil {
		return IncidentPicture{}, e
	}
	if !p.CanHandleComplaints && !original {
		out.Filename = "incident-preview"
		out.SHA256 = ""
		out.Width = 0
		out.Height = 0
		out.CreatedAt = 0
	}
	return out, tx.Commit()
}
