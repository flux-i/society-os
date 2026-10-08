package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type MoveChecklistInput struct {
	OperationKey  string `json:"operation_key"`
	Version       int    `json:"version"`
	FlatID        string `json:"flat_id"`
	ResidentID    string `json:"resident_id"`
	Kind          string `json:"kind"`
	EffectiveDate string `json:"effective_date"`
	Note          string `json:"note"`
	Reason        string `json:"reason"`
	Confirmed     bool   `json:"confirmed"`
}
type MoveChecklistAction struct {
	OperationKey  string `json:"operation_key"`
	Version       int    `json:"version"`
	Action        string `json:"action"`
	CheckKind     string `json:"check_kind"`
	CheckState    string `json:"check_state"`
	Reference     string `json:"reference"`
	SourceKey     string `json:"source_key"`
	EffectiveDate string `json:"effective_date"`
	Note          string `json:"note"`
	Reason        string `json:"reason"`
	Confirmed     bool   `json:"confirmed"`
}
type MoveChecklistCheck struct {
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Reference string `json:"reference"`
	CheckedBy string `json:"checked_by"`
	CheckedAt int64  `json:"checked_at"`
	SourceKey string `json:"source_key"`
}
type MoveChecklistSource struct {
	FlatVersion    int    `json:"registry_version"`
	ContactVersion int    `json:"contact_version"`
	ContactState   string `json:"contact_state"`
	Current        bool   `json:"current"`
	PersonName     string `json:"person_name"`
	HomeLabel      string `json:"home_label"`
	MembershipKey  string `json:"membership_key"`
	Key            string `json:"key"`
}
type MoveChecklistSnapshot struct {
	Version                 int                  `json:"version"`
	Action                  string               `json:"action"`
	Phase                   string               `json:"phase"`
	FlatID                  string               `json:"flat_id"`
	ResidentID              string               `json:"resident_id"`
	Kind                    string               `json:"kind"`
	AuthorID                string               `json:"author_id"`
	ProposedBy              string               `json:"proposed_by"`
	ReadyBy                 string               `json:"ready_by"`
	EffectiveDate           string               `json:"effective_date"`
	Note                    string               `json:"note"`
	PersonName              string               `json:"person_name"`
	HomeLabel               string               `json:"home_label"`
	Checks                  []MoveChecklistCheck `json:"checks"`
	SourceKey               string               `json:"source_key"`
	Source                  MoveChecklistSource  `json:"source"`
	PreviousApprovedVersion int                  `json:"previous_approved_version"`
	ActorID                 string               `json:"actor_id"`
	OccurredAt              int64                `json:"occurred_at"`
	Reason                  string               `json:"reason"`
}
type MoveChecklist struct {
	ID             string                 `json:"id"`
	Version        int                    `json:"version"`
	Phase          string                 `json:"phase"`
	State          string                 `json:"state"`
	Pending        bool                   `json:"pending"`
	Snapshot       MoveChecklistSnapshot  `json:"snapshot"`
	Approved       *MoveChecklistSnapshot `json:"approved,omitempty"`
	Source         MoveChecklistSource    `json:"source"`
	Checked        int                    `json:"checked"`
	NotApplicable  int                    `json:"not_applicable"`
	StaleChecks    int                    `json:"stale_checks"`
	CanRevise      bool                   `json:"can_revise"`
	CanCheck       bool                   `json:"can_check"`
	CanReady       bool                   `json:"can_ready"`
	CanReturn      bool                   `json:"can_return"`
	CanDecide      bool                   `json:"can_decide"`
	CanComplete    bool                   `json:"can_complete"`
	CanCancel      bool                   `json:"can_cancel"`
	CanCorrect     bool                   `json:"can_correct"`
	CanOpenHome    bool                   `json:"can_open_home"`
	CanOpenContact bool                   `json:"can_open_contact"`
}
type MoveChecklistEvent struct {
	Version  int                   `json:"version"`
	Action   string                `json:"action"`
	Actor    string                `json:"actor"`
	At       int64                 `json:"at"`
	Reason   string                `json:"reason"`
	Snapshot MoveChecklistSnapshot `json:"snapshot"`
}
type MoveChecklistDetail struct {
	MoveChecklist
	Events     []MoveChecklistEvent `json:"events"`
	EventTotal int                  `json:"event_total"`
	EventPage  int                  `json:"event_page"`
	PageSize   int                  `json:"page_size"`
	CurrentKey string               `json:"current_key"`
}
type MoveChecklistPage struct {
	Items      []MoveChecklist  `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Counts     map[string]int64 `json:"counts"`
	CanPrepare bool             `json:"can_prepare"`
	CurrentKey string           `json:"current_key"`
}
type moveChecklistHead struct {
	ID, FlatID, ResidentID, Kind, AuthorID, Phase string
	CreatedAt                                     int64
	Version, Latest, Pending, Approved            int
}

var moveChecklistKinds = map[string]bool{"MOVE_IN": true, "MOVE_OUT": true, "CONTACT_REVIEW": true}
var moveCheckKinds = []string{"IDENTITY", "REGISTRY", "CONTACT", "DOCUMENTS", "HANDOVER"}

func emptyMoveChecks() []MoveChecklistCheck {
	out := make([]MoveChecklistCheck, 0, 5)
	for _, kind := range moveCheckKinds {
		out = append(out, MoveChecklistCheck{Kind: kind, State: "INCOMPLETE"})
	}
	return out
}
func moveHeadIn(ctx context.Context, q identityReader, id string) (moveChecklistHead, error) {
	var h moveChecklistHead
	e := q.QueryRowContext(ctx, `SELECT id,flat_id,resident_id,kind,author_id,phase,created_at,version,latest_version,COALESCE(pending_version,0),COALESCE(approved_version,0) FROM move_checklist_resources WHERE id=?`, id).Scan(&h.ID, &h.FlatID, &h.ResidentID, &h.Kind, &h.AuthorID, &h.Phase, &h.CreatedAt, &h.Version, &h.Latest, &h.Pending, &h.Approved)
	return h, e
}
func moveSnapshotIn(ctx context.Context, q identityReader, id string, version int) (MoveChecklistSnapshot, error) {
	var out MoveChecklistSnapshot
	var raw string
	e := q.QueryRowContext(ctx, "SELECT snapshot_json FROM move_checklist_versions WHERE resource_id=? AND version=?", id, version).Scan(&raw)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &out)
	}
	return out, e
}
func ownMoveChecklist(p Principal, h moveChecklistHead) bool {
	return p.ResidentID != "" && p.ResidentID == h.ResidentID && p.ID == h.AuthorID
}
func mayReadMoveChecklist(p Principal, h moveChecklistHead) bool {
	return p.CanReadChecklists && (p.CanManageRegistry || ownMoveChecklist(p, h))
}
func requireMoveStaff(p Principal) error {
	if !p.CanManageRegistry {
		return ErrForbidden
	}
	if !p.Fresh {
		return ErrReauthRequired
	}
	return nil
}
func (s *Store) beginMoveRead(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, Principal{}, e
	}
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e == nil && !p.CanReadChecklists {
		e = ErrForbidden
	}
	if e != nil {
		tx.Rollback()
		return nil, p, e
	}
	return tx, p, nil
}
func (s *Store) beginMoveWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, Principal{}, e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); e != nil {
		tx.Rollback()
		return nil, Principal{}, e
	}
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e == nil && !p.CanReadChecklists {
		e = ErrForbidden
	}
	if e != nil {
		tx.Rollback()
		return nil, p, e
	}
	return tx, p, nil
}
func moveSourceIn(ctx context.Context, q identityReader, flat, person string) (MoveChecklistSource, error) {
	var out MoveChecklistSource
	e := q.QueryRowContext(ctx, `SELECT f.version,b.code||'-'||f.flat_number,r.full_name,COALESCE(c.version,0),COALESCE(c.state,'NONE') FROM flats f JOIN buildings b ON b.id=f.building_id JOIN residents r ON r.id=? LEFT JOIN resident_contacts c ON c.resident_id=r.id WHERE f.id=?`, person, flat).Scan(&out.FlatVersion, &out.HomeLabel, &out.PersonName, &out.ContactVersion, &out.ContactState)
	if e != nil {
		return out, e
	}
	type member struct {
		ID, Relationship, Start, End string
		Primary                      bool
	}
	members := []member{}
	rows, e := q.QueryContext(ctx, `SELECT id,relationship,start_date,COALESCE(end_date,''),is_primary_contact FROM flat_memberships WHERE flat_id=? AND resident_id=? ORDER BY id`, flat, person)
	if e != nil {
		return out, e
	}
	date := today()
	for rows.Next() {
		var m member
		if e = rows.Scan(&m.ID, &m.Relationship, &m.Start, &m.End, &m.Primary); e != nil {
			rows.Close()
			return out, e
		}
		members = append(members, m)
		if m.Start <= date && (m.End == "" || m.End > date) {
			out.Current = true
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	raw, e := json.Marshal(members)
	if e != nil {
		return out, e
	}
	out.MembershipKey = TokenHash(string(raw))
	raw, e = json.Marshal(out)
	if e == nil {
		out.Key = TokenHash(string(raw))
	}
	return out, e
}
func moveChecksCurrent(x MoveChecklistSnapshot, key string) (checked, na, stale int, ready bool) {
	ready = len(x.Checks) == 5
	seen := map[string]bool{}
	for _, c := range x.Checks {
		if seen[c.Kind] {
			ready = false
		}
		seen[c.Kind] = true
		if c.State == "CHECKED" {
			checked++
		} else if c.State == "NOT_APPLICABLE" && (c.Kind == "DOCUMENTS" || c.Kind == "HANDOVER") {
			na++
		} else {
			ready = false
		}
		if c.State != "INCOMPLETE" && c.SourceKey != key {
			stale++
			ready = false
		}
	}
	for _, kind := range moveCheckKinds {
		if !seen[kind] {
			ready = false
		}
	}
	return
}
func normaliseMoveInput(in MoveChecklistInput) (MoveChecklistInput, error) {
	in.Note = strings.TrimSpace(in.Note)
	in.Reason = strings.TrimSpace(in.Reason)
	if !in.Confirmed || in.Version != 0 || len(in.FlatID) < 1 || len(in.FlatID) > 100 || len(in.ResidentID) > 100 || !moveChecklistKinds[in.Kind] || !validMaintenanceDate(in.EffectiveDate) || !paragraph(in.Note, 10, 800) || !paragraph(in.Reason, 10, 800) {
		return in, invalid("Supply the exact current home, checklist kind, date, explanation and confirmation.")
	}
	return in, nil
}
func normaliseMoveAction(in MoveChecklistAction) (MoveChecklistAction, error) {
	in.Note = strings.TrimSpace(in.Note)
	in.Reason = strings.TrimSpace(in.Reason)
	in.Reference = strings.TrimSpace(in.Reference)
	if !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 800) || !exclusiveMoveFields(in) {
		return in, invalid("Confirm the exact checklist version and give a reason.")
	}
	switch in.Action {
	case "CHECK":
		if in.CheckState != "CHECKED" && in.CheckState != "NOT_APPLICABLE" {
			return in, ErrInvalid
		}
		found := false
		for _, kind := range moveCheckKinds {
			found = found || in.CheckKind == kind
		}
		if !found || !paragraph(in.Reference, 5, 300) || (in.CheckState == "NOT_APPLICABLE" && in.CheckKind != "DOCUMENTS" && in.CheckKind != "HANDOVER") {
			return in, invalid("Use an explicit source reference; required checks cannot be marked not applicable.")
		}
	case "REVISE", "CORRECTION":
		if !validMaintenanceDate(in.EffectiveDate) || !paragraph(in.Note, 10, 800) {
			return in, ErrInvalid
		}
	case "READY", "APPROVED":
		if len(in.SourceKey) != 64 {
			return in, invalid("Reload and review this exact current source before confirming.")
		}
	case "RETURN", "INFO", "DECLINED", "CANCELLED":
	default:
		return in, ErrInvalid
	}
	return in, nil
}
