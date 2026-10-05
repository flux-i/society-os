package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

const upkeepRegisterSelect = `SELECT r.id,r.kind,r.name,r.category,r.location,r.contact,r.phone,r.email,COALESCE(r.vendor_id,''),COALESCE(v.name,''),r.source_reference,r.amc_start,r.amc_end,r.inspection_date,r.state,r.version,r.created_by,r.created_at,r.updated_at FROM upkeep_register r LEFT JOIN upkeep_register v ON v.id=r.vendor_id`

func scanUpkeepRegister(row interface{ Scan(...any) error }) (UpkeepRegister, error) {
	var r UpkeepRegister
	err := row.Scan(&r.ID, &r.Kind, &r.Name, &r.Category, &r.Location, &r.Contact, &r.Phone, &r.Email, &r.VendorID, &r.Vendor, &r.SourceReference, &r.AMCStart, &r.AMCEnd, &r.InspectionDate, &r.State, &r.Version, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func readUpkeepEvents(ctx context.Context, tx *sql.Tx, table, field, id string, page int) ([]UpkeepEvent, int, int, error) {
	events := []UpkeepEvent{}
	var total int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE "+field+"=?", id).Scan(&total); err != nil {
		return events, 0, 0, err
	}
	page = clampMaintenancePage(page, total, 20)
	rows, err := tx.QueryContext(ctx, "SELECT e.action,u.display_name,e.reason,e.version,e.occurred_at FROM "+table+" e JOIN users u ON u.id=e.actor_id WHERE e."+field+"=? ORDER BY e.id DESC LIMIT 20 OFFSET ?", id, (page-1)*20)
	if err != nil {
		return events, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var e UpkeepEvent
		if err = rows.Scan(&e.Action, &e.Actor, &e.Reason, &e.Version, &e.At); err != nil {
			return events, 0, 0, err
		}
		events = append(events, e)
	}
	return events, total, page, rows.Err()
}

func (s *Store) UpkeepRegisterFor(ctx context.Context, token, kind, query, state string, page int) (UpkeepRegisterPage, error) {
	out := UpkeepRegisterPage{Items: []UpkeepRegister{}, PageSize: 12}
	if !upkeepKind(kind) || !validText(query, 0, 100) || (state != "" && state != "ACTIVE" && state != "INACTIVE") || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, _, err := beginUpkeepRead(ctx, s, token, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	where, args := "r.kind=?", []any{kind}
	if state != "" {
		where += " AND r.state=?"
		args = append(args, state)
	}
	if query != "" {
		where += " AND instr(lower(r.name),?)>0"
		args = append(args, strings.ToLower(strings.TrimSpace(query)))
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM upkeep_register r WHERE "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	rows, err := tx.QueryContext(ctx, upkeepRegisterSelect+" WHERE "+where+" ORDER BY r.state,r.name,r.id LIMIT ? OFFSET ?", append(args, out.PageSize, (out.Page-1)*out.PageSize)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		r, e := scanUpkeepRegister(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) UpkeepRegisterDetailFor(ctx context.Context, token, kind, id string, page int) (UpkeepRegisterDetail, error) {
	out := UpkeepRegisterDetail{PageSize: 20}
	if !upkeepKind(kind) || len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, _, err := beginUpkeepRead(ctx, s, token, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	out.UpkeepRegister, err = scanUpkeepRegister(tx.QueryRowContext(ctx, upkeepRegisterSelect+" WHERE r.id=? AND r.kind=?", id, kind))
	if err != nil {
		return out, err
	}
	out.Events, out.EventTotal, out.EventPage, err = readUpkeepEvents(ctx, tx, "upkeep_register_events", "register_id", id, page)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) UpkeepOptionsFor(ctx context.Context, token string) (UpkeepOptions, error) {
	out := UpkeepOptions{Vendors: []RecordHome{}, Assets: []RecordHome{}, Handlers: []ComplaintHandler{}, Buildings: []RecordHome{}}
	tx, _, err := beginUpkeepRead(ctx, s, token, true)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id,name,kind FROM upkeep_register WHERE state='ACTIVE' ORDER BY name,id")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var h RecordHome
		var kind string
		if err = rows.Scan(&h.ID, &h.Label, &kind); err != nil {
			rows.Close()
			return out, err
		}
		if kind == "VENDOR" {
			out.Vendors = append(out.Vendors, h)
		} else {
			out.Assets = append(out.Assets, h)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT u.id,u.display_name FROM users u WHERE u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id) AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=u.id AND g.role IN ('ADMINISTRATOR','COMMITTEE') AND g.valid_from<=unixepoch() AND g.valid_until>unixepoch() AND g.revoked_at IS NULL) ORDER BY u.display_name,u.id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var h ComplaintHandler
		if err = rows.Scan(&h.ID, &h.Name); err != nil {
			rows.Close()
			return out, err
		}
		out.Handlers = append(out.Handlers, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT code,'Wing '||code FROM buildings ORDER BY code")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			rows.Close()
			return out, err
		}
		out.Buildings = append(out.Buildings, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

const upkeepTaskSelect = `SELECT t.id,t.title,t.body,t.category,t.priority,t.due_date,t.visit_date,COALESCE(t.asset_id,''),COALESCE(a.name,''),COALESCE(t.vendor_id,''),COALESCE(v.name,''),COALESCE(t.assigned_to,''),COALESCE(u.display_name,''),
 EXISTS(SELECT 1 FROM users x WHERE x.id=t.assigned_to AND x.status='ACTIVE' AND x.suspended_at IS NULL AND x.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=x.id) AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=x.id AND g.role IN ('ADMINISTRATOR','COMMITTEE') AND g.valid_from<=unixepoch() AND g.valid_until>unixepoch() AND g.revoked_at IS NULL)),
 COALESCE(t.complaint_id,''),COALESCE(c.complaint_number,''),t.repeat_days,COALESCE(t.parent_id,''),t.state,t.version,t.created_by,t.created_at,t.updated_at,COALESCE(t.ready_by,''),COALESCE(ru.display_name,''),COALESCE(t.ready_at,0),COALESCE(t.checked_by,''),COALESCE(cu.display_name,''),COALESCE(t.checked_at,0),t.audience,t.building_code,t.public_version,t.public_snapshot_json,COALESCE(t.published_at,0)
 FROM upkeep_tasks t LEFT JOIN upkeep_register a ON a.id=t.asset_id LEFT JOIN upkeep_register v ON v.id=t.vendor_id LEFT JOIN users u ON u.id=t.assigned_to LEFT JOIN users ru ON ru.id=t.ready_by LEFT JOIN users cu ON cu.id=t.checked_by LEFT JOIN complaints c ON c.id=t.complaint_id`

func scanUpkeepTask(row interface{ Scan(...any) error }, operator bool) (UpkeepTask, error) {
	var t UpkeepTask
	var snapshot string
	err := row.Scan(&t.ID, &t.Title, &t.Body, &t.Category, &t.Priority, &t.DueDate, &t.VisitDate, &t.AssetID, &t.Asset, &t.VendorID, &t.Vendor, &t.AssignedTo, &t.AssignedName, &t.AssigneeEligible, &t.ComplaintID, &t.ComplaintNumber, &t.RepeatDays, &t.ParentID, &t.State, &t.Version, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.ReadyBy, &t.ReadyName, &t.ReadyAt, &t.CheckedBy, &t.CheckedName, &t.CheckedAt, &t.Audience, &t.BuildingCode, &t.PublicVersion, &snapshot, &t.PublishedAt)
	if err != nil {
		return t, err
	}
	if snapshot != "" {
		var public UpkeepPublication
		if err = json.Unmarshal([]byte(snapshot), &public); err != nil {
			return t, err
		}
		t.PublicSnapshot = &public
	}
	if !operator {
		if t.PublicSnapshot == nil {
			return UpkeepTask{}, sql.ErrNoRows
		}
		pub := t.PublicSnapshot
		// Construct the response from the frozen publication. Private versions, times,
		// assignments, service links and events cannot leak through a future field.
		t = UpkeepTask{ID: t.ID, Title: pub.Title, Body: pub.Body, Category: pub.Category, Priority: pub.Priority, DueDate: pub.DueDate, VisitDate: pub.VisitDate, State: pub.State, Version: t.PublicVersion, CreatedAt: t.PublishedAt, UpdatedAt: t.PublishedAt, PublishedAt: t.PublishedAt, Audience: t.Audience, BuildingCode: t.BuildingCode}
	}
	return t, nil
}
func upkeepTaskScope(p Principal) (string, []any) {
	if p.CanHandleComplaints {
		return "1=1", nil
	}
	return `t.audience!='INTERNAL' AND EXISTS(SELECT 1 FROM flat_memberships m JOIN flats f ON f.id=m.flat_id JOIN buildings b ON b.id=f.building_id WHERE m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?) AND (t.audience='ALL_RESIDENTS' OR (t.audience='BUILDING' AND b.code=t.building_code)))`, []any{p.ResidentID, today(), today()}
}
func upkeepVisible(field string, p Principal) string {
	if p.CanHandleComplaints {
		return "t." + field
	}
	return "json_extract(t.public_snapshot_json,'$." + field + "')"
}
func upkeepTaskCounts(ctx context.Context, tx *sql.Tx, p Principal, out map[string]int64) error {
	scope, args := upkeepTaskScope(p)
	state, due, visit := upkeepVisible("state", p), upkeepVisible("due_date", p), upkeepVisible("visit_date", p)
	open := state + " NOT IN ('DONE','CANCELLED')"
	unassigned, checks := "0", "0"
	if p.CanHandleComplaints {
		unassigned = open + " AND (t.assigned_to IS NULL OR NOT EXISTS(SELECT 1 FROM users u WHERE u.id=t.assigned_to AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id) AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=u.id AND g.role IN ('ADMINISTRATOR','COMMITTEE') AND g.valid_from<=unixepoch() AND g.valid_until>unixepoch() AND g.revoked_at IS NULL)))"
		checks = "t.state='READY_FOR_CHECK' AND t.ready_by!=?"
	}
	tomorrow := time.Now().In(societyZone).AddDate(0, 0, 7).Format("2006-01-02")
	// Parameters in the outer expressions precede the scoped WHERE.
	params := []any{today(), today(), tomorrow}
	if p.CanHandleComplaints {
		params = append([]any{today(), p.ID, today(), tomorrow}, args...)
	} else {
		params = append(params, args...)
	}
	names := []string{"open", "overdue", "unassigned", "ready_for_check", "upcoming_visits", "published"}
	values := make([]int64, len(names))
	ptrs := make([]any, len(names))
	for i := range ptrs {
		ptrs[i] = &values[i]
	}
	query := `SELECT COUNT(CASE WHEN ` + open + ` THEN 1 END),COUNT(CASE WHEN ` + open + ` AND ` + due + `<? THEN 1 END),COUNT(CASE WHEN ` + unassigned + ` THEN 1 END),COUNT(CASE WHEN ` + checks + ` THEN 1 END),COUNT(CASE WHEN ` + open + ` AND ` + visit + ` BETWEEN ? AND ? THEN 1 END),COUNT(CASE WHEN t.audience!='INTERNAL' THEN 1 END) FROM upkeep_tasks t WHERE ` + scope
	if err := tx.QueryRowContext(ctx, query, params...).Scan(ptrs...); err != nil {
		return err
	}
	for i, name := range names {
		out[name] = values[i]
	}
	return nil
}
func (s *Store) UpkeepTasksFor(ctx context.Context, token, query, state string, page int) (UpkeepTaskPage, error) {
	out := UpkeepTaskPage{Items: []UpkeepTask{}, PageSize: 12, Counts: map[string]int64{}}
	if !validText(query, 0, 100) || (state != "" && !upkeepState(state)) || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, err := beginUpkeepRead(ctx, s, token, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = upkeepTaskCounts(ctx, tx, p, out.Counts); err != nil {
		return out, err
	}
	scope, args := upkeepTaskScope(p)
	if query != "" {
		scope += " AND instr(lower(" + upkeepVisible("title", p) + "),?)>0"
		args = append(args, strings.ToLower(strings.TrimSpace(query)))
	}
	if state != "" {
		scope += " AND " + upkeepVisible("state", p) + "=?"
		args = append(args, state)
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM upkeep_tasks t WHERE "+scope, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	rows, err := tx.QueryContext(ctx, upkeepTaskSelect+" WHERE "+scope+" ORDER BY ("+upkeepVisible("state", p)+" IN ('DONE','CANCELLED')),"+upkeepVisible("due_date", p)+",t.id LIMIT ? OFFSET ?", append(args, out.PageSize, (out.Page-1)*out.PageSize)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		t, e := scanUpkeepTask(rows, p.CanHandleComplaints)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) UpkeepTaskFor(ctx context.Context, token, id string, page int) (UpkeepDetail, error) {
	out := UpkeepDetail{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, err := beginUpkeepRead(ctx, s, token, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	scope, args := upkeepTaskScope(p)
	out.UpkeepTask, err = scanUpkeepTask(tx.QueryRowContext(ctx, upkeepTaskSelect+" WHERE "+scope+" AND t.id=?", append(args, id)...), p.CanHandleComplaints)
	if err != nil {
		return out, err
	}
	if p.CanHandleComplaints {
		out.Events, out.EventTotal, out.EventPage, err = readUpkeepEvents(ctx, tx, "upkeep_task_events", "task_id", id, page)
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}
