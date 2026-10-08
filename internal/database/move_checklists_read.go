package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func moveChecklistIn(ctx context.Context, q identityReader, p Principal, id string) (MoveChecklist, error) {
	var out MoveChecklist
	h, e := moveHeadIn(ctx, q, id)
	if e != nil {
		return out, e
	}
	if !mayReadMoveChecklist(p, h) {
		return out, sql.ErrNoRows
	}
	x, e := moveSnapshotIn(ctx, q, id, h.Latest)
	if e != nil {
		return out, e
	}
	source, e := moveSourceIn(ctx, q, h.FlatID, h.ResidentID)
	if e != nil {
		return out, e
	}
	out = MoveChecklist{ID: id, Version: h.Version, Phase: h.Phase, State: h.Phase, Pending: h.Pending > 0, Snapshot: x, Source: source}
	if h.Approved > 0 {
		approved, err := moveSnapshotIn(ctx, q, id, h.Approved)
		if err != nil {
			return out, err
		}
		out.Approved = &approved
		out.State = "COMPLETED"
	}
	checked, na, stale, ready := moveChecksCurrent(x, source.Key)
	out.Checked = checked
	out.NotApplicable = na
	out.StaleChecks = stale
	own := ownMoveChecklist(p, h)
	staff := p.CanManageRegistry
	proposalOwner := x.ProposedBy == p.ID
	out.CanRevise = out.Pending && (h.Phase == "CHECKING" || h.Phase == "INFO") && proposalOwner && (staff || (own && source.Current))
	out.CanCheck = staff && out.Pending && (h.Phase == "CHECKING" || h.Phase == "INFO")
	out.CanReady = staff && out.Pending && h.Phase == "CHECKING" && ready
	out.CanReturn = staff && out.Pending && h.Phase == "READY"
	out.CanDecide = staff && out.Pending && p.ID != h.AuthorID && p.ID != x.ProposedBy && p.ID != x.ReadyBy
	out.CanComplete = out.CanDecide && h.Phase == "READY" && ready && x.SourceKey == source.Key
	out.CanCancel = out.Pending && proposalOwner && (staff || own)
	out.CanCorrect = h.Approved > 0 && !out.Pending && (staff || (own && source.Current))
	out.CanOpenHome = staff || (own && source.Current)
	out.CanOpenContact = out.CanOpenHome && p.CanReadContacts
	if !staff && !source.Current {
		out.Source = MoveChecklistSource{Current: false, Key: source.Key}
	}
	return out, nil
}
func moveChecklistScope(p Principal) (string, []any) {
	if p.CanManageRegistry {
		return "1=1", nil
	}
	return "h.author_id=? AND h.resident_id=?", []any{p.ID, p.ResidentID}
}
func moveChecklistFilter(state string) (string, bool) {
	switch state {
	case "":
		return "1=1", true
	case "CHECKING", "READY", "INFO":
		return "h.pending_version IS NOT NULL AND h.phase='" + state + "'", true
	case "PENDING":
		return "h.pending_version IS NOT NULL", true
	case "COMPLETED":
		return "h.approved_version IS NOT NULL", true
	case "CLOSED":
		return "h.pending_version IS NULL AND h.approved_version IS NULL", true
	}
	return "", false
}
func (s *Store) MoveChecklistsFor(ctx context.Context, token, query, state string, page int) (MoveChecklistPage, error) {
	out := MoveChecklistPage{Items: []MoveChecklist{}, Counts: map[string]int64{}, Page: page, PageSize: 12}
	filter, valid := moveChecklistFilter(state)
	query = strings.TrimSpace(query)
	if !valid || page < 1 || page > 10000 || !validText(query, 0, 100) {
		return out, ErrInvalid
	}
	tx, p, e := s.beginMoveRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := moveChecklistScope(p)
	var checking, ready, info, complete, closed int64
	e = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN h.pending_version IS NOT NULL AND h.phase='CHECKING' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN h.pending_version IS NOT NULL AND h.phase='READY' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN h.pending_version IS NOT NULL AND h.phase='INFO' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN h.approved_version IS NOT NULL THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN h.pending_version IS NULL AND h.approved_version IS NULL THEN 1 ELSE 0 END),0) FROM move_checklist_resources h WHERE `+scope, args...).Scan(&checking, &ready, &info, &complete, &closed)
	if e != nil {
		return out, e
	}
	out.Counts = map[string]int64{"checking": checking, "ready": ready, "info": info, "completed": complete, "closed": closed}
	where := scope + " AND " + filter
	values := append([]any{}, args...)
	query = strings.TrimSpace(query)
	if query != "" {
		where += ` AND (json_extract(v.snapshot_json,'$.person_name') LIKE ? OR json_extract(v.snapshot_json,'$.home_label') LIKE ? OR json_extract(v.snapshot_json,'$.note') LIKE ?)`
		values = append(values, "%"+query+"%", "%"+query+"%", "%"+query+"%")
	}
	from := ` FROM move_checklist_resources h JOIN move_checklist_versions v ON v.resource_id=h.id AND v.version=h.latest_version WHERE `
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from+where, values...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT h.id"+from+where+" ORDER BY h.created_at DESC,h.id LIMIT 12 OFFSET ?", append(values, (page-1)*12)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		item, err := moveChecklistIn(ctx, tx, p, id)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	out.CanPrepare = p.CanManageRegistry
	if !out.CanPrepare {
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, p.ResidentID, today(), today()).Scan(&out.CanPrepare)
		if e != nil {
			return out, e
		}
	}
	raw, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(string(raw))
	return out, tx.Commit()
}
func (s *Store) MoveChecklistFor(ctx context.Context, token, id string, page int) (MoveChecklistDetail, error) {
	out := MoveChecklistDetail{Events: []MoveChecklistEvent{}, EventPage: page, PageSize: 12}
	if len(id) < 1 || len(id) > 100 || page < 1 || page > 10000 {
		return out, ErrInvalid
	}
	tx, p, e := s.beginMoveRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.MoveChecklist, e = moveChecklistIn(ctx, tx, p, id)
	if e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM move_checklist_events WHERE resource_id=?", id).Scan(&out.EventTotal); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT e.version,e.action,u.display_name,e.occurred_at,e.reason,v.snapshot_json FROM move_checklist_events e JOIN users u ON u.id=e.actor_id JOIN move_checklist_versions v ON v.resource_id=e.resource_id AND v.version=e.version WHERE e.resource_id=? ORDER BY e.version DESC LIMIT 12 OFFSET ?`, id, (page-1)*12)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var event MoveChecklistEvent
		var raw string
		if e = rows.Scan(&event.Version, &event.Action, &event.Actor, &event.At, &event.Reason, &raw); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(raw), &event.Snapshot); e != nil {
			rows.Close()
			return out, e
		}
		out.Events = append(out.Events, event)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	out.CurrentKey = TokenHash(fmt.Sprintf("%s:%d:%s:%d:%d", id, out.Version, out.Source.Key, out.EventTotal, page))
	return out, tx.Commit()
}

