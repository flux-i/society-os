package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ComplaintInput struct {
	OperationKey string `json:"operation_key"`
	FlatID       string `json:"flat_id"`
	Category     string `json:"category"`
	Subject      string `json:"subject"`
	Description  string `json:"description"`
	Priority     string `json:"priority"`
}
type ComplaintAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Message      string `json:"message"`
	Visibility   string `json:"visibility"`
	Status       string `json:"status"`
	AssignedTo   string `json:"assigned_to"`
	Priority     string `json:"priority"`
}
type Complaint struct {
	ID              string `json:"id"`
	Number          string `json:"number"`
	FlatID          string `json:"flat_id"`
	Home            string `json:"home"`
	ReportedBy      string `json:"reported_by"`
	Reporter        string `json:"reporter"`
	Category        string `json:"category"`
	Subject         string `json:"subject"`
	Description     string `json:"description"`
	Priority        string `json:"priority"`
	Status          string `json:"status"`
	AssignedTo      string `json:"assigned_to"`
	AssignedName    string `json:"assigned_name"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	ResolvedAt      int64  `json:"resolved_at"`
	ClosedAt        int64  `json:"closed_at"`
	Version         int    `json:"version"`
	PublicVersion   int    `json:"-"`
	PublicUpdatedAt int64  `json:"-"`
}
type ComplaintUpdate struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	Message    string `json:"message"`
	Visibility string `json:"visibility"`
	Actor      string `json:"actor"`
	At         int64  `json:"at"`
	Status     string `json:"status"`
}
type ComplaintHandler struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type ComplaintDetail struct {
	Complaint
	Updates         []ComplaintUpdate  `json:"updates"`
	HistoryPage     int                `json:"history_page"`
	HistoryTotal    int                `json:"history_total"`
	HistoryPageSize int                `json:"history_page_size"`
	CanParticipate  bool               `json:"can_participate"`
	Handlers        []ComplaintHandler `json:"handlers"`
}
type ComplaintPage struct {
	Items    []Complaint  `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Homes    []RecordHome `json:"homes"`
}

const complaintSelect = `SELECT c.id,c.complaint_number,c.flat_id,b.code||'-'||f.flat_number,c.reported_by,u.display_name,c.category,c.subject,c.description,c.priority,c.status,COALESCE(c.assigned_to,''),COALESCE(a.display_name,''),c.created_at,c.updated_at,COALESCE(c.resolved_at,0),COALESCE(c.closed_at,0),c.version,c.public_version,c.public_updated_at FROM complaints c JOIN flats f ON f.id=c.flat_id JOIN buildings b ON b.id=f.building_id JOIN users u ON u.id=c.reported_by LEFT JOIN users a ON a.id=c.assigned_to`

