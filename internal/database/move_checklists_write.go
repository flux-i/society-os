package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func persistMoveChecklist(ctx context.Context, tx *sql.Tx, p Principal, h moveChecklistHead, x MoveChecklistSnapshot, action, reason string) (string, error) {
	version, at := h.Version+1, time.Now().Unix()
	x.Version = version
	x.Action = action
	x.ActorID = p.ID
	x.OccurredAt = at
	x.Reason = reason
	x.PreviousApprovedVersion = h.Approved
	if h.Version == 0 {
		if _, e := tx.ExecContext(ctx, `INSERT INTO move_checklist_resources VALUES(?,?,?,?,?,?,1,1,1,NULL,'CHECKING')`, h.ID, h.FlatID, h.ResidentID, h.Kind, h.AuthorID, at); e != nil {
			return "", e
		}
	}
	raw, e := json.Marshal(x)
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO move_checklist_versions VALUES(?,?,?,?,?,?,?)`, h.ID, version, action, string(raw), p.ID, at, reason); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO move_checklist_events VALUES(?,?,?,?,?,?)`, h.ID, version, action, p.ID, at, reason); e != nil {
		return "", e
	}
	if h.Version > 0 {
		var pending any
		if x.Phase == "CHECKING" || x.Phase == "READY" || x.Phase == "INFO" {
			pending = version
		}
		approved := optionalCommunityVersion(h.Approved)
		if action == "APPROVED" {
			approved = version
		}
		result, err := tx.ExecContext(ctx, `UPDATE move_checklist_resources SET version=?,latest_version=?,pending_version=?,approved_version=?,phase=? WHERE id=? AND version=?`, version, version, pending, approved, x.Phase, h.ID, h.Version)
		if err != nil {
			return "", err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if n != 1 {
			return "", ErrConflict
		}
	}
	if e = appendAudit(ctx, tx, p.ID, h.FlatID, "MOVE_CHECKLIST_"+action, "Checklist state changed", map[string]any{}, map[string]any{"id": h.ID, "version": version, "action": action}); e != nil {
		return "", e
	}
	return h.ID, nil
}
func (s *Store) SubmitMoveChecklist(ctx context.Context, token string, input MoveChecklistInput) (string, error) {
	in, e := normaliseMoveInput(input)
	if e != nil {
		return "", e
	}
	tx, p, e := s.beginMoveWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if p.CanManageRegistry {
		if e = requireMoveStaff(p); e != nil {
			return "", e
		}
		if in.ResidentID == "" {
			return "", invalid("Choose the supplied current person for this home.")
		}
	} else {
		if in.ResidentID != "" && in.ResidentID != p.ResidentID {
			return "", ErrForbidden
		}
		in.ResidentID = p.ResidentID
	}
	source, e := moveSourceIn(ctx, tx, in.FlatID, in.ResidentID)
	if e != nil {
		return "", e
	}
	if !source.Current {
		return "", ErrForbidden
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MOVE_CHECKLIST_SUBMIT", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h := moveChecklistHead{ID: randomToken(), FlatID: in.FlatID, ResidentID: in.ResidentID, Kind: in.Kind, AuthorID: p.ID}
	x := MoveChecklistSnapshot{Phase: "CHECKING", FlatID: in.FlatID, ResidentID: in.ResidentID, Kind: in.Kind, AuthorID: p.ID, ProposedBy: p.ID, EffectiveDate: in.EffectiveDate, Note: in.Note, PersonName: source.PersonName, HomeLabel: source.HomeLabel, Checks: emptyMoveChecks(), Source: source}
	id, e := persistMoveChecklist(ctx, tx, p, h, x, "SUBMIT", in.Reason)
	if e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) ActOnMoveChecklist(ctx context.Context, token, id string, input MoveChecklistAction) (string, error) {
	in, e := normaliseMoveAction(input)
	if e != nil {
		return "", e
	}
	if len(id) < 1 || len(id) > 100 {
		return "", ErrInvalid
	}
	tx, p, e := s.beginMoveWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	h, e := moveHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if !mayReadMoveChecklist(p, h) {
		return "", sql.ErrNoRows
	}
	x, e := moveSnapshotIn(ctx, tx, id, h.Latest)
	if e != nil {
		return "", e
	}
	source, e := moveSourceIn(ctx, tx, h.FlatID, h.ResidentID)
	if e != nil {
		return "", e
	}
	staff := in.Action != "REVISE" && in.Action != "CORRECTION" && in.Action != "CANCELLED"
	if staff || p.CanManageRegistry {
		if e = requireMoveStaff(p); e != nil {
			return "", e
		}
	}
	if in.Action == "CANCELLED" {
		if x.ProposedBy != p.ID {
			return "", ErrForbidden
		}
	} else if in.Action == "REVISE" {
		if x.ProposedBy != p.ID || (!p.CanManageRegistry && (!ownMoveChecklist(p, h) || !source.Current)) {
			return "", ErrForbidden
		}
	} else if in.Action == "CORRECTION" && !p.CanManageRegistry {
		if !ownMoveChecklist(p, h) || !source.Current {
			return "", ErrForbidden
		}
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MOVE_CHECKLIST_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if h.Version != in.Version {
		return "", ErrConflict
	}
	_, _, _, checksReady := moveChecksCurrent(x, source.Key)
	switch in.Action {
	case "REVISE":
		if h.Pending == 0 || (h.Phase != "CHECKING" && h.Phase != "INFO") {
			return "", ErrConflict
		}
		x.EffectiveDate = in.EffectiveDate
		x.Note = in.Note
		x.ProposedBy = p.ID
		x.Phase = "CHECKING"
		x.Checks = emptyMoveChecks()
		x.ReadyBy = ""
		x.SourceKey = ""
		x.Source = source
	case "CORRECTION":
		if h.Approved == 0 || h.Pending != 0 {
			return "", ErrConflict
		}
		accepted, err := moveSnapshotIn(ctx, tx, id, h.Approved)
		if err != nil {
			return "", err
		}
		x = accepted
		x.EffectiveDate = in.EffectiveDate
		x.Note = in.Note
		x.ProposedBy = p.ID
		x.Phase = "CHECKING"
		x.Checks = emptyMoveChecks()
		x.ReadyBy = ""
		x.SourceKey = ""
		x.Source = source
	case "CHECK":
		if h.Pending == 0 || (h.Phase != "CHECKING" && h.Phase != "INFO") {
			return "", ErrConflict
		}
		if in.SourceKey != source.Key {
			return "", ErrConflict
		}
		for i := range x.Checks {
			if x.Checks[i].Kind == in.CheckKind {
				x.Checks[i] = MoveChecklistCheck{Kind: in.CheckKind, State: in.CheckState, Reference: in.Reference, CheckedBy: p.ID, CheckedAt: time.Now().Unix(), SourceKey: source.Key}
			}
		}
		x.Phase = "CHECKING"
		x.ReadyBy = ""
		x.SourceKey = ""
		x.Source = source
	case "READY":
		if h.Pending == 0 || h.Phase != "CHECKING" || !checksReady || in.SourceKey != source.Key {
			return "", ErrConflict
		}
		x.Phase = "READY"
		x.ReadyBy = p.ID
		x.SourceKey = source.Key
		x.Source = source
	case "RETURN":
		if h.Pending == 0 || h.Phase != "READY" {
			return "", ErrConflict
		}
		x.Phase = "CHECKING"
		x.ReadyBy = ""
		x.SourceKey = ""
		x.Source = source
	case "APPROVED", "INFO", "DECLINED":
		if h.Pending == 0 {
			return "", ErrConflict
		}
		if x.AuthorID == p.ID || x.ProposedBy == p.ID || (x.ReadyBy != "" && x.ReadyBy == p.ID) {
			return "", ErrForbidden
		}
		if in.Action == "APPROVED" {
			if h.Phase != "READY" || !checksReady || x.SourceKey != source.Key || in.SourceKey != source.Key {
				return "", ErrConflict
			}
			x.Phase = "COMPLETED"
		} else if in.Action == "INFO" {
			if h.Phase != "READY" {
				return "", ErrConflict
			}
			x.Phase = "INFO"
			x.ReadyBy = ""
			x.SourceKey = ""
		} else {
			x.Phase = "DECLINED"
		}
	case "CANCELLED":
		if h.Pending == 0 {
			return "", ErrConflict
		}
		x.Phase = "CANCELLED"
	default:
		return "", ErrInvalid
	}
	result, e = persistMoveChecklist(ctx, tx, p, h, x, in.Action, in.Reason)
	if e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, result); e != nil {
		return "", e
	}
	return result, tx.Commit()
}

// Unused known fields are rejected rather than silently changing their meaning
// under a different action; retry hashes still bind the whole reviewed payload.
func exclusiveMoveFields(in MoveChecklistAction) bool {
	check := in.CheckKind != "" || in.CheckState != "" || in.Reference != ""
	proposal := in.EffectiveDate != "" || in.Note != ""
	switch in.Action {
	case "CHECK":
		return !proposal && len(in.SourceKey) == 64
	case "REVISE", "CORRECTION":
		return !check && in.SourceKey == ""
	case "READY", "APPROVED":
		return !check && !proposal
	default:
		return !check && !proposal && strings.TrimSpace(in.SourceKey) == ""
	}
}
