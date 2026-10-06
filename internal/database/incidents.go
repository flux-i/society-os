package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type RuleInput struct {
	OperationKey    string `json:"operation_key"`
	ReplacesID      string `json:"replaces_id"`
	Title           string `json:"title"`
	Text            string `json:"text"`
	PolicyReference string `json:"policy_reference"`
	EffectiveFrom   string `json:"effective_from"`
	EffectiveUntil  string `json:"effective_until"`
	FinePermitted   bool   `json:"fine_permitted"`
	Confirmed       bool   `json:"confirmed"`
}
type RuleAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type Rule struct {
	ID              string      `json:"id"`
	ReplacesID      string      `json:"replaces_id"`
	Title           string      `json:"title"`
	Text            string      `json:"text"`
	PolicyReference string      `json:"policy_reference"`
	EffectiveFrom   string      `json:"effective_from"`
	EffectiveUntil  string      `json:"effective_until"`
	FinePermitted   bool        `json:"fine_permitted"`
	State           string      `json:"state"`
	AuthorID        string      `json:"author_id,omitempty"`
	CreatedAt       int64       `json:"created_at"`
	UpdatedAt       int64       `json:"updated_at"`
	Version         int         `json:"version"`
	Events          []FundEvent `json:"events,omitempty"`
	EventTotal      int         `json:"event_total,omitempty"`
	EventPage       int         `json:"event_page,omitempty"`
	PageSize        int         `json:"page_size,omitempty"`
}
type RulePage struct {
	Items    []Rule `json:"items"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
type IncidentInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	RuleID       string `json:"rule_id"`
	FlatID       string `json:"flat_id"`
	IncidentDate string `json:"incident_date"`
	Comment      string `json:"comment"`
	PictureID    string `json:"picture_id"`
	Confirmed    bool   `json:"confirmed"`
}
type IncidentAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	DuplicateOf  string `json:"duplicate_of"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	ResponseBy   string `json:"response_by"`
	Confirmed    bool   `json:"confirmed"`
}
type Incident struct {
	ID                string `json:"id"`
	RuleID            string `json:"rule_id"`
	RuleTitle         string `json:"rule_title"`
	FlatID            string `json:"flat_id"`
	Home              string `json:"home"`
	ReporterID        string `json:"reporter_id,omitempty"`
	IncidentDate      string `json:"incident_date"`
	Comment           string `json:"comment"`
	PictureID         string `json:"picture_id"`
	State             string `json:"state"`
	DuplicateOf       string `json:"duplicate_of,omitempty"`
	NoticeID          string `json:"notice_id,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
	Version           int    `json:"version"`
	ReporterVersion   int    `json:"-"`
	ReporterUpdatedAt int64  `json:"-"`
}
type IncidentDetail struct {
	Incident
	Rule           Rule             `json:"rule"`
	Events         []FundEvent      `json:"events"`
	EventTotal     int              `json:"event_total"`
	EventPage      int              `json:"event_page"`
	PageSize       int              `json:"page_size"`
	CanParticipate bool             `json:"can_participate"`
	Notices        []IncidentNotice `json:"notices,omitempty"`
	NoticeTotal    int              `json:"notice_total,omitempty"`
	ResponseTotal  int              `json:"response_total,omitempty"`
}
type IncidentPage struct {
	Items     []Incident `json:"items"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
	CanReport bool       `json:"can_report"`
}
type IncidentOptions struct {
	Homes     []RecordHome `json:"homes"`
	CanReport bool         `json:"can_report"`
}
type IncidentNotice struct {
	CanRespond    bool               `json:"can_respond"`
	ID            string             `json:"id"`
	IncidentID    string             `json:"incident_id,omitempty"`
	FlatID        string             `json:"flat_id"`
	Home          string             `json:"home"`
	RuleID        string             `json:"rule_id"`
	IncidentDate  string             `json:"incident_date"`
	Title         string             `json:"title"`
	Body          string             `json:"body"`
	ResponseBy    string             `json:"response_by"`
	PictureID     string             `json:"picture_id"`
	CreatedAt     int64              `json:"created_at"`
	Version       int                `json:"version"`
	Active        bool               `json:"active"`
	Rule          Rule               `json:"rule"`
	Responses     []IncidentResponse `json:"responses"`
	ResponseTotal int                `json:"response_total"`
	ResponsePage  int                `json:"response_page"`
	PageSize      int                `json:"page_size"`
}
type IncidentNoticePage struct {
	Items    []IncidentNotice `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
type IncidentResponseInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Body         string `json:"body"`
	Confirmed    bool   `json:"confirmed"`
}
type IncidentResponse struct {
	ID        string `json:"id"`
	Actor     string `json:"actor,omitempty"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
}

const ruleSelect = `SELECT id,COALESCE(replaces_id,''),title,rule_text,policy_reference,effective_from,effective_until,fine_permitted,state,author_id,created_at,updated_at,version FROM society_rules`

