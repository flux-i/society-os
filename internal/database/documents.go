package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const MaxUploadBytes = 20 * 1024 * 1024
const LocalUserDocumentQuota = 100 * 1024 * 1024
const LocalSocietyDocumentQuota = 1024 * 1024 * 1024
const financialDocumentCategories = "('RECEIPT','INVOICE','PAYMENT_EVIDENCE','ACCOUNTING_EXPORT')"

var checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type DocumentInput struct {
	OperationKey string `json:"operation_key"`
	Title        string `json:"title"`
	Filename     string `json:"filename"`
	Size         int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	Category     string `json:"category"`
	Visibility   string `json:"visibility"`
	FlatID       string `json:"flat_id"`
	SubjectID    string `json:"subject_resident_id"`
	Expiry       string `json:"expiry_date"`
	Replaces     string `json:"replaces_document_id"`
	Version      int    `json:"version"`
}
type DocumentAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type LibraryDocument struct {
	ID              string            `json:"id"`
	GroupID         string            `json:"-"`
	Replaces        string            `json:"replaces_document_id"`
	Title           string            `json:"title"`
	Filename        string            `json:"filename"`
	Size            int64             `json:"size_bytes"`
	SHA256          string            `json:"sha256"`
	Type            string            `json:"content_type"`
	Category        string            `json:"category"`
	Visibility      string            `json:"visibility"`
	FlatID          string            `json:"flat_id"`
	Home            string            `json:"home"`
	SubjectID       string            `json:"subject_resident_id"`
	Subject         string            `json:"subject"`
	Expiry          string            `json:"expiry_date"`
	AuthorID        string            `json:"author_id"`
	Author          string            `json:"author"`
	CreatedAt       int64             `json:"created_at"`
	ExpiresAt       int64             `json:"expires_at"`
	Validation      string            `json:"validation_status"`
	ValidationError string            `json:"validation_error_code"`
	State           string            `json:"review_state"`
	Revision        int               `json:"revision"`
	Version         int               `json:"version"`
	Current         bool              `json:"current"`
	Uploaded        bool              `json:"uploaded"`
	CanUpload       bool              `json:"can_upload"`
	CanReplace      bool              `json:"can_replace"`
	CanReview       bool              `json:"can_review"`
	CanWithdraw     bool              `json:"can_withdraw"`
	CanArchive      bool              `json:"can_archive"`
	CanRetry        bool              `json:"can_retry_validation"`
	Events          []DocumentEvent   `json:"events,omitempty"`
	Versions        []LibraryDocument `json:"versions,omitempty"`
	HistoryTotal    int               `json:"history_total"`
	HistoryPage     int               `json:"history_page"`
}
type DocumentEvent struct {
	Action  string `json:"action"`
	Reason  string `json:"reason"`
	Actor   string `json:"actor"`
	At      int64  `json:"at"`
	Version int    `json:"version"`
}
type DocumentPage struct {
	Items     []LibraryDocument `json:"items"`
	Total     int               `json:"total"`
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
	Homes     []RecordHome      `json:"homes"`
	CanUpload bool              `json:"can_upload"`
}

const librarySelect = `SELECT x.id,x.group_id,COALESCE(x.replaces_document_id,''),x.title,x.original_filename,x.expected_size_bytes,x.verified_sha256,x.detected_content_type,x.category,x.visibility,COALESCE(x.flat_id,''),COALESCE(b.code||'-'||f.flat_number,''),COALESCE(x.subject_resident_id,''),COALESCE(r.full_name,''),COALESCE(x.expiry_date,''),x.uploaded_by,u.display_name,x.created_at,x.expires_at,x.validation_status,x.validation_error_code,x.review_state,x.revision,x.version,COALESCE(g.current_document_id=x.id,0),x.uploaded_at IS NOT NULL FROM library_documents x JOIN document_groups g ON g.id=x.group_id JOIN users u ON u.id=x.uploaded_by LEFT JOIN flats f ON f.id=x.flat_id LEFT JOIN buildings b ON b.id=f.building_id LEFT JOIN residents r ON r.id=x.subject_resident_id`

