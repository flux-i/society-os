package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type FineInput struct {
	OperationKey    string `json:"operation_key"`
	IncidentID      string `json:"incident_id"`
	ReplacesID      string `json:"replaces_id"`
	SourceKey       string `json:"source_key"`
	Title           string `json:"title"`
	Amount          string `json:"amount"`
	PolicyReference string `json:"policy_reference"`
	Reason          string `json:"reason"`
	DueDate         string `json:"due_date"`
	ResponseBy      string `json:"response_by"`
	NoticeBody      string `json:"notice_body"`
	Confirmed       bool   `json:"confirmed"`
}
type FineAction struct {
	OperationKey        string `json:"operation_key"`
	Version             int    `json:"version"`
	Action              string `json:"action"`
	Reason              string `json:"reason"`
	SourceKey           string `json:"source_key"`
	ResponseCount       int    `json:"response_count"`
	Resolution          string `json:"resolution"`
	EarlyIssueReference string `json:"early_issue_reference"`
	ResolutionKey       string `json:"resolution_key"`
	Confirmed           bool   `json:"confirmed"`
}
type FineSource struct {
	IncidentID      string            `json:"incident_id"`
	FlatID          string            `json:"flat_id"`
	Home            string            `json:"home"`
	Rule            Rule              `json:"rule"`
	IncidentDate    string            `json:"incident_date"`
	SubstantiatedAt int64             `json:"substantiated_at"`
	OutcomeEventID  int64             `json:"outcome_event_id"`
	MaterialKey     string            `json:"material_key"`
	SourceKey       string            `json:"source_key"`
	Notice          *FineSourceNotice `json:"notice,omitempty"`
	ResponseTotal   int               `json:"response_total"`
	ResponsePage    int               `json:"response_page"`
	PageSize        int               `json:"page_size"`
	reporterID      string
}
type FineSourceNotice struct {
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Body       string             `json:"body"`
	ResponseBy string             `json:"response_by"`
	Responses  []IncidentResponse `json:"responses"`
}
type FineSourcePage struct {
	Items    []FineSource `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}
type Fine struct {
	ID                  string      `json:"id"`
	IncidentID          string      `json:"incident_id,omitempty"`
	ReplacesID          string      `json:"replaces_id,omitempty"`
	FlatID              string      `json:"flat_id"`
	Home                string      `json:"home"`
	RuleID              string      `json:"rule_id,omitempty"`
	MaterialKey         string      `json:"material_key,omitempty"`
	SourceKey           string      `json:"source_key,omitempty"`
	Source              *FineSource `json:"source,omitempty"`
	CurrentSource       *FineSource `json:"current_source,omitempty"`
	Title               string      `json:"title"`
	AmountPaise         int64       `json:"amount_paise"`
	PolicyReference     string      `json:"policy_reference,omitempty"`
	Reason              string      `json:"reason,omitempty"`
	DueDate             string      `json:"due_date"`
	ResponseBy          string      `json:"response_by"`
	NoticeBody          string      `json:"notice_body,omitempty"`
	State               string      `json:"state"`
	AuthorID            string      `json:"author_id,omitempty"`
	CreatedAt           int64       `json:"created_at,omitempty"`
	UpdatedAt           int64       `json:"updated_at,omitempty"`
	Version             int         `json:"version,omitempty"`
	PublicVersion       int         `json:"-"`
	PublicUpdatedAt     int64       `json:"-"`
	NoticeID            string      `json:"notice_id,omitempty"`
	OriginalEntryID     string      `json:"original_entry_id,omitempty"`
	CurrentEntryID      string      `json:"current_entry_id,omitempty"`
	ChargeVersion       int         `json:"charge_version,omitempty"`
	WaivedPaise         int64       `json:"waived_paise"`
	ActivePaise         int64       `json:"active_paise"`
	AllocatedPaise      int64       `json:"allocated_paise"`
	OutstandingPaise    int64       `json:"outstanding_paise"`
	Resolution          string      `json:"resolution,omitempty"`
	ResolutionKey       string      `json:"resolution_key,omitempty"`
	EarlyIssueReference string      `json:"early_issue_reference,omitempty"`
	ResolvedBy          string      `json:"resolved_by,omitempty"`
	ResolvedAt          int64       `json:"resolved_at,omitempty"`
	IssuedBy            string      `json:"issued_by,omitempty"`
	IssuedAt            int64       `json:"issued_at,omitempty"`
	IssuedContextKey    string      `json:"-"`
	PauseUntil          string      `json:"pause_until,omitempty"`
	PauseAppealID       string      `json:"pause_appeal_id,omitempty"`
	SourceCurrent       bool        `json:"source_current"`
	ResolutionCurrent   bool        `json:"resolution_current"`
	NeedsReview         bool        `json:"needs_review"`
	ResponseTotal       int         `json:"response_total,omitempty"`
	CurrentSourceKey    string      `json:"current_source_key,omitempty"`
	CanDecide           bool        `json:"can_decide"`
}
type FineTotals struct {
	ActivePaise      int64 `json:"active_paise"`
	AllocatedPaise   int64 `json:"allocated_paise"`
	OutstandingPaise int64 `json:"outstanding_paise"`
	OverduePaise     int64 `json:"overdue_paise"`
	Pending          int   `json:"pending"`
	Notified         int   `json:"notified"`
	Paused           int   `json:"paused"`
	NeedsReview      int   `json:"needs_review"`
}
type FinePage struct {
	Items    []Fine     `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Totals   FineTotals `json:"totals"`
}
type FineDetail struct {
	Fine
	Events     []FundEvent `json:"events,omitempty"`
	EventTotal int         `json:"event_total,omitempty"`
	EventPage  int         `json:"event_page,omitempty"`
	PageSize   int         `json:"page_size"`
	Notice     *FineNotice `json:"notice,omitempty"`
}
type FineNotice struct {
	ID              string             `json:"id"`
	FineID          string             `json:"fine_id"`
	FlatID          string             `json:"flat_id"`
	Home            string             `json:"home"`
	Title           string             `json:"title"`
	Body            string             `json:"body"`
	AmountPaise     int64              `json:"amount_paise"`
	PolicyReference string             `json:"policy_reference"`
	DueDate         string             `json:"due_date"`
	ResponseBy      string             `json:"response_by"`
	CreatedAt       int64              `json:"created_at"`
	Version         int                `json:"version"`
	State           string             `json:"state"`
	CanRespond      bool               `json:"can_respond"`
	CanAppeal       bool               `json:"can_appeal"`
	CanReadFinance  bool               `json:"can_read_finance"`
	Responses       []IncidentResponse `json:"responses"`
	ResponseTotal   int                `json:"response_total"`
	ResponsePage    int                `json:"response_page"`
	PageSize        int                `json:"page_size"`
}
type FineNoticePage struct {
	Items    []FineNotice `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

const fineSelect = `SELECT f.id,f.incident_id,COALESCE(f.replaces_id,''),f.flat_id,b.code||'-'||h.flat_number,f.rule_id,f.material_key,f.source_key,f.source_json,f.title,f.amount_paise,f.policy_reference,f.reason,f.due_date,f.response_by,f.notice_body,f.state,f.author_id,f.created_at,f.updated_at,f.version,f.public_version,f.public_updated_at,COALESCE(f.notice_id,''),COALESCE(f.original_entry_id,''),COALESCE(f.current_entry_id,''),f.charge_version,f.waived_paise,f.resolution,f.resolution_key,f.early_issue_reference,COALESCE(f.resolved_by,''),COALESCE(f.resolved_at,0),COALESCE(f.issued_by,''),COALESCE(f.issued_at,0),f.issued_context_key,f.pause_until,COALESCE(f.pause_appeal_id,'') FROM fines f JOIN flats h ON h.id=f.flat_id JOIN buildings b ON b.id=h.building_id `

func scanFine(row interface{ Scan(...any) error }) (Fine, error) {
	var x Fine
	var blob string
	e := row.Scan(&x.ID, &x.IncidentID, &x.ReplacesID, &x.FlatID, &x.Home, &x.RuleID, &x.MaterialKey, &x.SourceKey, &blob, &x.Title, &x.AmountPaise, &x.PolicyReference, &x.Reason, &x.DueDate, &x.ResponseBy, &x.NoticeBody, &x.State, &x.AuthorID, &x.CreatedAt, &x.UpdatedAt, &x.Version, &x.PublicVersion, &x.PublicUpdatedAt, &x.NoticeID, &x.OriginalEntryID, &x.CurrentEntryID, &x.ChargeVersion, &x.WaivedPaise, &x.Resolution, &x.ResolutionKey, &x.EarlyIssueReference, &x.ResolvedBy, &x.ResolvedAt, &x.IssuedBy, &x.IssuedAt, &x.IssuedContextKey, &x.PauseUntil, &x.PauseAppealID)
	if e == nil {
		var source FineSource
		e = json.Unmarshal([]byte(blob), &source)
		x.Source = &source
	}
	return x, e
}
func (s *Store) beginFineWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, p, e := s.beginRecordWrite(ctx, token)
	if e == nil && !p.Fresh {
		tx.Rollback()
		return nil, p, ErrReauthRequired
	}
	return tx, p, e
}
func fineEligible(ctx context.Context, q identityReader, p Principal, x Fine) error {
	if !p.CanManageRecords {
		return ErrForbidden
	}
	if !p.Fresh {
		return ErrReauthRequired
	}
	if x.AuthorID == p.ID {
		return ErrForbidden
	}
	var reporter string
	e := q.QueryRowContext(ctx, "SELECT reporter_id FROM incidents WHERE id=?", x.IncidentID).Scan(&reporter)
	if e != nil {
		return e
	}
	if reporter == p.ID {
		return ErrForbidden
	}
	return nil
}
func fineEvent(ctx context.Context, tx *sql.Tx, p Principal, fine, kind, id, action, reason string, version int, snapshot any) error {
	data, e := json.Marshal(snapshot)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO fine_events(fine_id,subject_type,subject_id,version,action,actor_id,reason,snapshot_json,occurred_at) VALUES(?,?,?,?,?,?,?,?,?)`, fine, kind, id, version, action, p.ID, reason, string(data), time.Now().Unix())
	if e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "FINE_"+kind+"_"+action, reason, map[string]any{"id": id}, map[string]any{"id": id, "fine_id": fine, "version": version})
}
func fineTouch(ctx context.Context, tx *sql.Tx, p Principal, x Fine, action, reason string, public bool, snapshot any) error {
	delta := 0
	if public {
		delta = 1
	}
	_, e := tx.ExecContext(ctx, `UPDATE fines SET version=version+1,public_version=public_version+?,updated_at=? WHERE id=? AND version=?`, delta, time.Now().Unix(), x.ID, x.Version)
	if e != nil {
		return e
	}
	return fineEvent(ctx, tx, p, x.ID, "FINE", x.ID, action, reason, x.Version+1, snapshot)
}
func fineReadScope(p Principal) (string, []any) {
	if p.CanReadAllRecords {
		return "1=1", nil
	}
	return `f.state IN('ISSUED','WAIVED') AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.flat_id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ResidentID, today(), today()}
}
func fineNoticeScope(p Principal) (string, []any) {
	if p.CanReadAllRecords {
		return "1=1", nil
	}
	return `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=n.flat_id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ResidentID, today(), today()}
}
func fineFinanceAllowed(ctx context.Context, q *sql.Tx, p Principal, home string) bool {
	if !p.CanReadRecords {
		return false
	}
	_, e := permittedFinancialHome(ctx, q, p, home)
	return e == nil
}
func fineResponseIdentity(ctx context.Context, tx *sql.Tx, notice string) (string, int, error) {
	ids := []string{}
	rows, e := tx.QueryContext(ctx, "SELECT id FROM fine_responses WHERE notice_id=? ORDER BY id", notice)
	if e != nil {
		return "", 0, e
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return "", 0, e
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		return "", 0, e
	}
	data, _ := json.Marshal(ids)
	return TokenHash(string(data)), len(ids), nil
}
func fineContext(ctx context.Context, tx *sql.Tx, x Fine) (FineSource, string, int, error) {
	source, e := fineSourceIn(ctx, tx, x.IncidentID, 1)
	if e != nil {
		return source, "", 0, e
	}
	if source.MaterialKey != x.MaterialKey {
		return source, "", 0, ErrConflict
	}
	replies, count, e := fineResponseIdentity(ctx, tx, x.NoticeID)
	return source, TokenHash(source.SourceKey + "\x00" + replies), count + source.ResponseTotal, e
}
func fineProjection(x Fine, p Principal) Fine {
	if p.CanReadAllRecords {
		return x
	}
	x.IncidentID = ""
	x.ReplacesID = ""
	x.RuleID = ""
	x.MaterialKey = ""
	x.SourceKey = ""
	x.Source = nil
	x.CurrentSource = nil
	x.AuthorID = ""
	x.Reason = ""
	x.Resolution = ""
	x.ResolutionKey = ""
	x.EarlyIssueReference = ""
	x.ResolvedBy = ""
	x.ResolvedAt = 0
	x.IssuedBy = ""
	x.CurrentSourceKey = ""
	x.ResponseTotal = 0
	x.ChargeVersion = 0
	x.PauseAppealID = ""
	x.CanDecide = false
	x.NeedsReview = false
	x.SourceCurrent = false
	x.ResolutionCurrent = false
	x.Version = x.PublicVersion
	x.CreatedAt = x.IssuedAt
	x.UpdatedAt = x.PublicUpdatedAt
	return x
}
func fineEnrich(ctx context.Context, tx *sql.Tx, p Principal, x *Fine) error {
	if x.CurrentEntryID != "" {
		entry, e := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", x.CurrentEntryID))
		if e != nil {
			return e
		}
		if entry.State == "POSTED" {
			x.ActivePaise = entry.AmountPaise
			e = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount_paise),0) FROM live_entry_allocations WHERE charge_id=?", x.CurrentEntryID).Scan(&x.AllocatedPaise)
			if e != nil {
				return e
			}
			x.OutstandingPaise = x.ActivePaise - x.AllocatedPaise
		}
	}
	if !p.CanReadAllRecords {
		return nil
	}
	source, key, count, e := fineContext(ctx, tx, *x)
	if e != nil && !errors.Is(e, sql.ErrNoRows) && !errors.Is(e, ErrConflict) && !errors.Is(e, ErrInvalid) {
		return e
	}
	x.SourceCurrent = e == nil
	if e == nil {
		x.CurrentSource = &source
	}
	x.CurrentSourceKey = source.SourceKey
	x.ResponseTotal = count
	x.ResolutionCurrent = e == nil && x.ResolutionKey != "" && x.ResolutionKey == key
	x.NeedsReview = (x.State == "ISSUED" || x.State == "WAIVED") && (e != nil || x.IssuedContextKey != key)
	x.CanDecide = fineEligible(ctx, tx, p, *x) == nil
	return nil
}
func fineEvents(ctx context.Context, tx *sql.Tx, kind, id string, page int) ([]FundEvent, int, int, error) {
	total := 0
	e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fine_events WHERE subject_type=? AND subject_id=?", kind, id).Scan(&total)
	if e != nil {
		return nil, 0, page, e
	}
	page = incidentPageBound(page, total, 20)
	rows, e := tx.QueryContext(ctx, `SELECT x.action,u.display_name,x.reason,x.version,x.occurred_at FROM fine_events x JOIN users u ON u.id=x.actor_id WHERE x.subject_type=? AND x.subject_id=? ORDER BY x.id DESC LIMIT 20 OFFSET ?`, kind, id, (page-1)*20)
	if e != nil {
		return nil, total, page, e
	}
	defer rows.Close()
	items := []FundEvent{}
	for rows.Next() {
		var x FundEvent
		if e = rows.Scan(&x.Action, &x.Actor, &x.Reason, &x.Version, &x.At); e != nil {
			return nil, total, page, e
		}
		items = append(items, x)
	}
	return items, total, page, rows.Err()
}
func fineReadInput(q, state string, page int) error {
	if e := incidentReadInput(q, state, page); e != nil {
		return e
	}
	if state != "" && !strings.Contains("|PENDING|NOTIFIED|ISSUED|WAIVED|DECLINED|WITHDRAWN|", "|"+state+"|") {
		return ErrInvalid
	}
	return nil
}

func fineSearchPattern(q string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q) + "%"
}
