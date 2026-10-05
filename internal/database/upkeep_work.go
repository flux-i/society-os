package database

import (
	"context"
	"encoding/json"
	"time"
)

func validUpkeepCategory(category string) bool {
	switch category {
	case "PLUMBING", "LIFT", "ELECTRICAL", "SECURITY", "CLEANING", "WATER", "PARKING", "COMMON_AREA", "OTHER":
		return true
	}
	return false
}
func (s *Store) CreateUpkeepTask(ctx context.Context, token string, in UpkeepTaskInput) (string, error) {
	if !in.Confirmed || !validText(in.Title, 5, 120) || !paragraph(in.Body, 10, 4000) || !validUpkeepCategory(in.Category) || !complaintPriority(in.Priority) || !validMaintenanceDate(in.DueDate) || !optionalUpkeepDate(in.VisitDate) || in.RepeatDays < 0 || in.RepeatDays > 365 || len(in.ComplaintID) > 100 || len(in.ParentID) > 100 {
		return "", invalid("Review the title, details, category, priority and explicit calendar dates before confirming work.")
	}
	tx, p, err := s.beginUpkeepWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "UPKEEP_CREATE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if err = activeUpkeepReference(ctx, tx, in.AssetID, "ASSET"); err != nil {
		return "", err
	}
	if err = activeUpkeepReference(ctx, tx, in.VendorID, "VENDOR"); err != nil {
		return "", err
	}
	if err = currentUpkeepAssignee(ctx, tx, in.AssignedTo); err != nil {
		return "", err
	}
	if in.ComplaintID != "" {
		var exists bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM complaints WHERE id=?)", in.ComplaintID).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return "", invalid("The linked service case is unavailable. Check its identity before confirming.")
		}
	}
	action := "CREATED"
	if in.ParentID != "" {
		var state, due string
		var interval int
		if err = tx.QueryRowContext(ctx, "SELECT state,due_date,repeat_days FROM upkeep_tasks WHERE id=?", in.ParentID).Scan(&state, &due, &interval); err != nil {
			return "", err
		}
		if (state != "DONE" && state != "CANCELLED") || interval == 0 || in.DueDate <= due {
			return "", invalid("Repeat a completed or cancelled recurring item with a later, explicitly reviewed date.")
		}
		var duplicate bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM upkeep_tasks WHERE parent_id=? AND due_date=?)", in.ParentID, in.DueDate).Scan(&duplicate); err != nil {
			return "", err
		}
		if duplicate {
			return "", ErrConflict
		}
		action = "REPEATED"
	}
	id, now := randomToken(), time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO upkeep_tasks(id,title,body,category,priority,due_date,visit_date,asset_id,vendor_id,assigned_to,complaint_id,repeat_days,parent_id,state,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'PLANNED',?,?,?)`, id, in.Title, in.Body, in.Category, in.Priority, in.DueDate, in.VisitDate, optionalID(in.AssetID), optionalID(in.VendorID), optionalID(in.AssignedTo), optionalID(in.ComplaintID), in.RepeatDays, optionalID(in.ParentID), p.ID, now, now)
	if err != nil {
		return "", err
	}
	saved, err := scanUpkeepTask(tx.QueryRowContext(ctx, upkeepTaskSelect+" WHERE t.id=?", id), true)
	if err != nil {
		return "", err
	}
	if err = upkeepEvent(ctx, tx, p, "upkeep_task_events", "task_id", id, action, "Reviewed and confirmed the supplied work record", 1, saved); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func validateUpkeepAction(id string, in UpkeepAction) error {
	if len(id) < 1 || len(id) > 100 || in.Version < 1 || !in.Confirmed || !paragraph(in.Reason, 10, 2000) {
		return invalid("Supply a current version, a reason of 10–2000 characters and confirmation.")
	}
	switch in.Action {
	case "START", "WAIT", "SUBMIT_CHECK", "CONFIRM_DONE", "RETURN_WORK", "CANCEL", "REOPEN", "ASSIGN", "RESCHEDULE", "COMMENT", "PUBLISH", "UNPUBLISH":
	default:
		return invalid("Choose a supported work action.")
	}
	if in.Action != "ASSIGN" && in.AssignedTo != "" {
		return ErrInvalid
	}
	if in.Action != "RESCHEDULE" && (in.DueDate != "" || in.VisitDate != "") {
		return ErrInvalid
	}
	if in.Action != "PUBLISH" && (in.Audience != "" || in.BuildingCode != "" || in.PublicTitle != "" || in.PublicBody != "") {
		return ErrInvalid
	}
	return nil
}
func (s *Store) UpdateUpkeepTask(ctx context.Context, token, id string, in UpkeepAction) (string, error) {
	if err := validateUpkeepAction(id, in); err != nil {
		return "", err
	}
	tx, p, err := s.beginUpkeepWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "UPKEEP_ACTION:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	old, err := scanUpkeepTask(tx.QueryRowContext(ctx, upkeepTaskSelect+" WHERE t.id=?", id), true)
	if err != nil {
		return "", err
	}
	if old.Version != in.Version {
		return "", ErrConflict
	}
	state, assigned, due, visit := old.State, old.AssignedTo, old.DueDate, old.VisitDate
	readyBy, readyAt, checkedBy, checkedAt := old.ReadyBy, old.ReadyAt, old.CheckedBy, old.CheckedAt
	audience, building, publicVersion, publishedAt := old.Audience, old.BuildingCode, old.PublicVersion, old.PublishedAt
	var publishedBy string
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(published_by,'') FROM upkeep_tasks WHERE id=?", id).Scan(&publishedBy); err != nil {
		return "", err
	}
	snapshot := ""
	if old.PublicSnapshot != nil {
		data, e := json.Marshal(old.PublicSnapshot)
		if e != nil {
			return "", e
		}
		snapshot = string(data)
	}
	terminal := state == "DONE" || state == "CANCELLED"
	now := time.Now().Unix()
	switch in.Action {
	case "START":
		if state != "PLANNED" && state != "WAITING" {
			return "", ErrConflict
		}
		state = "IN_PROGRESS"
	case "WAIT":
		if state != "IN_PROGRESS" {
			return "", ErrConflict
		}
		state = "WAITING"
	case "SUBMIT_CHECK":
		if state != "IN_PROGRESS" {
			return "", ErrConflict
		}
		state, readyBy, readyAt = "READY_FOR_CHECK", p.ID, now
	case "CONFIRM_DONE":
		if state != "READY_FOR_CHECK" {
			return "", ErrConflict
		}
		if readyBy == p.ID {
			return "", ErrForbidden
		}
		state, checkedBy, checkedAt = "DONE", p.ID, now
	case "RETURN_WORK":
		if state != "READY_FOR_CHECK" {
			return "", ErrConflict
		}
		state, readyBy, readyAt = "IN_PROGRESS", "", 0
	case "CANCEL":
		if terminal {
			return "", ErrConflict
		}
		state = "CANCELLED"
	case "REOPEN":
		if !terminal {
			return "", ErrConflict
		}
		state, readyBy, readyAt, checkedBy, checkedAt = "PLANNED", "", 0, "", 0
	case "ASSIGN":
		if terminal {
			return "", ErrConflict
		}
		if err = currentUpkeepAssignee(ctx, tx, in.AssignedTo); err != nil {
			return "", err
		}
		assigned = in.AssignedTo
	case "RESCHEDULE":
		if terminal {
			return "", ErrConflict
		}
		if !validMaintenanceDate(in.DueDate) || !optionalUpkeepDate(in.VisitDate) {
			return "", invalid("Use explicit valid due and optional visit dates.")
		}
		due, visit = in.DueDate, in.VisitDate
	case "COMMENT": // Only the private immutable event changes.
	case "PUBLISH":
		if !validText(in.PublicTitle, 5, 120) || !paragraph(in.PublicBody, 10, 4000) || (in.Audience != "ALL_RESIDENTS" && in.Audience != "BUILDING") || len(in.BuildingCode) > 50 {
			return "", invalid("Review the public title, description and resident audience.")
		}
		if in.Audience == "BUILDING" {
			var exists bool
			if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM buildings WHERE code=?)", in.BuildingCode).Scan(&exists); err != nil {
				return "", err
			}
			if !exists {
				return "", ErrInvalid
			}
		} else if in.BuildingCode != "" {
			return "", ErrInvalid
		}
		data, e := json.Marshal(UpkeepPublication{in.PublicTitle, in.PublicBody, old.Category, old.Priority, due, visit, state})
		if e != nil {
			return "", e
		}
		snapshot, audience, building, publicVersion, publishedBy, publishedAt = string(data), in.Audience, in.BuildingCode, publicVersion+1, p.ID, now
	case "UNPUBLISH":
		if audience == "INTERNAL" {
			return "", ErrConflict
		}
		audience, building, publicVersion = "INTERNAL", "", publicVersion+1
	}
	_, err = tx.ExecContext(ctx, `UPDATE upkeep_tasks SET state=?,assigned_to=?,due_date=?,visit_date=?,ready_by=?,ready_at=?,checked_by=?,checked_at=?,audience=?,building_code=?,public_version=?,public_snapshot_json=?,published_by=?,published_at=?,version=version+1,updated_at=? WHERE id=? AND version=?`, state, optionalID(assigned), due, visit, optionalID(readyBy), nullableComplaintTime(readyAt), optionalID(checkedBy), nullableComplaintTime(checkedAt), audience, building, publicVersion, snapshot, optionalID(publishedBy), nullableComplaintTime(publishedAt), now, id, in.Version)
	if err != nil {
		return "", err
	}
	saved, err := scanUpkeepTask(tx.QueryRowContext(ctx, upkeepTaskSelect+" WHERE t.id=?", id), true)
	if err != nil {
		return "", err
	}
	if err = upkeepEvent(ctx, tx, p, "upkeep_task_events", "task_id", id, in.Action, in.Reason, old.Version+1, map[string]any{"before": old, "after": saved}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
