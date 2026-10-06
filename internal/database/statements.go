package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

type StatementInput struct {
	OperationKey string `json:"operation_key"`
	Title        string `json:"title"`
	Kind         string `json:"kind"`
	PeriodStart  string `json:"period_start"`
	PeriodEnd    string `json:"period_end"`
	PreparedBy   string `json:"prepared_by"`
	Source       string `json:"source"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	Replaces     string `json:"replaces_id"`
	Version      int    `json:"version"`
	Confirmed    bool   `json:"confirmed"`
	Reason       string `json:"reason"`
}
type StatementFile struct {
	ID             string `json:"id"`
	GroupID        string `json:"-"`
	Replaces       string `json:"replaces_id"`
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	PreparedBy     string `json:"prepared_by"`
	Source         string `json:"source"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size_bytes"`
	SHA256         string `json:"sha256"`
	ContentType    string `json:"content_type"`
	UploaderID     string `json:"uploader_id"`
	CreatedAt      int64  `json:"created_at"`
	ExpiresAt      int64  `json:"expires_at"`
	UploadedAt     int64  `json:"uploaded_at"`
	AvailableAt    int64  `json:"available_at"`
	Validation     string `json:"validation"`
	ValidationCode string `json:"validation_code"`
	State          string `json:"state"`
	ReviewerID     string `json:"reviewer_id"`
	ReviewedAt     int64  `json:"reviewed_at"`
	Revision       int    `json:"revision"`
	Version        int    `json:"version"`
	Current        bool   `json:"current"`
	PublicationID  string `json:"publication_id"`
	CanUpload      bool   `json:"can_upload"`
	CanReview      bool   `json:"can_review"`
	CanWithdraw    bool   `json:"can_withdraw"`
	CanRetry       bool   `json:"can_retry"`
	CanReplace     bool   `json:"can_replace"`
	CanPublish     bool   `json:"can_publish"`
	CanDownload    bool   `json:"can_download"`
}
type StatementAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type StatementEvent struct {
	Action  string `json:"action"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
	At      int64  `json:"at"`
	Version int    `json:"version"`
}
type StatementPage struct {
	Items      []StatementFile `json:"items"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	CanPrepare bool            `json:"can_prepare"`
}

const statementFileSelect = `SELECT x.id,x.group_id,COALESCE(x.replaces_id,''),x.title,x.kind,x.period_start,x.period_end,x.prepared_by,x.source,x.filename,x.size_bytes,x.sha256,x.content_type,x.uploaded_by,x.created_at,x.expires_at,x.uploaded_at,x.available_at,x.validation,x.validation_code,x.state,COALESCE(x.reviewed_by,''),x.reviewed_at,x.revision,x.version,COALESCE(g.current_file_id=x.id,0),COALESCE((SELECT sp.id FROM statement_publications sp WHERE sp.id=g.current_publication_id AND sp.file_id=x.id AND sp.state='PUBLISHED'),'') FROM statement_files x JOIN statement_groups g ON g.id=x.group_id`

