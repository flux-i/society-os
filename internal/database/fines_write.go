package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func (s *Store) CreateFine(ctx context.Context, token string, in FineInput) (string, error) {
	amount, e := ParseAmount(in.Amount)
	if e != nil {
		return "", e
	}
	if !in.Confirmed || len(in.IncidentID) < 1 || len(in.IncidentID) > 100 || len(in.ReplacesID) > 100 || len(in.SourceKey) != 64 || !validText(in.Title, 5, 120) || !validText(in.PolicyReference, 5, 300) || !paragraph(in.Reason, 10, 2000) || !paragraph(in.NoticeBody, 10, 4000) || !validMaintenanceDate(in.DueDate) || !validMaintenanceDate(in.ResponseBy) || in.ResponseBy < today() || in.DueDate < in.ResponseBy {
		return "", invalid("Review a supplied amount, policy, household wording and current response/due dates.")
	}
	tx, p, e := s.beginFineWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_CREATE", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	source, e := fineSourceIn(ctx, tx, in.IncidentID, 1)
	if e != nil {
		return "", e
	}
	if source.SourceKey != in.SourceKey {
		return "", ErrConflict
	}
	if in.ReplacesID != "" {
		var previous, incident string
		e = tx.QueryRowContext(ctx, "SELECT state,incident_id FROM fines WHERE id=?", in.ReplacesID).Scan(&previous, &incident)
		if e != nil {
			return "", e
		}
		if incident != in.IncidentID || (previous != "DECLINED" && previous != "WITHDRAWN") {
			return "", ErrConflict
		}
	}
	var active bool
	e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM fines WHERE incident_id=? AND state IN('PENDING','NOTIFIED','ISSUED','WAIVED'))", in.IncidentID).Scan(&active)
	if e != nil {
		return "", e
	}
	if active {
		return "", invalid("This incident already has an active fine. Use its retained decision or correction workflow.")
	}
	blob, e := json.Marshal(source)
	if e != nil {
		return "", e
	}
	id, now := randomToken(), time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO fines(id,incident_id,replaces_id,flat_id,rule_id,material_key,source_key,source_json,title,amount_paise,policy_reference,reason,due_date,response_by,notice_body,state,author_id,created_at,updated_at,version) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'PENDING',?,?,?,1)`, id, in.IncidentID, optionalID(in.ReplacesID), source.FlatID, source.Rule.ID, source.MaterialKey, source.SourceKey, string(blob), in.Title, amount, in.PolicyReference, in.Reason, in.DueDate, in.ResponseBy, in.NoticeBody, p.ID, now, now)
	if e != nil {
		return "", e
	}
	if e = fineEvent(ctx, tx, p, id, "FINE", id, "PROPOSED", in.Reason, 1, in); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) ActOnFine(ctx context.Context, token, id string, in FineAction) (string, error) {
	if id == "" || len(id) > 100 || !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 2000) {
		return "", ErrInvalid
	}
	switch in.Action {
	case "NOTE", "NOTIFY", "RESOLVE", "ISSUE", "DECLINED", "WITHDRAWN":
	default:
		return "", ErrInvalid
	}
	if in.Action != "RESOLVE" && (in.SourceKey != "" || in.ResponseCount != 0 || in.Resolution != "" || in.EarlyIssueReference != "") {
		return "", ErrInvalid
	}
	if in.Action != "ISSUE" && in.ResolutionKey != "" {
		return "", ErrInvalid
	}
	tx, p, e := s.beginFineWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", id))
	if e != nil {
		return "", e
	}
	if in.Action == "WITHDRAWN" {
		if x.AuthorID != p.ID {
			return "", ErrForbidden
		}
	} else if in.Action != "NOTE" {
		if e = fineEligible(ctx, tx, p, x); e != nil {
			return "", e
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if in.Version != x.Version {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	public := false
	switch in.Action {
	case "NOTE":
		if x.State == "DECLINED" || x.State == "WITHDRAWN" {
			return "", ErrConflict
		}
	case "DECLINED", "WITHDRAWN":
		if x.State != "PENDING" && x.State != "NOTIFIED" {
			return "", ErrConflict
		}
		public = x.NoticeID != ""
		_, e = tx.ExecContext(ctx, "UPDATE fines SET state=?,version=version+1,public_version=public_version+?,public_updated_at=CASE WHEN ?=1 THEN ? ELSE public_updated_at END,updated_at=? WHERE id=?", in.Action, boolInt(public), boolInt(public), now, now, id)
	case "NOTIFY":
		if x.State != "PENDING" || x.ResponseBy < today() {
			return "", invalid("A notice requires a current pending proposal and a response date that has not passed.")
		}
		source, _, _, err := fineContext(ctx, tx, x)
		if err != nil {
			return "", err
		}
		if source.SourceKey != x.SourceKey {
			return "", ErrConflict
		}
		notice := randomToken()
		_, e = tx.ExecContext(ctx, `INSERT INTO fine_notices VALUES(?,?,?,?,?,?,?,?,?,?,?,1)`, notice, id, x.FlatID, x.Title, x.NoticeBody, x.AmountPaise, x.PolicyReference, x.DueDate, x.ResponseBy, p.ID, now)
		if e != nil {
			return "", e
		}
		_, e = tx.ExecContext(ctx, `UPDATE fines SET state='NOTIFIED',notice_id=?,version=version+1,public_version=public_version+1,public_updated_at=?,updated_at=? WHERE id=?`, notice, now, now, id)
	case "RESOLVE":
		if x.State != "NOTIFIED" || !paragraph(in.Resolution, 10, 4000) || !validText(in.EarlyIssueReference, 0, 300) {
			return "", ErrInvalid
		}
		source, key, count, err := fineContext(ctx, tx, x)
		if err != nil {
			return "", err
		}
		if source.SourceKey != in.SourceKey || count != in.ResponseCount {
			return "", ErrConflict
		}
		if today() <= x.ResponseBy && in.EarlyIssueReference == "" {
			return "", invalid("The response date has not passed. Supply the explicit policy/response decision permitting early issuance.")
		}
		_, e = tx.ExecContext(ctx, `UPDATE fines SET resolution=?,resolution_key=?,early_issue_reference=?,resolved_by=?,resolved_at=?,version=version+1,updated_at=? WHERE id=?`, in.Resolution, key, in.EarlyIssueReference, p.ID, now, now, id)
	case "ISSUE":
		if x.State != "NOTIFIED" || x.ResolutionKey == "" || in.ResolutionKey != x.ResolutionKey {
			return "", ErrConflict
		}
		source, key, count, err := fineContext(ctx, tx, x)
		if err != nil {
			return "", err
		}
		if key != x.ResolutionKey {
			return "", invalid("Responses or the source have changed. Review them and record a fresh resolution before issuance.")
		}
		if today() <= x.ResponseBy && x.EarlyIssueReference == "" {
			return "", ErrConflict
		}
		entry := randomToken()
		_, e = tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at) VALUES(?,?,'CHARGE',?,?,?,'','','',?,'POSTED',?,?,?,?)`, entry, x.FlatID, x.AmountPaise, today(), "Fine · "+x.Title, "Separately reviewed supplied fine; approved notice retained", x.AuthorID, x.CreatedAt, p.ID, now)
		if e != nil {
			return "", e
		}
		incidentNotice := ""
		if source.Notice != nil {
			incidentNotice = source.Notice.ID
		}
		_, e = tx.ExecContext(ctx, `UPDATE fines SET state='ISSUED',original_entry_id=?,current_entry_id=?,charge_version=1,issued_by=?,issued_at=?,issued_context_key=?,issued_incident_notice_id=?,issued_response_count=?,version=version+1,public_version=public_version+1,public_updated_at=?,updated_at=? WHERE id=?`, entry, entry, p.ID, now, key, incidentNotice, count, now, now, id)
		if e != nil {
			return "", e
		}
		if e = appendAudit(ctx, tx, p.ID, x.FlatID, "FINE_CHARGE_ISSUED", in.Reason, map[string]any{"fine_id": id}, map[string]any{"entry_id": entry, "amount_paise": x.AmountPaise}); e != nil {
			return "", e
		}
	}
	if e != nil {
		return "", e
	}
	if in.Action == "NOTE" {
		e = fineTouch(ctx, tx, p, x, in.Action, in.Reason, false, in)
	} else {
		e = fineEvent(ctx, tx, p, id, "FINE", id, in.Action, in.Reason, x.Version+1, in)
	}
	if e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func boolInt(x bool) int {
	if x {
		return 1
	}
	return 0
}
func (s *Store) RespondToFineNotice(ctx context.Context, token, id string, in IncidentResponseInput) (string, error) {
	if id == "" || len(id) > 100 || !in.Confirmed || in.Version != 1 || !paragraph(in.Body, 10, 4000) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var fine, home, state string
	e = tx.QueryRowContext(ctx, `SELECT n.fine_id,n.flat_id,f.state FROM fine_notices n JOIN fines f ON f.id=n.fine_id WHERE n.id=?`, id).Scan(&fine, &home, &state)
	if e != nil {
		return "", e
	}
	member, e := incidentHomeMember(ctx, tx, p, home)
	if e != nil {
		return "", e
	}
	if !member {
		return "", sql.ErrNoRows
	}
	if state == "DECLINED" || state == "WITHDRAWN" {
		return "", ErrConflict
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "FINE_RESPONSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	response := randomToken()
	_, e = tx.ExecContext(ctx, "INSERT INTO fine_responses VALUES(?,?,?,?,?)", response, id, p.ID, in.Body, time.Now().Unix())
	if e != nil {
		return "", e
	}
	x, e := scanFine(tx.QueryRowContext(ctx, fineSelect+"WHERE f.id=?", fine))
	if e != nil {
		return "", e
	}
	if e = fineTouch(ctx, tx, p, x, "RESPONSE", "Household supplied its private response", false, map[string]string{"response_id": response}); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, response); e != nil {
		return "", e
	}
	return response, tx.Commit()
}
