package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func meetingEvent(ctx context.Context, tx *sql.Tx, p Principal, id string, version, proposal int, action, reason string, x MeetingSnapshot) error {
	at := time.Now().Unix()
	fingerprint := ""
	if action == "APPROVED" {
		data, e := json.Marshal(struct {
			ID       string
			Snapshot MeetingSnapshot
			Nonce    string
			At       int64
		}{id, meetingPublic(x), randomToken(), at})
		if e != nil {
			return e
		}
		fingerprint = TokenHash(string(data))
	}
	if _, e := tx.ExecContext(ctx, "INSERT INTO meeting_events VALUES(?,?,?,?,?,?,?,?)", id, version, proposal, action, p.ID, reason, at, fingerprint); e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "MEETING_"+action, "Meeting resource decision recorded", map[string]any{}, map[string]any{"id": id, "version": version, "proposal_version": proposal})
}
func (s *Store) ProposeMeeting(ctx context.Context, token, id string, input MeetingInput) (string, error) {
	in, e := normaliseMeeting(input)
	if e != nil {
		return "", e
	}
	if len(id) > 100 {
		return "", ErrInvalid
	}
	tx, p, e := s.beginCommunityWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MEETING_PROPOSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	version := 1
	action := "PROPOSED"
	h := meetingHead{}
	var x MeetingSnapshot
	if id == "" {
		if in.Version != 0 || in.Action != "AGENDA" {
			return "", ErrInvalid
		}
		id = randomToken()
	} else {
		h, e = meetingHeadIn(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if h.Version != in.Version {
			return "", ErrConflict
		}
		version = h.Version + 1
		if h.Pending > 0 {
			pending, err := meetingSnapshotIn(ctx, tx, id, h.Pending)
			if err != nil {
				return "", err
			}
			if pending.SubmittedBy != p.ID {
				return "", ErrForbidden
			}
			action = "REVISED"
		}
		if h.Published > 0 {
			x, e = meetingSnapshotIn(ctx, tx, id, h.Published)
			if e != nil {
				return "", e
			}
			if x.Action == "WITHDRAW" {
				return "", invalid("A withdrawn meeting is retained. Prepare a new meeting for a new event.")
			}
		}
	}
	if in.Action == "AGENDA" {
		if h.Published > 0 && x.Action != "AGENDA" {
			return "", invalid("Published minutes and cancellations retain their original meeting. Prepare a new meeting for a new event.")
		}
		homes, err := communityHomes(ctx, tx, CommunityInput{Scope: in.Scope, BuildingCode: in.BuildingCode, AreaKey: in.AreaKey, HomeIDs: in.HomeIDs})
		if err != nil {
			return "", err
		}
		x = MeetingSnapshot{Action: "AGENDA", Title: in.Title, Body: in.Body, Location: in.Location, Scope: in.Scope, BuildingCode: in.BuildingCode, Homes: homes, StartAt: in.StartAt, EndAt: in.EndAt, AckRequired: in.AckRequired, AckDeadline: in.AckDeadline}
	} else {
		if h.Published == 0 {
			return "", ErrConflict
		}
		if in.Action == "MINUTES" {
			if (x.Action != "AGENDA" && x.Action != "MINUTES") || in.HeldAt < x.StartAt || in.HeldAt > time.Now().Unix() {
				return "", invalid("Minutes need an approved agenda and a held time at or after its start and no later than now.")
			}
			x.HeldAt = in.HeldAt
			x.Minutes = in.Minutes
			x.UpdateText = ""
			x.AckRequired = in.AckRequired
			x.AckDeadline = in.AckDeadline
		} else {
			if in.Action == "CANCEL" {
				if x.Action != "AGENDA" {
					return "", invalid("Cancellation applies to an approved agenda.")
				}
				x.UpdateText = in.UpdateText
				x.Minutes = ""
				x.HeldAt = 0
			}
			x.AckRequired = false
			x.AckDeadline = 0
		}
		x.Action = in.Action
	}
	x.Version = version
	x.SubmittedBy = p.ID
	x.SubmittedAt = time.Now().Unix()
	x.Reason = in.Reason
	if h.ID == "" {
		if _, e = tx.ExecContext(ctx, "INSERT INTO meeting_resources VALUES(?,1,1,1,NULL,'PENDING',?,?)", id, p.ID, x.SubmittedAt); e != nil {
			return "", e
		}
	}
	homes, e := json.Marshal(x.Homes)
	if e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO meeting_versions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, version, x.Action, x.Title, x.Body, x.Location, x.Scope, x.BuildingCode, string(homes), x.StartAt, x.EndAt, x.HeldAt, x.Minutes, x.UpdateText, x.AckRequired, x.AckDeadline, x.SubmittedBy, x.SubmittedAt, x.Reason); e != nil {
		return "", e
	}
	if e = meetingEvent(ctx, tx, p, id, version, version, action, in.Reason, x); e != nil {
		return "", e
	}
	if h.ID != "" {
		if _, e = tx.ExecContext(ctx, "UPDATE meeting_resources SET version=?,latest_version=?,pending_version=?,decision='PENDING' WHERE id=? AND version=?", version, version, version, id, in.Version); e != nil {
			return "", e
		}
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecideMeeting(ctx context.Context, token, id string, in MeetingAction) (string, error) {
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 1 || in.Version > 1000000 || len(id) > 100 || (in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "CANCELLED") {
		return "", invalid("Confirm the exact proposal with a decision and a reason of 10–800 characters.")
	}
	tx, p, e := s.beginCommunityWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MEETING_DECIDE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h, e := meetingHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if h.Version != in.Version || h.Pending == 0 {
		return "", ErrConflict
	}
	x, e := meetingSnapshotIn(ctx, tx, id, h.Pending)
	if e != nil {
		return "", e
	}
	if (in.Action == "CANCELLED" && p.ID != x.SubmittedBy) || (in.Action != "CANCELLED" && p.ID == x.SubmittedBy) {
		return "", ErrForbidden
	}
	version := h.Version + 1
	if e = meetingEvent(ctx, tx, p, id, version, h.Pending, in.Action, in.Reason, x); e != nil {
		return "", e
	}
	published := optionalCommunityVersion(h.Published)
	if in.Action == "APPROVED" {
		published = h.Pending
	}
	if _, e = tx.ExecContext(ctx, "UPDATE meeting_resources SET version=?,pending_version=NULL,published_version=?,decision=? WHERE id=? AND version=?", version, published, in.Action, id, in.Version); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) AcknowledgeMeeting(ctx context.Context, token, id string, in MeetingAcknowledgementInput) (string, error) {
	if !in.Confirmed || in.Version < 1 || in.Version > 1000000 || len(id) < 1 || len(id) > 100 || len(in.Fingerprint) != 64 {
		return "", invalid("Confirm the exact published meeting version.")
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	h, e := meetingHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if h.Published == 0 {
		return "", sql.ErrNoRows
	}
	x, e := meetingSnapshotIn(ctx, tx, id, h.Published)
	if e != nil {
		return "", e
	}
	if x.Action == "WITHDRAW" {
		return "", sql.ErrNoRows
	}
	personal, e := meetingPersonalHomes(ctx, tx, p, x)
	if e != nil {
		return "", e
	}
	if len(personal) == 0 {
		return "", ErrForbidden
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MEETING_ACKNOWLEDGE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	var fingerprint string
	var eventVersion int
	if e = tx.QueryRowContext(ctx, "SELECT version,publication_fingerprint FROM meeting_events WHERE resource_id=? AND proposal_version=? AND action='APPROVED'", id, h.Published).Scan(&eventVersion, &fingerprint); e != nil {
		return "", e
	}
	if in.Version != x.Version || in.Fingerprint != fingerprint {
		return "", ErrConflict
	}
	if !x.AckRequired || (x.Action != "AGENDA" && x.Action != "MINUTES") {
		return "", ErrForbidden
	}
	e = tx.QueryRowContext(ctx, "SELECT id FROM meeting_acknowledgements WHERE resource_id=? AND publication_version=? AND resident_id=?", id, x.Version, p.ResidentID).Scan(&result)
	if !errors.Is(e, sql.ErrNoRows) && e != nil {
		return "", e
	}
	if errors.Is(e, sql.ErrNoRows) {
		result = randomToken()
		homes, err := json.Marshal(personal)
		if err != nil {
			return "", err
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO meeting_acknowledgements VALUES(?,?,?,?,?,?,?,?,?)", result, id, x.Version, fingerprint, eventVersion, p.ResidentID, p.ID, string(homes), time.Now().Unix()); e != nil {
			return "", e
		}
		if e = appendAudit(ctx, tx, p.ID, "", "MEETING_ACKNOWLEDGED", "Personal acknowledgement recorded", map[string]any{}, map[string]any{"meeting_id": id, "publication_version": x.Version}); e != nil {
			return "", e
		}
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, result); e != nil {
		return "", e
	}
	return result, tx.Commit()
}