func scanDocument(row interface{ Scan(...any) error }) (LibraryDocument, error) {
	var x LibraryDocument
	err := row.Scan(&x.ID, &x.GroupID, &x.Replaces, &x.Title, &x.Filename, &x.Size, &x.SHA256, &x.Type, &x.Category, &x.Visibility, &x.FlatID, &x.Home, &x.SubjectID, &x.Subject, &x.Expiry, &x.AuthorID, &x.Author, &x.CreatedAt, &x.ExpiresAt, &x.Validation, &x.ValidationError, &x.State, &x.Revision, &x.Version, &x.Current, &x.Uploaded)
	return x, err
}
func financialDocument(category string) bool {
	switch category {
	case "RECEIPT", "INVOICE", "PAYMENT_EVIDENCE", "ACCOUNTING_EXPORT":
		return true
	}
	return false
}
func documentReviewer(p Principal, category string) bool {
	if financialDocument(category) {
		return p.CanReadAllRecords
	}
	return p.CanManageDocuments
}
func nullableText(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func documentScope(p Principal) (string, []any) {
	// Pending filenames/versions are visible only to the uploader/current eligible reviewers.
	reviewer := `((x.category IN ` + financialDocumentCategories + ` AND :review_finance=1) OR (x.category NOT IN ` + financialDocumentCategories + ` AND :review_docs=1))`
	active := `EXISTS(SELECT 1 FROM flat_memberships a WHERE a.resident_id=:resident AND a.start_date<=:day AND (a.end_date IS NULL OR a.end_date>:day))`
	home := `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.resident_id=:resident AND m.flat_id=x.flat_id AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day) AND (x.category NOT IN ` + financialDocumentCategories + ` OR m.can_view_finances=1))`
	author := `(x.uploaded_by=:actor AND ((x.category NOT IN ` + financialDocumentCategories + ` AND ((x.visibility='ALL_AUTHORIZED_RESIDENTS' AND ` + active + `) OR (x.visibility='FLAT_SPECIFIC' AND ` + home + `) OR (x.visibility='RESIDENT_SPECIFIC' AND x.subject_resident_id=:resident AND (` + active + ` OR x.review_state='APPROVED')))) OR (x.category IN ` + financialDocumentCategories + ` AND (` + home + ` OR :review_finance=1))))`
	audience := `(x.review_state='APPROVED' AND g.archived=0 AND ((x.visibility='ALL_AUTHORIZED_RESIDENTS' AND ` + active + `) OR (x.visibility='FLAT_SPECIFIC' AND ` + home + `) OR (x.visibility='RESIDENT_SPECIFIC' AND x.subject_resident_id=:resident AND :resident!='') OR (x.visibility='ACCOUNTING_ONLY' AND :review_finance=1) OR (x.visibility='COMMITTEE_ONLY' AND :review_docs=1)))`
	return "(" + reviewer + " OR " + author + " OR " + audience + ")", []any{sql.Named("actor", p.ID), sql.Named("resident", p.ResidentID), sql.Named("day", today()), sql.Named("review_docs", p.CanManageDocuments), sql.Named("review_finance", p.CanReadAllRecords)}
}
func currentDocumentResident(ctx context.Context, q identityReader, p Principal) (bool, error) {
	var active bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, p.ResidentID, today(), today()).Scan(&active)
	return active, err
}
func canPrepareDocument(ctx context.Context, q identityReader, p Principal, in DocumentInput) error {
	fin := financialDocument(in.Category)
	staff := documentReviewer(p, in.Category)
	active, err := currentDocumentResident(ctx, q, p)
	if err != nil {
		return err
	}
	if !staff && !active {
		return ErrForbidden
	}
	if fin && !staff && in.Category != "PAYMENT_EVIDENCE" {
		return ErrForbidden
	}
	if in.Visibility == "ACCOUNTING_ONLY" && !p.CanReadAllRecords {
		return ErrForbidden
	}
	if in.Visibility == "COMMITTEE_ONLY" && !p.CanManageDocuments {
		return ErrForbidden
	}
	if in.Visibility == "RESIDENT_SPECIFIC" {
		if !staff && in.SubjectID != p.ResidentID {
			return sql.ErrNoRows
		}
		var exists bool
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, in.SubjectID, today(), today()).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return sql.ErrNoRows
		}
	}
	if in.FlatID != "" {
		var allowed bool
		err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flats f WHERE f.id=? AND (? OR EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?) AND (? OR m.can_view_finances=1))))`, in.FlatID, staff, p.ResidentID, today(), today(), !fin).Scan(&allowed)
		if err != nil {
			return err
		}
		if !allowed {
			return sql.ErrNoRows
		}
	}
	return nil
}
func documentAsInput(x LibraryDocument) DocumentInput {
	return DocumentInput{Title: x.Title, Filename: x.Filename, Size: x.Size, SHA256: x.SHA256, Category: x.Category, Visibility: x.Visibility, FlatID: x.FlatID, SubjectID: x.SubjectID, Expiry: x.Expiry}
}
func documentPreparationAllowed(ctx context.Context, q identityReader, p Principal, x LibraryDocument) (bool, error) {
	err := canPrepareDocument(ctx, q, p, documentAsInput(x))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrForbidden) || errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}
func documentCapabilities(ctx context.Context, q identityReader, p Principal, x *LibraryDocument) error {
	allowed, err := documentPreparationAllowed(ctx, q, p, *x)
	if err != nil {
		return err
	}
	reviewer := documentReviewer(p, x.Category)
	x.CanUpload = x.AuthorID == p.ID && allowed && !x.Uploaded && x.Validation == "PENDING" && x.State == "PENDING" && x.ExpiresAt > time.Now().Unix()
	x.CanReplace = allowed && x.State == "APPROVED" && x.Current && (reviewer || x.AuthorID == p.ID)
	x.CanReview = reviewer && x.AuthorID != p.ID && x.State == "PENDING"
	x.CanWithdraw = x.AuthorID == p.ID && allowed && x.State == "PENDING"
	x.CanArchive = x.State == "APPROVED" && x.Current && (reviewer || (x.AuthorID == p.ID && x.Visibility == "RESIDENT_SPECIFIC" && allowed))
	x.CanRetry = allowed && (reviewer || x.AuthorID == p.ID) && x.State == "PENDING" && x.Validation == "REJECTED" && x.ValidationError == "CHECKS_UNAVAILABLE"
	return nil
}
func validateDocumentInput(in DocumentInput) error {
	if !validText(in.Title, 5, 120) || !validText(in.Filename, 1, 150) || strings.ContainsAny(in.Filename, "/\\\";:") || filepath.Base(in.Filename) != in.Filename || in.Filename == "." || in.Filename == ".." {
		return invalid("Use a title of 5–120 characters and a simple filename without paths or quotes.")
	}
	ext := strings.ToLower(filepath.Ext(in.Filename))
	if ext != ".pdf" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return invalid("Choose a PDF, PNG or JPEG file.")
	}
	if in.Size < 1 || in.Size > MaxUploadBytes || (ext == ".pdf" && in.Size > 4*1024*1024) || !checksumPattern.MatchString(in.SHA256) {
		return invalid("Use a nonempty file up to 20 MiB (PDF up to 4 MiB), with its SHA-256 checksum.")
	}
	switch in.Category {
	case "RECEIPT", "INVOICE", "NOTICE", "AGM_MINUTES", "AUDIT_REPORT", "CONTRACT", "AMC", "LEGAL", "CIRCULAR", "RESIDENT_DOCUMENT", "COMMITTEE_DOCUMENT", "PAYMENT_EVIDENCE", "ACCOUNTING_EXPORT":
	default:
		return ErrInvalid
	}
	switch in.Visibility {
	case "RESIDENT_SPECIFIC":
		if in.SubjectID == "" || in.FlatID != "" {
			return ErrInvalid
		}
	case "FLAT_SPECIFIC":
		if in.FlatID == "" || in.SubjectID != "" {
			return ErrInvalid
		}
	case "ALL_AUTHORIZED_RESIDENTS", "COMMITTEE_ONLY", "ACCOUNTING_ONLY":
		if in.FlatID != "" || in.SubjectID != "" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if financialDocument(in.Category) && in.Visibility != "FLAT_SPECIFIC" && in.Visibility != "ACCOUNTING_ONLY" {
		return ErrInvalid
	}
	if in.Category == "PAYMENT_EVIDENCE" && in.Visibility != "FLAT_SPECIFIC" {
		return ErrInvalid
	}
	if in.Category == "COMMITTEE_DOCUMENT" && in.Visibility != "COMMITTEE_ONLY" {
		return ErrInvalid
	}
	parsedExpiry, expiryErr := time.Parse("2006-01-02", in.Expiry)
	if in.Expiry != "" && ((in.Category != "CONTRACT" && in.Category != "AMC") || expiryErr != nil || parsedExpiry.Format("2006-01-02") != in.Expiry || in.Expiry < "1900-01-01" || in.Expiry > "2100-12-31") {
		return invalid("An expiry date applies only to a contract or maintenance agreement.")
	}
	return nil
}
func appendDocumentEvent(ctx context.Context, tx *sql.Tx, pID string, x LibraryDocument, action, reason string, audit bool) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO document_events VALUES(?,?,?,?,?,?,?)`, randomToken(), x.ID, nullableText(pID), action, reason, time.Now().Unix(), x.Version)
	if err != nil {
		return err
	}
	if audit {
		return appendAudit(ctx, tx, pID, "", "DOCUMENT_"+action, reason, map[string]any{}, map[string]any{"id": x.ID, "version": x.Version, "state": x.State})
	}
	return nil
}
func (s *Store) ReserveDocument(ctx context.Context, token string, in DocumentInput) (string, error) {
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if in.Visibility == "RESIDENT_SPECIFIC" && in.SubjectID == "" {
		in.SubjectID = p.ResidentID
	}
	if err = validateDocumentInput(in); err != nil {
		return "", err
	}
	if err = canPrepareDocument(ctx, tx, p, in); err != nil {
		return "", err
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "DOCUMENT_RESERVE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	group := randomToken()
	if in.Replaces != "" {
		scope, args := documentScope(p)
		parent, e := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=:target AND "+scope, append(args, sql.Named("target", in.Replaces))...))
		if e != nil {
			return "", e
		}
		if parent.Version != in.Version || parent.State != "APPROVED" || !parent.Current {
			return "", ErrConflict
		}
		if !documentReviewer(p, parent.Category) && parent.AuthorID != p.ID {
			return "", ErrForbidden
		}
		if parent.Category != in.Category || parent.Visibility != in.Visibility || parent.FlatID != in.FlatID || parent.SubjectID != in.SubjectID {
			return "", invalid("A replacement preserves its category, audience and subject.")
		}
		group = parent.GroupID
		var pending bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM library_documents WHERE group_id=? AND review_state='PENDING' AND validation_status IN('PENDING','VALIDATING','AVAILABLE'))`, group).Scan(&pending)
		if e != nil {
			return "", e
		}
		if pending {
			return "", ErrConflict
		}
	} else {
		if in.Version != 0 {
			return "", ErrInvalid
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO document_groups(id) VALUES(?)`, group); err != nil {
			return "", err
		}
	}
	// Original bytes remain retained; only expired/withdrawn reservations without bytes release quota.
	var own, total int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN uploaded_by=? THEN expected_size_bytes ELSE 0 END),0),COALESCE(SUM(expected_size_bytes),0) FROM library_documents WHERE uploaded_at IS NOT NULL OR (review_state='PENDING' AND validation_status='PENDING' AND expires_at>?)`, p.ID, time.Now().Unix()).Scan(&own, &total)
	if err != nil {
		return "", err
	}
	if own+in.Size > LocalUserDocumentQuota || total+in.Size > LocalSocietyDocumentQuota {
		return "", invalid("The local document storage allowance is full. Ask the records custodian to review retention.")
	}
	id := randomToken()
	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO library_documents(id,group_id,replaces_document_id,title,original_filename,expected_size_bytes,verified_sha256,category,visibility,flat_id,subject_resident_id,expiry_date,uploaded_by,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, group, nullableText(in.Replaces), in.Title, in.Filename, in.Size, in.SHA256, in.Category, in.Visibility, nullableText(in.FlatID), nullableText(in.SubjectID), nullableText(in.Expiry), p.ID, now, now+86400)
	if err != nil {
		return "", err
	}
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=?", id))
	if err != nil {
		return "", err
	}
	if err = appendDocumentEvent(ctx, tx, p.ID, x, "RESERVED", "Upload reserved for separate document review", true); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) CompleteDocument(ctx context.Context, token, id string, bytes []byte) error {
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=? AND x.uploaded_by=?", id, p.ID))
	if err != nil {
		return err
	}
	if err = canPrepareDocument(ctx, tx, p, documentAsInput(x)); err != nil {
		return err
	}
	sum := sha256.Sum256(bytes)
	if int64(len(bytes)) != x.Size || hex.EncodeToString(sum[:]) != x.SHA256 {
		return invalid("The uploaded file does not match its reserved size and checksum. Choose the original file again.")
	}
	if x.Uploaded {
		return tx.Commit()
	}
	if x.ExpiresAt <= time.Now().Unix() || x.State != "PENDING" || x.Validation != "PENDING" {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO local_document_objects VALUES(?,?)`, id, bytes); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE library_documents SET uploaded_at=?,version=version+1 WHERE id=?`, time.Now().Unix(), id); err != nil {
		return err
	}
	x.Version++
	x.Uploaded = true
	if err = appendDocumentEvent(ctx, tx, p.ID, x, "UPLOADED", "Original file received; content checks pending", true); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) DocumentFor(ctx context.Context, token, id string, historyPage int) (LibraryDocument, error) {
	if historyPage < 1 || historyPage > 10000 {
		return LibraryDocument{}, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return LibraryDocument{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return LibraryDocument{}, err
	}
	scope, args := documentScope(p)
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=:target AND "+scope, append(args, sql.Named("target", id))...))
	if err != nil {
		return x, err
	}
	if err = documentCapabilities(ctx, tx, p, &x); err != nil {
		return x, err
	}
	if x.AuthorID == p.ID || documentReviewer(p, x.Category) {
		rows, e := tx.QueryContext(ctx, `SELECT e.action,e.reason,COALESCE(u.display_name,'File checks'),e.occurred_at,e.version FROM document_events e LEFT JOIN users u ON u.id=e.actor_id WHERE document_id=? ORDER BY version DESC LIMIT 30`, id)
		if e != nil {
			return x, e
		}
		x.Events = []DocumentEvent{}
		for rows.Next() {
			var event DocumentEvent
			if e = rows.Scan(&event.Action, &event.Reason, &event.Actor, &event.At, &event.Version); e != nil {
				rows.Close()
				return x, e
			}
			x.Events = append(x.Events, event)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
	}
	args = append(args, sql.Named("group", x.GroupID))
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_documents x JOIN document_groups g ON g.id=x.group_id WHERE x.group_id=:group AND `+scope, args...).Scan(&x.HistoryTotal); err != nil {
		return x, err
	}
	x.HistoryPage = historyPage
	maxPage := (x.HistoryTotal + 11) / 12
	if maxPage < 1 {
		maxPage = 1
	}
	if x.HistoryPage > maxPage {
		x.HistoryPage = maxPage
	}
	rows, err := tx.QueryContext(ctx, librarySelect+" WHERE x.group_id=:group AND "+scope+" ORDER BY x.created_at DESC,x.rowid DESC LIMIT 12 OFFSET :offset", append(args, sql.Named("offset", (x.HistoryPage-1)*12))...)
	if err != nil {
		return x, err
	}
	x.Versions = []LibraryDocument{}
	for rows.Next() {
		v, e := scanDocument(rows)
		if e != nil {
			rows.Close()
			return x, e
		}
		x.Versions = append(x.Versions, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
func (s *Store) DocumentsFor(ctx context.Context, token, query, category, state string, page int) (DocumentPage, error) {
	out := DocumentPage{Items: []LibraryDocument{}, Homes: []RecordHome{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || !validText(query, 0, 100) {
		return out, ErrInvalid
	}
	if state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "WITHDRAWN" && state != "ARCHIVED" {
		return out, ErrInvalid
	}
	if category != "" {
		switch category {
		case "RECEIPT", "INVOICE", "NOTICE", "AGM_MINUTES", "AUDIT_REPORT", "CONTRACT", "AMC", "LEGAL", "CIRCULAR", "RESIDENT_DOCUMENT", "COMMITTEE_DOCUMENT", "PAYMENT_EVIDENCE", "ACCOUNTING_EXPORT":
		default:
			return out, ErrInvalid
		}
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return out, err
	}
	scope, args := documentScope(p)
	if state == "" {
		scope += ` AND (g.current_document_id=x.id OR x.review_state='PENDING')`
	} else {
		scope += ` AND x.review_state=:state`
		args = append(args, sql.Named("state", state))
	}
	if query != "" {
		scope += ` AND instr(lower(x.title||' '||x.original_filename),lower(:query))>0`
		args = append(args, sql.Named("query", query))
	}
	if category != "" {
		scope += ` AND x.category=:category`
		args = append(args, sql.Named("category", category))
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM library_documents x JOIN document_groups g ON g.id=x.group_id WHERE `+scope, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	maxPage := (out.Total + 11) / 12
	if maxPage < 1 {
		maxPage = 1
	}
	if out.Page > maxPage {
		out.Page = maxPage
	}
	rows, err := tx.QueryContext(ctx, librarySelect+" WHERE "+scope+" ORDER BY x.created_at DESC,x.rowid DESC LIMIT 12 OFFSET :offset", append(args, sql.Named("offset", (out.Page-1)*12))...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanDocument(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	homesPrincipal := p
	homesPrincipal.CanReviewRequests = p.CanManageDocuments || p.CanReadAllRecords
	out.Homes, err = reviewHomes(ctx, tx, homesPrincipal)
	if err != nil {
		return out, err
	}
	out.CanUpload = len(out.Homes) > 0 || p.CanManageDocuments || p.CanReadAllRecords
	return out, tx.Commit()
}
func (s *Store) DocumentSubjects(ctx context.Context, token, query string) ([]Person, error) {
	if !validText(query, 0, 100) {
		return nil, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return nil, err
	}
	if !p.CanManageDocuments {
		return nil, ErrForbidden
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.full_name FROM residents r WHERE instr(lower(r.full_name),lower(?))>0 AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.resident_id=r.id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)) ORDER BY r.full_name LIMIT 20`, query, today(), today())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		var x Person
		if err = rows.Scan(&x.ID, &x.Name); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
func (s *Store) DecideDocument(ctx context.Context, token, id string, in DocumentAction) (string, error) {
	if in.Version < 1 || !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", invalid("Confirm the decision and give a reason of 5–300 characters.")
	}
	if in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "ARCHIVED" && in.Action != "RETRY_VALIDATION" {
		return "", ErrInvalid
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	scope, args := documentScope(p)
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=:target AND "+scope, append(args, sql.Named("target", id))...))
	if err != nil {
		return "", err
	}
	reviewer := documentReviewer(p, x.Category)
	allowed, err := documentPreparationAllowed(ctx, tx, p, x)
	if err != nil {
		return "", err
	}
	if in.Action == "WITHDRAWN" {
		if x.AuthorID != p.ID || !allowed {
			return "", ErrForbidden
		}
	} else if in.Action == "RETRY_VALIDATION" {
		if !allowed || (!reviewer && x.AuthorID != p.ID) {
			return "", ErrForbidden
		}
	} else if in.Action == "ARCHIVED" {
		if !reviewer && !(x.AuthorID == p.ID && x.Visibility == "RESIDENT_SPECIFIC" && allowed) {
			return "", ErrForbidden
		}
	} else {
		if !reviewer || x.AuthorID == p.ID {
			return "", ErrForbidden
		}
	}
	if in.Action != "WITHDRAWN" && !p.Fresh {
		return "", ErrReauthRequired
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "DOCUMENT_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	if in.Action == "ARCHIVED" {
		if x.State != "APPROVED" || !x.Current {
			return "", ErrConflict
		}
	} else if x.State != "PENDING" {
		return "", ErrConflict
	}
	if in.Action == "APPROVED" {
		if x.Validation != "AVAILABLE" {
			return "", ErrConflict
		}
		var head string
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(current_document_id,''),approved_revision+1 FROM document_groups WHERE id=? AND archived=0`, x.GroupID).Scan(&head, &x.Revision)
		if err != nil {
			return "", err
		}
		if head != x.Replaces {
			return "", ErrConflict
		}
	}
	if in.Action == "RETRY_VALIDATION" {
		if x.Validation != "REJECTED" || x.ValidationError != "CHECKS_UNAVAILABLE" {
			return "", ErrConflict
		}
		_, err = tx.ExecContext(ctx, `UPDATE library_documents SET validation_status='PENDING',validation_error_code='',validation_attempts=0,version=version+1 WHERE id=?`, id)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE library_documents SET review_state=?,revision=?,version=version+1 WHERE id=?`, in.Action, x.Revision, id)
	}
	if err != nil {
		return "", err
	}
	x.Version++
	if in.Action != "RETRY_VALIDATION" {
		x.State = in.Action
	}
	if in.Action == "APPROVED" {
		_, err = tx.ExecContext(ctx, `UPDATE document_groups SET current_document_id=?,approved_revision=? WHERE id=?`, id, x.Revision, x.GroupID)
	} else if in.Action == "ARCHIVED" {
		_, err = tx.ExecContext(ctx, `UPDATE document_groups SET current_document_id=NULL,archived=1 WHERE id=?`, x.GroupID)
	}
	if err != nil {
		return "", err
	}
	if err = appendDocumentEvent(ctx, tx, p.ID, x, in.Action, in.Reason, true); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) DownloadDocument(ctx context.Context, token, id string) (LibraryDocument, []byte, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return LibraryDocument{}, nil, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return LibraryDocument{}, nil, err
	}
	scope, args := documentScope(p)
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=:target AND "+scope, append(args, sql.Named("target", id))...))
	if err != nil {
		return x, nil, err
	}
	if x.Validation != "AVAILABLE" {
		return x, nil, ErrConflict
	}
	var bytes []byte
	if err = tx.QueryRowContext(ctx, `SELECT original_bytes FROM local_document_objects WHERE document_id=?`, id).Scan(&bytes); err != nil {
		return x, nil, err
	}
	sum := sha256.Sum256(bytes)
	if int64(len(bytes)) != x.Size || hex.EncodeToString(sum[:]) != x.SHA256 {
		return x, nil, errors.New("document integrity mismatch")
	}
	return x, bytes, tx.Commit()
}

type DocumentCheckJob struct {
	ID, Filename, SHA256, Lease string
	Bytes                       []byte
}

func (s *Store) ClaimDocumentCheck(ctx context.Context, now time.Time) (DocumentCheckJob, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DocumentCheckJob{}, err
	}
	defer tx.Rollback()
	// Acquire the writer before selecting work, including after a crashed worker lease.
	if _, err = tx.ExecContext(ctx, `UPDATE library_documents SET version=version WHERE id=(SELECT id FROM library_documents LIMIT 1)`); err != nil {
		return DocumentCheckJob{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,version FROM library_documents WHERE review_state='PENDING' AND validation_status='PENDING' AND uploaded_at IS NULL AND expires_at<=? LIMIT 20`, now.Unix())
	if err != nil {
		return DocumentCheckJob{}, err
	}
	type expired struct {
		id string
		v  int
	}
	expiredItems := []expired{}
	for rows.Next() {
		var e expired
		if err = rows.Scan(&e.id, &e.v); err != nil {
			rows.Close()
			return DocumentCheckJob{}, err
		}
		expiredItems = append(expiredItems, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return DocumentCheckJob{}, err
	}
	for _, e := range expiredItems {
		if _, err = tx.ExecContext(ctx, `UPDATE library_documents SET validation_status='ABANDONED',version=version+1 WHERE id=?`, e.id); err != nil {
			return DocumentCheckJob{}, err
		}
		if err = appendDocumentEvent(ctx, tx, "", LibraryDocument{ID: e.id, Version: e.v + 1}, "ABANDONED", "The unused upload reservation expired", false); err != nil {
			return DocumentCheckJob{}, err
		}
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,version FROM library_documents WHERE validation_status='VALIDATING' AND validation_attempts=3 AND validation_lease_until<=? LIMIT 20`, now.Unix())
	if err != nil {
		return DocumentCheckJob{}, err
	}
	exhausted := []expired{}
	for rows.Next() {
		var e expired
		if err = rows.Scan(&e.id, &e.v); err != nil {
			rows.Close()
			return DocumentCheckJob{}, err
		}
		exhausted = append(exhausted, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return DocumentCheckJob{}, err
	}
	for _, e := range exhausted {
		if _, err = tx.ExecContext(ctx, `UPDATE library_documents SET validation_status='REJECTED',validation_error_code='PDF_CHECK_LIMIT',validation_lease_token='',validation_lease_until=0,version=version+1 WHERE id=?`, e.id); err != nil {
			return DocumentCheckJob{}, err
		}
		if err = appendDocumentEvent(ctx, tx, "", LibraryDocument{ID: e.id, Version: e.v + 1}, "VALIDATION_REJECTED", "File checks exceeded their retry limit", false); err != nil {
			return DocumentCheckJob{}, err
		}
	}
	var job DocumentCheckJob
	err = tx.QueryRowContext(ctx, `SELECT x.id,x.original_filename,x.verified_sha256,o.original_bytes FROM library_documents x JOIN local_document_objects o ON o.document_id=x.id WHERE x.review_state='PENDING' AND x.validation_attempts<3 AND (x.validation_status='PENDING' OR (x.validation_status='VALIDATING' AND x.validation_lease_until<=?)) ORDER BY x.uploaded_at,x.rowid LIMIT 1`, now.Unix()).Scan(&job.ID, &job.Filename, &job.SHA256, &job.Bytes)
	if errors.Is(err, sql.ErrNoRows) {
		if err = tx.Commit(); err != nil {
			return job, err
		}
		return job, sql.ErrNoRows
	}
	if err != nil {
		return job, err
	}
	job.Lease = randomToken()
	_, err = tx.ExecContext(ctx, `UPDATE library_documents SET validation_status='VALIDATING',validation_lease_token=?,validation_lease_until=?,validation_attempts=validation_attempts+1,version=version+1 WHERE id=?`, job.Lease, now.Add(30*time.Second).Unix(), job.ID)
	if err != nil {
		return job, err
	}
	return job, tx.Commit()
}
func (s *Store) FinishDocumentCheck(ctx context.Context, job DocumentCheckJob, contentType, code string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status, action, reason := "AVAILABLE", "VALIDATED", "File checks passed; separate review is still required"
	if code != "" {
		status, action, reason = "REJECTED", "VALIDATION_REJECTED", "File checks rejected this upload: "+code
		contentType = ""
	}
	result, err := tx.ExecContext(ctx, `UPDATE library_documents SET validation_status=?,detected_content_type=?,validation_error_code=?,available_at=?,validation_lease_until=0,validation_lease_token='',version=version+1 WHERE id=? AND validation_status='VALIDATING' AND validation_lease_token=?`, status, contentType, code, now.Unix(), job.ID, job.Lease)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	var v int
	if err = tx.QueryRowContext(ctx, `SELECT version FROM library_documents WHERE id=?`, job.ID).Scan(&v); err != nil {
		return err
	}
	if err = appendDocumentEvent(ctx, tx, "", LibraryDocument{ID: job.ID, Version: v}, action, reason, false); err != nil {
		return err
	}
	return tx.Commit()
}