type MoveChecklistPerson struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type MoveChecklistOptions struct {
	Homes      []RecordHome          `json:"homes"`
	People     []MoveChecklistPerson `json:"people"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	CanPrepare bool                  `json:"can_prepare"`
}

func (s *Store) MoveChecklistOptionsFor(ctx context.Context, token, query, home string, page int) (MoveChecklistOptions, error) {
	out := MoveChecklistOptions{Homes: []RecordHome{}, People: []MoveChecklistPerson{}, Page: page, PageSize: 12}
	query = strings.TrimSpace(query)
	if !validText(query, 0, 100) || len(home) > 100 || page < 1 || page > 10000 {
		return out, ErrInvalid
	}
	tx, p, e := s.beginMoveRead(ctx, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	date := today()
	scope := `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
	args := []any{date, date}
	if !p.CanManageRegistry {
		scope = `EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = []any{p.ResidentID, date, date}
	}
	query = strings.TrimSpace(query)
	if query != "" {
		scope += ` AND b.code||'-'||f.flat_number LIKE ?`
		args = append(args, "%"+query+"%")
	}
	from := ` FROM flats f JOIN buildings b ON b.id=f.building_id WHERE ` + scope
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.CanPrepare = out.Total > 0
	rows, e := tx.QueryContext(ctx, `SELECT f.id,b.code||'-'||f.flat_number`+from+` ORDER BY b.code,f.flat_number,f.id LIMIT 12 OFFSET ?`, append(args, (page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var h RecordHome
		if e = rows.Scan(&h.ID, &h.Label); e != nil {
			rows.Close()
			return out, e
		}
		out.Homes = append(out.Homes, h)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if home != "" {
		where := `m.flat_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)`
		values := []any{home, date, date}
		if !p.CanManageRegistry {
			where += " AND m.resident_id=?"
			values = append(values, p.ResidentID)
		}
		rows, e = tx.QueryContext(ctx, `SELECT DISTINCT r.id,r.full_name FROM flat_memberships m JOIN residents r ON r.id=m.resident_id WHERE `+where+` ORDER BY r.full_name,r.id LIMIT 101`, values...)
		if e != nil {
			return out, e
		}
		for rows.Next() {
			var person MoveChecklistPerson
			if e = rows.Scan(&person.ID, &person.Name); e != nil {
				rows.Close()
				return out, e
			}
			out.People = append(out.People, person)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
		if len(out.People) == 0 {
			return out, sql.ErrNoRows
		}
		if len(out.People) > 100 {
			return out, invalid("This home's person choices exceed the bounded review limit.")
		}
	}
	return out, tx.Commit()
}
