package database

import (
	"context"
	"database/sql"
	"time"
)

func (s *Store) RulesFor(ctx context.Context, token, query, state string, page int) (RulePage, error) {
	out := RulePage{Items: []Rule{}, Page: page, PageSize: 12}
	if e := incidentReadInput(query, state, page); e != nil {
		return out, e
	}
	if state != "" && state != "PENDING" && state != "PUBLISHED" && state != "RETIRED" && state != "DECLINED" && state != "WITHDRAWN" {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	yes, e := incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	if !yes {
		return out, ErrForbidden
	}
	scope := "1=1"
	args := []any{}
	if !p.CanHandleComplaints {
		scope = "state IN('PUBLISHED','RETIRED')"
	}
	if query != "" {
		scope += " AND instr(lower(title),lower(?))>0"
		args = append(args, query)
	}
	if state != "" {
		scope += " AND state=?"
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM society_rules WHERE "+scope, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, ruleSelect+" WHERE "+scope+" ORDER BY updated_at DESC,rowid DESC LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, e := scanRule(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		if !p.CanHandleComplaints {
			x.AuthorID = ""
		}
		out.Items = append(out.Items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) RuleFor(ctx context.Context, token, id string, eventPage int) (Rule, error) {
	var out Rule
	if eventPage < 1 || eventPage > 100000 {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	yes, e := incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	if !yes {
		return out, ErrForbidden
	}
	out, e = scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", id))
	if e != nil {
		return out, e
	}
	if !p.CanHandleComplaints {
		if out.State != "PUBLISHED" && out.State != "RETIRED" {
			return Rule{}, sql.ErrNoRows
		}
		out.AuthorID = ""
	} else {
		out.Events, out.EventTotal, out.EventPage, e = incidentEvents(ctx, tx, "society_rule_events", "rule_id", id, true, eventPage)
		out.PageSize = 20
		if e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func incidentEvents(ctx context.Context, tx *sql.Tx, table, field, id string, operator bool, page int) ([]FundEvent, int, int, error) {
	out := []FundEvent{}
	total := 0
	scope := "x." + field + "=?"
	version := "x.version"
	if !operator {
		scope += " AND x.reporter_visible=1"
		version = "x.reporter_version"
	}
	if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" x WHERE "+scope, id).Scan(&total); e != nil {
		return out, total, page, e
	}
	page = incidentPageBound(page, total, 20)
	rows, e := tx.QueryContext(ctx, "SELECT x.action,u.display_name,x.reason,"+version+",x.occurred_at FROM "+table+" x JOIN users u ON u.id=x.actor_id WHERE "+scope+" ORDER BY x.id DESC LIMIT 20 OFFSET ?", id, (page-1)*20)
	if e != nil {
		return out, total, page, e
	}
	defer rows.Close()
	for rows.Next() {
		var x FundEvent
		if e = rows.Scan(&x.Action, &x.Actor, &x.Reason, &x.Version, &x.At); e != nil {
			return out, total, page, e
		}
		out = append(out, x)
	}
	return out, total, page, rows.Err()
}

const incidentSelect = `SELECT c.id,c.rule_id,r.title,c.flat_id,b.code||'-'||f.flat_number,c.reporter_id,c.incident_date,c.comment,COALESCE(c.picture_id,''),c.state,COALESCE(c.duplicate_of,''),COALESCE(c.notice_id,''),c.created_at,c.updated_at,c.version,c.reporter_version,c.reporter_updated_at FROM incidents c JOIN society_rules r ON r.id=c.rule_id JOIN flats f ON f.id=c.flat_id JOIN buildings b ON b.id=f.building_id`

func scanIncident(row interface{ Scan(...any) error }) (Incident, error) {
	var x Incident
	e := row.Scan(&x.ID, &x.RuleID, &x.RuleTitle, &x.FlatID, &x.Home, &x.ReporterID, &x.IncidentDate, &x.Comment, &x.PictureID, &x.State, &x.DuplicateOf, &x.NoticeID, &x.CreatedAt, &x.UpdatedAt, &x.Version, &x.ReporterVersion, &x.ReporterUpdatedAt)
	return x, e
}
func incidentScope(p Principal) (string, []any) {
	if p.CanHandleComplaints {
		return "1=1", []any{}
	}
	return "c.reporter_id=?", []any{p.ID}
}
func incidentReporterSnapshot(x Incident, p Principal) Incident {
	if !p.CanHandleComplaints {
		x.Version = x.ReporterVersion
		x.UpdatedAt = x.ReporterUpdatedAt
		x.DuplicateOf = ""
		x.NoticeID = ""
	}
	return x
}
func incidentState(state string) bool {
	return state == "REPORTED" || state == "NEEDS_INFO" || state == "UNDER_REVIEW" || state == "DISMISSED" || state == "SUBSTANTIATED" || state == "WITHDRAWN"
}
func (s *Store) IncidentsFor(ctx context.Context, token, query, state string, page int) (IncidentPage, error) {
	out := IncidentPage{Items: []Incident{}, Page: page, PageSize: 12}
	if e := incidentReadInput(query, state, page); e != nil {
		return out, e
	}
	if state != "" && !incidentState(state) {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.CanReport, e = incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	scope, args := incidentScope(p)
	if state != "" {
		scope += " AND c.state=?"
		args = append(args, state)
	}
	if query != "" {
		scope += " AND instr(lower(r.title||' '||b.code||'-'||f.flat_number||' '||c.incident_date||' '||c.id),lower(?))>0"
		args = append(args, query)
	}
	from := ` FROM incidents c JOIN society_rules r ON r.id=c.rule_id JOIN flats f ON f.id=c.flat_id JOIN buildings b ON b.id=f.building_id WHERE `
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from+scope, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	order := "c.updated_at"
	if !p.CanHandleComplaints {
		order = "c.reporter_updated_at"
	}
	rows, e := tx.QueryContext(ctx, incidentSelect+" WHERE "+scope+" ORDER BY "+order+" DESC,c.rowid DESC LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, e := scanIncident(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, incidentReporterSnapshot(x, p))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) IncidentFor(ctx context.Context, token, id string, eventPage int) (IncidentDetail, error) {
	out := IncidentDetail{Events: []FundEvent{}, PageSize: 20}
	if eventPage < 1 || eventPage > 100000 {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := incidentScope(p)
	out.Incident, e = scanIncident(tx.QueryRowContext(ctx, incidentSelect+" WHERE c.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return out, e
	}
	out.CanParticipate, e = incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	out.Rule, e = scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", out.RuleID))
	if e != nil {
		return out, e
	}
	out.Events, out.EventTotal, out.EventPage, e = incidentEvents(ctx, tx, "incident_events", "incident_id", id, p.CanHandleComplaints, eventPage)
	if e != nil {
		return out, e
	}
	if p.CanHandleComplaints {
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM incident_notices WHERE incident_id=?", id).Scan(&out.NoticeTotal); e != nil {
			return out, e
		}
		if e = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM incident_responses s JOIN incident_notices n ON n.id=s.notice_id WHERE n.incident_id=?`, id).Scan(&out.ResponseTotal); e != nil {
			return out, e
		}
		rows, e := tx.QueryContext(ctx, noticeSelect+" WHERE n.incident_id=? ORDER BY n.version DESC LIMIT 20", id)
		if e != nil {
			return out, e
		}
		out.Notices = []IncidentNotice{}
		for rows.Next() {
			x, e := scanIncidentNotice(rows)
			if e != nil {
				rows.Close()
				return out, e
			}
			out.Notices = append(out.Notices, x)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
	} else {
		out.Rule.AuthorID = ""
	}
	out.Incident = incidentReporterSnapshot(out.Incident, p)
	return out, tx.Commit()
}
func (s *Store) IncidentReportOptions(ctx context.Context, token, query string) (IncidentOptions, error) {
	out := IncidentOptions{Homes: []RecordHome{}}
	if !validText(query, 0, 100) || len(query) > 400 {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.CanReport, e = incidentParticipant(ctx, tx, p)
	if e != nil {
		return out, e
	}
	if !out.CanReport {
		return out, ErrForbidden
	}
	rows, e := tx.QueryContext(ctx, `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE instr(lower(b.code||'-'||f.flat_number),lower(?))>0 ORDER BY b.code,f.floor,f.flat_number LIMIT 500`, query)
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
	return out, tx.Commit()
}

const noticeSelect = `SELECT n.id,n.incident_id,n.flat_id,b.code||'-'||f.flat_number,n.rule_id,n.incident_date,n.title,n.body,n.response_by,COALESCE(n.picture_id,''),n.created_at,n.version,COALESCE(c.notice_id=n.id,0) FROM incident_notices n JOIN incidents c ON c.id=n.incident_id JOIN flats f ON f.id=n.flat_id JOIN buildings b ON b.id=f.building_id`

func scanIncidentNotice(row interface{ Scan(...any) error }) (IncidentNotice, error) {
	var x IncidentNotice
	e := row.Scan(&x.ID, &x.IncidentID, &x.FlatID, &x.Home, &x.RuleID, &x.IncidentDate, &x.Title, &x.Body, &x.ResponseBy, &x.PictureID, &x.CreatedAt, &x.Version, &x.Active)
	return x, e
}
func incidentNoticeScope(p Principal) (string, []any) {
	return `c.notice_id=n.id AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=n.flat_id AND m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ResidentID, today(), today()}
}
func (s *Store) IncidentNoticesFor(ctx context.Context, token, query string, page int) (IncidentNoticePage, error) {
	out := IncidentNoticePage{Items: []IncidentNotice{}, Page: page, PageSize: 12}
	if e := incidentReadInput(query, "", page); e != nil {
		return out, e
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := incidentNoticeScope(p)
	if query != "" {
		scope += " AND instr(lower(n.title||' '||b.code||'-'||f.flat_number),lower(?))>0"
		args = append(args, query)
	}
	from := ` FROM incident_notices n JOIN incidents c ON c.id=n.incident_id JOIN flats f ON f.id=n.flat_id JOIN buildings b ON b.id=f.building_id WHERE `
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from+scope, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, noticeSelect+" WHERE "+scope+" ORDER BY n.created_at DESC,n.rowid DESC LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, e := scanIncidentNotice(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		x.IncidentID = ""
		out.Items = append(out.Items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) IncidentNoticeFor(ctx context.Context, token, id string, page int) (IncidentNotice, error) {
	out := IncidentNotice{Responses: []IncidentResponse{}, PageSize: 20}
	if page < 1 || page > 100000 {
		return out, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	scope, args := "1=1", []any{}
	if !p.CanHandleComplaints {
		scope, args = incidentNoticeScope(p)
	}
	x, e := scanIncidentNotice(tx.QueryRowContext(ctx, noticeSelect+" WHERE n.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return out, e
	}
	out.ID = x.ID
	out.IncidentID = x.IncidentID
	out.FlatID = x.FlatID
	out.Home = x.Home
	out.RuleID = x.RuleID
	out.IncidentDate = x.IncidentDate
	out.Title = x.Title
	out.Body = x.Body
	out.ResponseBy = x.ResponseBy
	out.PictureID = x.PictureID
	out.CreatedAt = x.CreatedAt
	out.Version = x.Version
	out.Active = x.Active
	out.CanRespond, e = incidentHomeMember(ctx, tx, p, out.FlatID)
	if e != nil {
		return out, e
	}
	out.CanRespond = out.CanRespond && out.Active
	out.Rule, e = scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", out.RuleID))
	if e != nil {
		return out, e
	}
	out.Rule.AuthorID = ""
	if !p.CanHandleComplaints {
		out.IncidentID = ""
	}
	where := "s.notice_id=?"
	args = []any{id}
	if !p.CanHandleComplaints {
		where += " AND s.actor_id=?"
		args = append(args, p.ID)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM incident_responses s WHERE "+where, args...).Scan(&out.ResponseTotal); e != nil {
		return out, e
	}
	out.ResponsePage = incidentPageBound(page, out.ResponseTotal, 20)
	rows, e := tx.QueryContext(ctx, `SELECT s.id,u.display_name,s.body,s.created_at FROM incident_responses s JOIN users u ON u.id=s.actor_id WHERE `+where+" ORDER BY s.rowid DESC LIMIT 20 OFFSET ?", append(args, (out.ResponsePage-1)*20)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var response IncidentResponse
		if e = rows.Scan(&response.ID, &response.Actor, &response.Body, &response.CreatedAt); e != nil {
			rows.Close()
			return out, e
		}
		if !p.CanHandleComplaints {
			response.Actor = ""
		}
		out.Responses = append(out.Responses, response)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}

// Keep domain writes behind current authority checks; a static identifier is never a capability.
func (s *Store) RespondToIncident(ctx context.Context, token, id string, in IncidentResponseInput) (string, error) {
	if !in.Confirmed || in.Version < 1 || !paragraph(in.Body, 10, 4000) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	scope, args := incidentNoticeScope(p)
	n, e := scanIncidentNotice(tx.QueryRowContext(ctx, noticeSelect+" WHERE n.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "INCIDENT_RESPONSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if in.Version != n.Version {
		return "", ErrConflict
	}
	response := randomToken()
	_, e = tx.ExecContext(ctx, `INSERT INTO incident_responses VALUES(?,?,?,?,?)`, response, id, p.ID, in.Body, time.Now().Unix())
	if e != nil {
		return "", e
	}
	if e = appendAudit(ctx, tx, p.ID, n.FlatID, "INCIDENT_RESPONSE", "Response submitted to frozen notice.", nil, map[string]any{"notice_id": id, "response_id": response}); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, response); e != nil {
		return "", e
	}
	return response, tx.Commit()
}
