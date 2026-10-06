package database

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Financial review receives supplied outcome/rule and deliberately issued response
// context. It never receives the original allegation, picture or reporter identity.
func fineSourceIn(ctx context.Context, tx *sql.Tx, id string, page int) (FineSource, error) {
	x := FineSource{IncidentID: id, ResponsePage: page, PageSize: 20}
	var event int64
	var notice string
	e := tx.QueryRowContext(ctx, `SELECT i.flat_id,b.code||'-'||h.flat_number,i.rule_id,i.incident_date,i.reporter_id,COALESCE(i.notice_id,''),
 (SELECT MAX(id) FROM incident_events WHERE incident_id=i.id AND action='SUBSTANTIATED'),
 (SELECT occurred_at FROM incident_events WHERE incident_id=i.id AND action='SUBSTANTIATED' ORDER BY id DESC LIMIT 1)
 FROM incidents i JOIN flats h ON h.id=i.flat_id JOIN buildings b ON b.id=h.building_id WHERE i.id=? AND i.state='SUBSTANTIATED' AND i.duplicate_of IS NULL`, id).Scan(&x.FlatID, &x.Home, &x.Rule.ID, &x.IncidentDate, &x.reporterID, &notice, &event, &x.SubstantiatedAt)
	if e != nil {
		return x, e
	}
	x.OutcomeEventID = event
	x.Rule, e = scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", x.Rule.ID))
	if e != nil {
		return x, e
	}
	if !x.Rule.FinePermitted || x.IncidentDate < x.Rule.EffectiveFrom || (x.Rule.EffectiveUntil != "" && x.IncidentDate > x.Rule.EffectiveUntil) {
		return x, ErrInvalid
	}
	x.Rule.AuthorID = ""
	x.Rule.ReplacesID = ""
	x.Rule.Events = nil
	x.Rule.EventPage = 0
	x.Rule.EventTotal = 0
	x.Rule.PageSize = 0
	x.Rule.CreatedAt = 0
	x.Rule.UpdatedAt = 0
	data, _ := json.Marshal([]any{x.IncidentID, event, x.FlatID, x.Rule.ID, x.IncidentDate})
	x.MaterialKey = TokenHash(string(data))
	ids := []string{}
	if notice != "" {
		n := &FineSourceNotice{ID: notice, Responses: []IncidentResponse{}}
		e = tx.QueryRowContext(ctx, `SELECT title,body,response_by FROM incident_notices WHERE id=?`, notice).Scan(&n.Title, &n.Body, &n.ResponseBy)
		if e != nil {
			return x, e
		}
		rows, e := tx.QueryContext(ctx, "SELECT id FROM incident_responses WHERE notice_id=? ORDER BY id", notice)
		if e != nil {
			return x, e
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return x, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
		x.ResponseTotal = len(ids)
		x.ResponsePage = incidentPageBound(page, x.ResponseTotal, 20)
		rows, e = tx.QueryContext(ctx, `SELECT r.id,u.display_name,r.body,r.created_at FROM incident_responses r JOIN users u ON u.id=r.actor_id WHERE r.notice_id=? ORDER BY r.created_at,r.id LIMIT 20 OFFSET ?`, notice, (x.ResponsePage-1)*20)
		if e != nil {
			return x, e
		}
		for rows.Next() {
			var r IncidentResponse
			if e = rows.Scan(&r.ID, &r.Actor, &r.Body, &r.CreatedAt); e != nil {
				rows.Close()
				return x, e
			}
			n.Responses = append(n.Responses, r)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return x, e
		}
		x.Notice = n
	}
	data, _ = json.Marshal([]any{x.MaterialKey, notice, ids})
	x.SourceKey = TokenHash(string(data))
	return x, nil
}
func (s *Store) FineSourceFor(ctx context.Context, token, id string, page int) (FineSource, error) {
	if id == "" || len(id) > 100 || !boundedMaintenancePage(page) {
		return FineSource{}, ErrInvalid
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return FineSource{}, e
	}
	defer tx.Rollback()
	if !p.CanReadAllRecords {
		return FineSource{}, ErrForbidden
	}
	out, e := fineSourceIn(ctx, tx, id, page)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func (s *Store) FineSourcesFor(ctx context.Context, token, q string, page int) (FineSourcePage, error) {
	out := FineSourcePage{Items: []FineSource{}, Page: page, PageSize: 12}
	if e := incidentReadInput(q, "", page); e != nil {
		return out, e
	}
	tx, p, e := beginIncidentRead(ctx, s, token)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if !p.CanManageRecords {
		return out, ErrForbidden
	}
	where := `i.state='SUBSTANTIATED' AND i.duplicate_of IS NULL AND r.fine_permitted=1 AND NOT EXISTS(SELECT 1 FROM fines f WHERE f.incident_id=i.id AND f.state IN('PENDING','NOTIFIED','ISSUED','WAIVED')) AND (r.title LIKE ? ESCAPE '\' OR b.code||'-'||h.flat_number LIKE ? ESCAPE '\')`
	pattern := fineSearchPattern(q)
	args := []any{pattern, pattern}
	from := ` FROM incidents i JOIN society_rules r ON r.id=i.rule_id JOIN flats h ON h.id=i.flat_id JOIN buildings b ON b.id=h.building_id WHERE ` + where
	e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+from, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	out.Page = incidentPageBound(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, "SELECT i.id"+from+" ORDER BY i.updated_at DESC,i.id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
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
		x, e := fineSourceIn(ctx, tx, id, 1)
		if e != nil {
			return out, e
		}
		x.Notice = nil
		x.ResponseTotal = 0
		out.Items = append(out.Items, x)
	}
	return out, tx.Commit()
}
