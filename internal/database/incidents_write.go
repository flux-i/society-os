package database

import (
	"context"
	"database/sql"
	"time"
)

func incidentOwnedPicture(ctx context.Context, q identityReader, p Principal, id string) error {
	if id == "" {
		return nil
	}
	if len(id) > 100 {
		return ErrInvalid
	}
	var yes bool
	e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM incident_pictures WHERE id=? AND uploader_id=?)`, id, p.ID).Scan(&yes)
	if e != nil {
		return e
	}
	if !yes {
		return sql.ErrNoRows
	}
	return nil
}
func (s *Store) SaveIncident(ctx context.Context, token, id string, in IncidentInput) (string, error) {
	if !in.Confirmed || !paragraph(in.Comment, 10, 4000) || !validMaintenanceDate(in.IncidentDate) || in.IncidentDate > today() || len(in.FlatID) < 1 || len(in.FlatID) > 100 || len(in.RuleID) < 1 || len(in.RuleID) > 100 || len(in.PictureID) > 100 || (id == "" && in.Version != 0) || (id != "" && in.Version < 1) {
		return "", invalid("Choose a home and applicable rule, give the incident date and details, then confirm the reviewed report.")
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	yes, e := incidentParticipant(ctx, tx, p)
	if e != nil {
		return "", e
	}
	if !yes {
		return "", ErrForbidden
	}
	var x Incident
	if id != "" {
		x, e = scanIncident(tx.QueryRowContext(ctx, incidentSelect+" WHERE c.id=? AND c.reporter_id=?", id, p.ID))
		if e != nil {
			return "", e
		}
	}
	if e = incidentOwnedPicture(ctx, tx, p, in.PictureID); e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "INCIDENT_SAVE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	rule, e := scanRule(tx.QueryRowContext(ctx, ruleSelect+" WHERE id=?", in.RuleID))
	if e != nil {
		return "", e
	}
	if rule.State != "PUBLISHED" || in.IncidentDate < rule.EffectiveFrom || (rule.EffectiveUntil != "" && in.IncidentDate > rule.EffectiveUntil) {
		return "", invalid("Choose a currently published rule whose effective dates include this incident.")
	}
	var exists bool
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)", in.FlatID).Scan(&exists); e != nil {
		return "", e
	}
	if !exists {
		return "", sql.ErrNoRows
	}
	if in.PictureID != "" {
		var used bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM incident_events WHERE json_extract(snapshot_json,'$.picture_id')=? AND incident_id<>?)`, in.PictureID, id).Scan(&used)
		if e != nil {
			return "", e
		}
		if used {
			return "", invalid("This picture belongs to another retained report. Upload a separate picture for this report.")
		}
	}
	now := time.Now().Unix()
	if id == "" {
		id = randomToken()
		x = Incident{ID: id, ReporterID: p.ID, CreatedAt: now, Version: 1, ReporterVersion: 1, State: "REPORTED"}
	} else {
		version := x.ReporterVersion
		if p.CanHandleComplaints {
			version = x.Version
		}
		if in.Version != version {
			return "", ErrConflict
		}
		if x.State != "REPORTED" && x.State != "NEEDS_INFO" {
			return "", ErrConflict
		}
		if x.NoticeID != "" && (in.RuleID != x.RuleID || in.FlatID != x.FlatID || in.IncidentDate != x.IncidentDate || in.PictureID != x.PictureID) {
			return "", invalid("A response notice is active. Ask the handler to remove and reissue it before correcting the tagged facts or picture.")
		}
		x.Version++
		x.ReporterVersion++
		x.State = "REPORTED"
	}
	x.RuleID = in.RuleID
	x.RuleTitle = rule.Title
	x.FlatID = in.FlatID
	x.IncidentDate = in.IncidentDate
	x.Comment = in.Comment
	x.PictureID = in.PictureID
	x.UpdatedAt = now
	x.ReporterUpdatedAt = now
	var home string
	if e = tx.QueryRowContext(ctx, `SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?`, x.FlatID).Scan(&home); e != nil {
		return "", e
	}
	x.Home = home
	action := "REVISED"
	if in.Version == 0 {
		action = "REPORTED"
		_, e = tx.ExecContext(ctx, `INSERT INTO incidents(id,rule_id,flat_id,reporter_id,incident_date,comment,picture_id,state,created_at,updated_at,version,reporter_version,reporter_updated_at) VALUES(?,?,?,?,?,?,?,'REPORTED',?,?,1,1,?)`, x.ID, x.RuleID, x.FlatID, p.ID, x.IncidentDate, x.Comment, optionalID(x.PictureID), now, now, now)
	} else {
		e = saveIncidentState(ctx, tx, x)
	}
	if e != nil {
		return "", e
	}
	if e = incidentEvent(ctx, tx, p, x, action, "Reporter submitted the reviewed incident details.", true); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func saveIncidentState(ctx context.Context, tx *sql.Tx, x Incident) error {
	_, e := tx.ExecContext(ctx, `UPDATE incidents SET rule_id=?,flat_id=?,incident_date=?,comment=?,picture_id=?,state=?,duplicate_of=?,notice_id=?,updated_at=?,version=?,reporter_version=?,reporter_updated_at=? WHERE id=?`, x.RuleID, x.FlatID, x.IncidentDate, x.Comment, optionalID(x.PictureID), x.State, optionalID(x.DuplicateOf), optionalID(x.NoticeID), x.UpdatedAt, x.Version, x.ReporterVersion, x.ReporterUpdatedAt, x.ID)
	return e
}
func (s *Store) ActOnIncident(ctx context.Context, token, id string, in IncidentAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 2000) || len(in.DuplicateOf) > 100 {
		return "", ErrInvalid
	}
	switch in.Action {
	case "NOTE", "NEEDS_INFO", "UNDER_REVIEW", "DISMISSED", "SUBSTANTIATED", "WITHDRAWN", "REOPEN", "ISSUE_NOTICE", "REMOVE_NOTICE", "DUPLICATE":
	default:
		return "", ErrInvalid
	}
	if in.Action != "DUPLICATE" && in.DuplicateOf != "" {
		return "", ErrInvalid
	}
	if in.Action != "ISSUE_NOTICE" && (in.Title != "" || in.Body != "" || in.ResponseBy != "") {
		return "", ErrInvalid
	}
	if in.Action == "ISSUE_NOTICE" && (!validText(in.Title, 5, 120) || !paragraph(in.Body, 10, 4000) || !validMaintenanceDate(in.ResponseBy) || in.ResponseBy < today()) {
		return "", invalid("Review the notice title, resident wording and an explicit current or future response date.")
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if in.Action != "WITHDRAWN" {
		if e = incidentOperator(p); e != nil {
			return "", e
		}
	}
	scope, args := incidentScope(p)
	x, e := scanIncident(tx.QueryRowContext(ctx, incidentSelect+" WHERE c.id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return "", e
	}
	if in.Action == "WITHDRAWN" {
		yes, e := incidentParticipant(ctx, tx, p)
		if e != nil {
			return "", e
		}
		if !yes || x.ReporterID != p.ID {
			return "", ErrForbidden
		}
	} else {
		if e = incidentOperator(p); e != nil {
			return "", e
		}
		if in.Action != "NOTE" && x.ReporterID == p.ID {
			return "", ErrForbidden
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "INCIDENT_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	version := x.Version
	if !p.CanHandleComplaints {
		version = x.ReporterVersion
	}
	if in.Version != version {
		return "", ErrConflict
	}
	unresolved := x.State == "REPORTED" || x.State == "NEEDS_INFO" || x.State == "UNDER_REVIEW"
	visible := true
	switch in.Action {
	case "NOTE":
		visible = false
	case "REOPEN":
		if x.State != "DISMISSED" && x.State != "SUBSTANTIATED" && x.State != "WITHDRAWN" {
			return "", ErrConflict
		}
		x.State = "UNDER_REVIEW"
		x.DuplicateOf = ""
	case "ISSUE_NOTICE":
		if !unresolved && x.State != "SUBSTANTIATED" {
			return "", ErrConflict
		}
		if x.NoticeID != "" {
			return "", invalid("Remove the current response notice before issuing a replacement.")
		}
		var next int
		if e = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0)+1 FROM incident_notices WHERE incident_id=?", id).Scan(&next); e != nil {
			return "", e
		}
		notice := randomToken()
		_, e = tx.ExecContext(ctx, `INSERT INTO incident_notices VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, notice, id, x.FlatID, x.RuleID, x.IncidentDate, in.Title, in.Body, in.ResponseBy, optionalID(x.PictureID), p.ID, time.Now().Unix(), next)
		if e != nil {
			return "", e
		}
		x.NoticeID = notice
		visible = false
	case "REMOVE_NOTICE":
		if x.NoticeID == "" {
			return "", ErrConflict
		}
		x.NoticeID = ""
		visible = false
	case "DUPLICATE":
		if !unresolved || in.DuplicateOf == "" || in.DuplicateOf == id {
			return "", ErrInvalid
		}
		var target bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM incidents WHERE id=? AND flat_id=? AND rule_id=? AND duplicate_of IS NULL AND state<>'WITHDRAWN')`, in.DuplicateOf, x.FlatID, x.RuleID).Scan(&target)
		if e != nil {
			return "", e
		}
		if !target {
			return "", invalid("Choose a retained original case for this same home and rule.")
		}
		x.DuplicateOf = in.DuplicateOf
		x.State = "DISMISSED"
		x.NoticeID = ""
	default:
		if !unresolved {
			return "", ErrConflict
		}
		x.State = in.Action
		if in.Action == "WITHDRAWN" || in.Action == "DISMISSED" {
			x.NoticeID = ""
		}
	}
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	if visible {
		x.ReporterVersion++
		x.ReporterUpdatedAt = x.UpdatedAt
	}
	if e = saveIncidentState(ctx, tx, x); e != nil {
		return "", e
	}
	if e = incidentEvent(ctx, tx, p, x, in.Action, in.Reason, visible); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