func scanComplaint(row interface{ Scan(...any) error }) (Complaint, error) {
	var c Complaint
	err := row.Scan(&c.ID, &c.Number, &c.FlatID, &c.Home, &c.ReportedBy, &c.Reporter, &c.Category, &c.Subject, &c.Description, &c.Priority, &c.Status, &c.AssignedTo, &c.AssignedName, &c.CreatedAt, &c.UpdatedAt, &c.ResolvedAt, &c.ClosedAt, &c.Version, &c.PublicVersion, &c.PublicUpdatedAt)
	return c, err
}
func complaintScope(p Principal) (string, []any) {
	if p.CanHandleComplaints {
		return "1=1", nil
	}
	return "c.reported_by=?", []any{p.ID}
}
func complaintPriority(value string) bool {
	return value == "NORMAL" || value == "HIGH" || value == "URGENT"
}
func complaintStatus(value string) bool {
	return value == "OPEN" || value == "ACKNOWLEDGED" || value == "IN_PROGRESS" || value == "WAITING" || value == "RESOLVED" || value == "CLOSED"
}
func complaintHomeAllowed(ctx context.Context, q identityReader, p Principal, id string) (bool, error) {
	var allowed bool
	query := "SELECT EXISTS(SELECT 1 FROM flats f WHERE f.id=?"
	args := []any{id}
	if !p.CanHandleComplaints {
		query += ` AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = append(args, p.ResidentID, today(), today())
	}
	err := q.QueryRowContext(ctx, query+")", args...).Scan(&allowed)
	return allowed, err
}
func complaintHomes(ctx context.Context, q identityReader, p Principal) ([]RecordHome, error) {
	query := `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id`
	args := []any{}
	if !p.CanHandleComplaints {
		query += ` WHERE EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = []any{p.ResidentID, today(), today()}
	}
	rows, err := q.QueryContext(ctx, query+" ORDER BY b.code,f.floor,f.flat_number", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecordHome{}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func complaintHandlers(ctx context.Context, q identityReader) ([]ComplaintHandler, error) {
	now := time.Now().Unix()
	rows, err := q.QueryContext(ctx, `SELECT u.id,u.display_name FROM users u WHERE u.status='ACTIVE' AND u.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=u.id AND g.role IN ('ADMINISTRATOR','COMMITTEE') AND g.valid_from<=? AND g.valid_until>? AND g.revoked_at IS NULL) ORDER BY u.display_name,u.id`, now, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ComplaintHandler{}
	for rows.Next() {
		var h ComplaintHandler
		if err = rows.Scan(&h.ID, &h.Name); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Store) CreateComplaint(ctx context.Context, token string, in ComplaintInput) (string, error) {
	if !validText(in.Subject, 5, 120) || !paragraph(in.Description, 10, 4000) || !complaintPriority(in.Priority) {
		return "", invalid("Choose a priority, a subject of 5–120 characters and details of 10–4000 characters.")
	}
	switch in.Category {
	case "PLUMBING", "LIFT", "ELECTRICAL", "SECURITY", "CLEANING", "WATER", "PARKING", "COMMON_AREA", "OTHER":
	default:
		return "", invalid("Choose a supported service category.")
	}
	if len(in.FlatID) < 1 || len(in.FlatID) > 100 {
		return "", invalid("Choose a current home for this request.")
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	allowed, err := complaintHomeAllowed(ctx, tx, p, in.FlatID)
	if err != nil {
		return "", err
	}
	if !allowed {
		return "", sql.ErrNoRows
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "COMPLAINT_CREATE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var number int
	year := time.Now().In(time.FixedZone("IST", 19800)).Year()
	err = tx.QueryRowContext(ctx, `INSERT INTO complaint_counters VALUES(?,1) ON CONFLICT(year) DO UPDATE SET last_number=last_number+1 RETURNING last_number`, year).Scan(&number)
	if err != nil {
		return "", err
	}
	id := randomToken()
	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO complaints(id,complaint_number,flat_id,reported_by,category,subject,description,priority,status,created_at,updated_at,public_updated_at) VALUES(?,?,?,?,?,?,?,?,'OPEN',?,?,?)`, id, fmt.Sprintf("SR-%d-%06d", year, number), in.FlatID, p.ID, in.Category, in.Subject, in.Description, in.Priority, now, now, now)
	if err != nil {
		return "", err
	}
	c, err := scanComplaint(tx.QueryRowContext(ctx, complaintSelect+" WHERE c.id=?", id))
	if err != nil {
		return "", err
	}
	if err = complaintEvent(ctx, tx, p, c, "CREATED", "Service request reported.", "RESIDENT_VISIBLE", nil); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func complaintEvent(ctx context.Context, tx *sql.Tx, p Principal, c Complaint, action, message, visibility string, before any) error {
	bytes, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO complaint_updates(public_id,complaint_id,actor_id,action,message,visibility,occurred_at,version,snapshot_json) VALUES(?,?,?,?,?,?,?,?,?)`, randomToken(), c.ID, p.ID, action, message, visibility, time.Now().Unix(), c.Version, string(bytes))
	if err != nil {
		return err
	}
	return appendAudit(ctx, tx, p.ID, "", "COMPLAINT_"+action, message, before, c)
}
func (s *Store) ComplaintsFor(ctx context.Context, token, query, status string, page int) (ComplaintPage, error) {
	out := ComplaintPage{Items: []Complaint{}, Page: page, PageSize: 12, Homes: []RecordHome{}}
	if len(query) > 400 || (query != "" && !validText(query, 1, 100)) || page < 1 || page > 10000 || (status != "" && !complaintStatus(status)) {
		return out, ErrInvalid
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
	if out.Homes, err = complaintHomes(ctx, tx, p); err != nil {
		return out, err
	}
	scope, args := complaintScope(p)
	if query != "" {
		scope += " AND instr(lower(c.complaint_number||' '||c.subject||' '||c.description),lower(?))>0"
		args = append(args, query)
	}
	if status != "" {
		scope += " AND c.status=?"
		args = append(args, status)
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM complaints c WHERE "+scope, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	max := (out.Total + 11) / 12
	if max < 1 {
		max = 1
	}
	if out.Page > max {
		out.Page = max
	}
	order := "c.updated_at"
	if !p.CanHandleComplaints {
		order = "c.public_updated_at"
	}
	rows, err := tx.QueryContext(ctx, complaintSelect+" WHERE "+scope+" ORDER BY "+order+" DESC,c.rowid DESC LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		c, e := scanComplaint(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		if !p.CanHandleComplaints {
			c.Version = c.PublicVersion
			c.UpdatedAt = c.PublicUpdatedAt
		}
		out.Items = append(out.Items, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) ComplaintFor(ctx context.Context, token, id string, historyPage int) (ComplaintDetail, error) {
	out := ComplaintDetail{Updates: []ComplaintUpdate{}, Handlers: []ComplaintHandler{}, HistoryPage: historyPage, HistoryPageSize: 30}
	if historyPage < 1 || historyPage > 10000 {
		return out, ErrInvalid
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
	scope, args := complaintScope(p)
	out.Complaint, err = scanComplaint(tx.QueryRowContext(ctx, complaintSelect+" WHERE c.id=? AND "+scope, append([]any{id}, args...)...))
	if err != nil {
		return out, err
	}
	out.CanParticipate, err = complaintHomeAllowed(ctx, tx, p, out.FlatID)
	if err != nil {
		return out, err
	}
	if p.CanHandleComplaints {
		if out.Handlers, err = complaintHandlers(ctx, tx); err != nil {
			return out, err
		}
	}
	visible := "complaint_id=?"
	args = []any{id}
	if !p.CanHandleComplaints {
		visible += " AND visibility='RESIDENT_VISIBLE'"
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM complaint_updates WHERE "+visible, args...).Scan(&out.HistoryTotal); err != nil {
		return out, err
	}
	max := (out.HistoryTotal + 29) / 30
	if max < 1 {
		max = 1
	}
	if out.HistoryPage > max {
		out.HistoryPage = max
	}
	rows, err := tx.QueryContext(ctx, `SELECT x.public_id,x.action,x.message,x.visibility,u.display_name,x.occurred_at,json_extract(x.snapshot_json,'$.status') FROM complaint_updates x JOIN users u ON u.id=x.actor_id WHERE `+strings.ReplaceAll(visible, "complaint_id", "x.complaint_id")+" ORDER BY x.id DESC LIMIT 30 OFFSET ?", append(args, (out.HistoryPage-1)*30)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x ComplaintUpdate
		if err = rows.Scan(&x.ID, &x.Action, &x.Message, &x.Visibility, &x.Actor, &x.At, &x.Status); err != nil {
			rows.Close()
			return out, err
		}
		out.Updates = append(out.Updates, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i, j := 0, len(out.Updates)-1; i < j; i, j = i+1, j-1 {
		out.Updates[i], out.Updates[j] = out.Updates[j], out.Updates[i]
	}
	if !p.CanHandleComplaints {
		out.Version = out.PublicVersion
		out.UpdatedAt = out.PublicUpdatedAt
	}
	return out, tx.Commit()
}
func complaintTransition(from, to string) bool {
	allowed := map[string][]string{"OPEN": {"ACKNOWLEDGED", "IN_PROGRESS", "WAITING"}, "ACKNOWLEDGED": {"IN_PROGRESS", "WAITING"}, "IN_PROGRESS": {"WAITING", "RESOLVED"}, "WAITING": {"IN_PROGRESS", "RESOLVED"}, "RESOLVED": {"IN_PROGRESS", "CLOSED"}, "CLOSED": {"OPEN"}}
	for _, value := range allowed[from] {
		if value == to {
			return true
		}
	}
	return false
}
func (s *Store) UpdateComplaint(ctx context.Context, token, id string, in ComplaintAction) (string, error) {
	if in.Version < 1 || !paragraph(in.Message, 5, 2000) {
		return "", invalid("Give an update or reason of 5–2000 characters.")
	}
	if in.Action != "COMMENT" && in.Action != "STATUS" && in.Action != "ASSIGN" && in.Action != "PRIORITY" {
		return "", ErrInvalid
	}
	if (in.Action != "STATUS" && in.Status != "") || (in.Action != "ASSIGN" && in.AssignedTo != "") || (in.Action != "PRIORITY" && in.Priority != "") {
		return "", invalid("Each update changes one selected action.")
	}
	if in.Visibility != "RESIDENT_VISIBLE" && in.Visibility != "STAFF_ONLY" {
		return "", ErrInvalid
	}
	if in.Action != "COMMENT" && in.Visibility != "RESIDENT_VISIBLE" {
		return "", invalid("Status, priority and assignment updates are visible to the resident.")
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	scope, args := complaintScope(p)
	c, err := scanComplaint(tx.QueryRowContext(ctx, complaintSelect+" WHERE c.id=? AND "+scope, append([]any{id}, args...)...))
	if err != nil {
		return "", err
	}
	if !p.CanHandleComplaints {
		allowed, e := complaintHomeAllowed(ctx, tx, p, c.FlatID)
		if e != nil {
			return "", e
		}
		if !allowed {
			return "", ErrForbidden
		}
		if in.Visibility != "RESIDENT_VISIBLE" || (in.Action != "COMMENT" && in.Action != "STATUS") || (in.Action == "STATUS" && in.Status != "CLOSED" && in.Status != "IN_PROGRESS" && in.Status != "OPEN") {
			return "", ErrForbidden
		}
	} else if in.Action != "COMMENT" && !p.Fresh {
		return "", ErrReauthRequired
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "COMPLAINT_UPDATE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	expectedVersion := c.Version
	if !p.CanHandleComplaints {
		expectedVersion = c.PublicVersion
	}
	if expectedVersion != in.Version {
		return "", ErrConflict
	}
	if !p.CanHandleComplaints && in.Action == "STATUS" && !((c.Status == "RESOLVED" && (in.Status == "CLOSED" || in.Status == "IN_PROGRESS")) || (c.Status == "CLOSED" && in.Status == "OPEN")) {
		return "", ErrForbidden
	}
	before := c
	if c.Status == "CLOSED" && in.Action != "STATUS" {
		return "", invalid("Reopen this closed request before adding more work.")
	}
	switch in.Action {
	case "STATUS":
		if !complaintTransition(c.Status, in.Status) {
			return "", invalid("That status change is not available from the current state.")
		}
		c.Status = in.Status
		c.ResolvedAt = 0
		c.ClosedAt = 0
		if c.Status == "RESOLVED" {
			c.ResolvedAt = time.Now().Unix()
		}
		if c.Status == "CLOSED" {
			c.ResolvedAt = before.ResolvedAt
			c.ClosedAt = time.Now().Unix()
		}
	case "PRIORITY":
		if !complaintPriority(in.Priority) || c.Priority == in.Priority {
			return "", invalid("Choose a different valid priority.")
		}
		c.Priority = in.Priority
	case "ASSIGN":
		if c.AssignedTo == in.AssignedTo {
			return "", invalid("Choose a different handler or leave this request unassigned.")
		}
		c.AssignedTo = in.AssignedTo
		c.AssignedName = ""
		if in.AssignedTo != "" {
			handlers, e := complaintHandlers(ctx, tx)
			if e != nil {
				return "", e
			}
			for _, h := range handlers {
				if h.ID == in.AssignedTo {
					c.AssignedName = h.Name
					break
				}
			}
			if c.AssignedName == "" {
				return "", invalid("The selected handler is no longer available. Reload the details.")
			}
		}
	}
	c.Version++
	c.UpdatedAt = time.Now().Unix()
	if in.Visibility == "RESIDENT_VISIBLE" {
		c.PublicVersion++
		c.PublicUpdatedAt = c.UpdatedAt
	}
	_, err = tx.ExecContext(ctx, `UPDATE complaints SET status=?,priority=?,assigned_to=NULLIF(?,''),updated_at=?,resolved_at=?,closed_at=?,version=?,public_version=?,public_updated_at=? WHERE id=? AND version=?`, c.Status, c.Priority, c.AssignedTo, c.UpdatedAt, nullableComplaintTime(c.ResolvedAt), nullableComplaintTime(c.ClosedAt), c.Version, c.PublicVersion, c.PublicUpdatedAt, c.ID, before.Version)
	if err != nil {
		return "", err
	}
	if err = complaintEvent(ctx, tx, p, c, in.Action, in.Message, in.Visibility, before); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, c.ID); err != nil {
		return "", err
	}
	return c.ID, tx.Commit()
}
func nullableComplaintTime(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