func scanRule(row interface{ Scan(...any) error }) (Rule, error) {
	var x Rule
	e := row.Scan(&x.ID, &x.ReplacesID, &x.Title, &x.Text, &x.PolicyReference, &x.EffectiveFrom, &x.EffectiveUntil, &x.FinePermitted, &x.State, &x.AuthorID, &x.CreatedAt, &x.UpdatedAt, &x.Version)
	return x, e
}
func incidentParticipant(ctx context.Context, q identityReader, p Principal) (bool, error) {
	if p.CanHandleComplaints {
		return true, nil
	}
	var yes bool
	e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, p.ResidentID, today(), today()).Scan(&yes)
	return yes, e
}
func incidentHomeMember(ctx context.Context, q identityReader, p Principal, flat string) (bool, error) {
	var yes bool
	e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND flat_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, p.ResidentID, flat, today(), today()).Scan(&yes)
	return yes, e
}
func beginIncidentRead(ctx context.Context, s *Store, token string) (*sql.Tx, Principal, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, Principal{}, e
	}
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e != nil {
		tx.Rollback()
		return nil, p, e
	}
	return tx, p, nil
}
func incidentOperator(p Principal) error {
	if !p.CanHandleComplaints {
		return ErrForbidden
	}
	if !p.Fresh {
		return ErrReauthRequired
	}
	return nil
}
func incidentEvent(ctx context.Context, tx *sql.Tx, p Principal, x Incident, action, reason string, visible bool) error {
	data, e := json.Marshal(x)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO incident_events(incident_id,version,reporter_version,reporter_visible,actor_id,action,reason,occurred_at,snapshot_json) VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, x.Version, x.ReporterVersion, visible, p.ID, action, reason, time.Now().Unix(), string(data))
	if e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "INCIDENT_"+action, reason, map[string]any{"id": x.ID}, map[string]any{"id": x.ID, "version": x.Version})
}
func ruleEvent(ctx context.Context, tx *sql.Tx, p Principal, x Rule, action, reason string) error {
	data, e := json.Marshal(x)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO society_rule_events(rule_id,version,actor_id,action,reason,occurred_at,snapshot_json) VALUES(?,?,?,?,?,?,?)`, x.ID, x.Version, p.ID, action, reason, time.Now().Unix(), string(data))
	if e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "RULE_"+action, reason, map[string]any{"id": x.ID}, map[string]any{"id": x.ID, "version": x.Version})
}
func incidentPageBound(page, total, size int) int {
	max := (total + size - 1) / size
	if max < 1 {
		max = 1
	}
	if page > max {
		return max
	}
	return page
}
func incidentReadInput(query, state string, page int) error {
	if !validText(query, 0, 100) || len(query) > 400 || page < 1 || page > 100000 || len(state) > 30 {
		return ErrInvalid
	}
	return nil
}

func (s *Store) CreateRule(ctx context.Context, token string, in RuleInput) (string, error) {
	if !in.Confirmed || !validText(in.Title, 5, 120) || !paragraph(in.Text, 10, 4000) || !validText(in.PolicyReference, 5, 300) || !validMaintenanceDate(in.EffectiveFrom) || !optionalUpkeepDate(in.EffectiveUntil) || (in.EffectiveUntil != "" && in.EffectiveUntil < in.EffectiveFrom) || len(in.ReplacesID) > 100 {
		return "", invalid("Supply the policy text, authority reference and valid effective dates, then confirm the reviewed rule.")
	}
	tx, p, e := s.beginUpkeepWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if e = incidentOperator(p); e != nil {
		return "", e
	}
	var parent Rule
	if in.ReplacesID != "" {
		parent, e = scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", in.ReplacesID))
		if e != nil {
			return "", e
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "RULE_CREATE", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if in.ReplacesID != "" {
		if parent.State != "PUBLISHED" {
			return "", ErrConflict
		}
		var pending bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM society_rules WHERE replaces_id=? AND state='PENDING')`, parent.ID).Scan(&pending); e != nil {
			return "", e
		}
		if pending {
			return "", ErrConflict
		}
	}
	id := randomToken()
	now := time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO society_rules VALUES(?,?,?,?,?,?,?,?,'PENDING',?,?,?,1)`, id, optionalID(in.ReplacesID), in.Title, in.Text, in.PolicyReference, in.EffectiveFrom, in.EffectiveUntil, in.FinePermitted, p.ID, now, now)
	if e != nil {
		return "", e
	}
	x, e := scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", id))
	if e != nil {
		return "", e
	}
	if e = ruleEvent(ctx, tx, p, x, "PROPOSED", "Supplied rule submitted for separate review."); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecideRule(ctx context.Context, token, id string, in RuleAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 2000) || (in.Action != "PUBLISHED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "RETIRED") {
		return "", ErrInvalid
	}
	tx, p, e := s.beginUpkeepWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if e = incidentOperator(p); e != nil {
		return "", e
	}
	x, e := scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", id))
	if e != nil {
		return "", e
	}
	if (in.Action == "WITHDRAWN" && x.AuthorID != p.ID) || ((in.Action == "PUBLISHED" || in.Action == "DECLINED") && x.AuthorID == p.ID) {
		return "", ErrForbidden
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "RULE_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	if !((x.State == "PENDING" && (in.Action == "PUBLISHED" || in.Action == "DECLINED" || in.Action == "WITHDRAWN")) || (x.State == "PUBLISHED" && in.Action == "RETIRED")) {
		return "", ErrConflict
	}
	if in.Action == "PUBLISHED" && x.ReplacesID != "" {
		parent, e := scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", x.ReplacesID))
		if e != nil {
			return "", e
		}
		if parent.State != "PUBLISHED" {
			return "", ErrConflict
		}
		parent.State = "RETIRED"
		parent.Version++
		parent.UpdatedAt = time.Now().Unix()
		if _, e = tx.ExecContext(ctx, `UPDATE society_rules SET state='RETIRED',version=?,updated_at=? WHERE id=?`, parent.Version, parent.UpdatedAt, parent.ID); e != nil {
			return "", e
		}
		if e = ruleEvent(ctx, tx, p, parent, "REPLACED", in.Reason); e != nil {
			return "", e
		}
	}
	x.State = in.Action
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	if _, e = tx.ExecContext(ctx, `UPDATE society_rules SET state=?,version=?,updated_at=? WHERE id=?`, x.State, x.Version, x.UpdatedAt, id); e != nil {
		return "", e
	}
	if e = ruleEvent(ctx, tx, p, x, in.Action, in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