func scanStatement(row interface{ Scan(...any) error }) (StatementFile, error) {
	var x StatementFile
	err := row.Scan(&x.ID, &x.GroupID, &x.Replaces, &x.Title, &x.Kind, &x.PeriodStart, &x.PeriodEnd, &x.PreparedBy, &x.Source, &x.Filename, &x.Size, &x.SHA256, &x.ContentType, &x.UploaderID, &x.CreatedAt, &x.ExpiresAt, &x.UploadedAt, &x.AvailableAt, &x.Validation, &x.ValidationCode, &x.State, &x.ReviewerID, &x.ReviewedAt, &x.Revision, &x.Version, &x.Current, &x.PublicationID)
	return x, err
}
func statementStaff(p Principal) bool { return p.CanManageRecords && !p.MFAPending }
func statementAuthority(p Principal) error {
	if !statementStaff(p) {
		return ErrForbidden
	}
	if !p.Fresh {
		return ErrReauthRequired
	}
	return nil
}
func validateStatementInput(in StatementInput) error {
	if !in.Confirmed || !validText(in.Reason, 5, 300) || !validText(in.Title, 5, 120) || !validText(in.PreparedBy, 2, 120) || !validText(in.Source, 5, 300) {
		return invalid("Confirm the supplied statement and give its title, preparer, source and reason.")
	}
	if in.Kind != "INCOME" && in.Kind != "BALANCE" && in.Kind != "BUDGET" && in.Kind != "AUDIT" {
		return ErrInvalid
	}
	if !validMaintenanceDate(in.PeriodStart) || !validMaintenanceDate(in.PeriodEnd) || in.PeriodEnd < in.PeriodStart {
		return invalid("Choose an explicit valid statement period.")
	}
	if !validText(in.Filename, 1, 150) || filepath.Base(in.Filename) != in.Filename || strings.ContainsAny(in.Filename, "/\\\";:") {
		return ErrInvalid
	}
	ext := strings.ToLower(filepath.Ext(in.Filename))
	if ext != ".pdf" && ext != ".xlsx" && ext != ".csv" {
		return invalid("Choose a plain PDF, XLSX or UTF-8 CSV original.")
	}
	if in.Size < 1 || in.Size > 10*1024*1024 || ((ext == ".pdf" || ext == ".csv") && in.Size > 4*1024*1024) || !checksumPattern.MatchString(in.SHA256) {
		return invalid("Use a nonempty original up to 10 MiB (PDF/CSV 4 MiB) and its checksum.")
	}
	return nil
}
func statementFileEvent(ctx context.Context, tx *sql.Tx, actor string, x StatementFile, action, reason string, at int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO statement_events VALUES(?,?,?,?,?,?,?)`, randomToken(), x.ID, nullableText(actor), action, reason, at, x.Version)
	if err != nil || actor == "" {
		return err
	}
	return appendAudit(ctx, tx, actor, "", "STATEMENT_"+action, "Prepared statement workflow", map[string]any{}, map[string]any{"id": x.ID, "version": x.Version, "state": x.State})
}

func (s *Store) ReserveStatement(ctx context.Context, token string, in StatementInput) (string, error) {
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = statementAuthority(p); err != nil {
		return "", err
	}
	if err = validateStatementInput(in); err != nil {
		return "", err
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "STATEMENT_RESERVE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	group := randomToken()
	if in.Replaces != "" {
		parent, e := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", in.Replaces))
		if e != nil {
			return "", e
		}
		if parent.Version != in.Version || parent.State != "APPROVED" || !parent.Current {
			return "", ErrConflict
		}
		if parent.Kind != in.Kind || parent.PeriodStart != in.PeriodStart || parent.PeriodEnd != in.PeriodEnd {
			return "", invalid("A replacement retains its statement type and period.")
		}
		group = parent.GroupID
		var pending bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM statement_files WHERE group_id=? AND state='PENDING' AND validation IN('PENDING','VALIDATING','AVAILABLE'))`, group).Scan(&pending); err != nil {
			return "", err
		}
		if pending {
			return "", ErrConflict
		}
	} else {
		if in.Version != 0 {
			return "", ErrInvalid
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO statement_groups(id) VALUES(?)`, group); err != nil {
			return "", err
		}
	}
	own, total, err := documentStorageUsage(ctx, tx, p.ID)
	if err != nil {
		return "", err
	}
	if own+in.Size > LocalUserDocumentQuota || total+in.Size > LocalSocietyDocumentQuota {
		return "", invalid("The shared local original-file allowance is full.")
	}
	id, now := randomToken(), time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO statement_files(id,group_id,replaces_id,title,kind,period_start,period_end,prepared_by,source,filename,size_bytes,sha256,uploaded_by,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, group, nullableText(in.Replaces), in.Title, in.Kind, in.PeriodStart, in.PeriodEnd, in.PreparedBy, in.Source, in.Filename, in.Size, in.SHA256, p.ID, now, now+86400)
	if err != nil {
		return "", err
	}
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", id))
	if err != nil {
		return "", err
	}
	if err = statementFileEvent(ctx, tx, p.ID, x, "RESERVED", in.Reason, now); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) CompleteStatement(ctx context.Context, token, id string, data []byte) error {
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = statementAuthority(p); err != nil {
		return err
	}
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=? AND x.uploaded_by=?", id, p.ID))
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != x.Size || hex.EncodeToString(sum[:]) != x.SHA256 {
		return invalid("Choose the unchanged original matching this reserved size and checksum.")
	}
	if x.UploadedAt > 0 {
		return tx.Commit()
	}
	if x.State != "PENDING" || x.Validation != "PENDING" || x.ExpiresAt <= time.Now().Unix() {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO statement_objects VALUES(?,?)`, id, data); err != nil {
		return err
	}
	now := time.Now().Unix()
	if _, err = tx.ExecContext(ctx, `UPDATE statement_files SET uploaded_at=?,version=version+1 WHERE id=?`, now, id); err != nil {
		return err
	}
	x.Version++
	if err = statementFileEvent(ctx, tx, p.ID, x, "UPLOADED", "Original received; independent content checks pending", now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DecideStatement(ctx context.Context, token, id string, in StatementAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !validText(in.Reason, 5, 300) || (in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "RETRY_VALIDATION") {
		return "", ErrInvalid
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = statementAuthority(p); err != nil {
		return "", err
	}
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", id))
	if err != nil {
		return "", err
	}
	if in.Action == "WITHDRAWN" {
		if x.UploaderID != p.ID {
			return "", ErrForbidden
		}
	} else if in.Action != "RETRY_VALIDATION" && x.UploaderID == p.ID {
		return "", ErrForbidden
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "STATEMENT_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version || x.State != "PENDING" {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	if in.Action == "RETRY_VALIDATION" {
		if x.Validation != "REJECTED" || x.ValidationCode != "CHECKS_UNAVAILABLE" {
			return "", ErrConflict
		}
		_, err = tx.ExecContext(ctx, `UPDATE statement_files SET validation='PENDING',validation_code='',attempts=0,lease_until=0,lease_token='',version=version+1 WHERE id=?`, id)
	} else {
		if in.Action == "APPROVED" {
			if x.Validation != "AVAILABLE" {
				return "", ErrConflict
			}
			var head string
			if err = tx.QueryRowContext(ctx, `SELECT COALESCE(current_file_id,'') FROM statement_groups WHERE id=?`, x.GroupID).Scan(&head); err != nil {
				return "", err
			}
			if head != x.Replaces {
				return "", ErrConflict
			}
			if head != "" {
				if err = tx.QueryRowContext(ctx, `SELECT revision+1 FROM statement_files WHERE id=?`, head).Scan(&x.Revision); err != nil {
					return "", err
				}
			} else {
				x.Revision = 1
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE statement_files SET state=?,reviewed_by=?,reviewed_at=?,revision=?,version=version+1 WHERE id=?`, in.Action, p.ID, now, x.Revision, id)
		if err == nil && in.Action == "APPROVED" {
			_, err = tx.ExecContext(ctx, `UPDATE statement_groups SET current_file_id=? WHERE id=?`, id, x.GroupID)
		}
		x.State = in.Action
	}
	if err != nil {
		return "", err
	}
	x.Version++
	if err = statementFileEvent(ctx, tx, p.ID, x, in.Action, in.Reason, now); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) DownloadStatement(ctx context.Context, token, id string) (StatementFile, []byte, error) {
	// Record server access to the original, not a claim that a client saved it.
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return StatementFile{}, nil, err
	}
	defer tx.Rollback()
	if p.MFAPending {
		return StatementFile{}, nil, ErrForbidden
	}
	scope, args := statementScope(p)
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=:id AND "+scope, append(args, sql.Named("id", id))...))
	if err != nil {
		return x, nil, err
	}
	if x.Validation != "AVAILABLE" {
		return x, nil, ErrConflict
	}
	var data []byte
	if err = tx.QueryRowContext(ctx, `SELECT original_bytes FROM statement_objects WHERE file_id=?`, id).Scan(&data); err != nil {
		return x, nil, err
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != x.Size || hex.EncodeToString(sum[:]) != x.SHA256 {
		return x, nil, errors.New("statement original integrity mismatch")
	}
	publication := ""
	if !statementStaff(p) {
		publication = x.PublicationID
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO statement_accesses VALUES(?,?,?,?,?,?,?)`, randomToken(), id, nullableText(publication), p.ID, time.Now().Unix(), x.Size, x.SHA256); err != nil {
		return x, nil, err
	}
	if err = statementCapabilities(ctx, tx, p, &x); err != nil {
		return x, nil, err
	}
	return x, data, tx.Commit()
}
