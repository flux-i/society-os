package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func (s *Store) beginCommunityWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return nil, p, e
	}
	if !p.CanReviewRequests {
		tx.Rollback()
		return nil, p, ErrForbidden
	}
	if !p.Fresh {
		tx.Rollback()
		return nil, p, ErrReauthRequired
	}
	return tx, p, nil
}
func communityHomes(ctx context.Context, q identityReader, in CommunityInput) ([]RecordHome, error) {
	all, e := reviewHomes(ctx, q, Principal{CanReviewRequests: true})
	if e != nil {
		return nil, e
	}
	data, e := json.Marshal(all)
	if e != nil {
		return nil, e
	}
	if TokenHash(string(data)) != in.AreaKey {
		return nil, ErrConflict
	}
	homes := []RecordHome{}
	selected := map[string]bool{}
	for _, id := range in.HomeIDs {
		selected[id] = true
	}
	for _, h := range all {
		if in.Scope == "ALL" || (in.Scope == "WING" && len(h.Label) > len(in.BuildingCode) && h.Label[:len(in.BuildingCode)+1] == in.BuildingCode+"-") || (in.Scope == "HOMES" && selected[h.ID]) {
			homes = append(homes, h)
		}
	}
	if len(homes) == 0 || len(homes) > 118 || (in.Scope == "HOMES" && len(homes) != len(in.HomeIDs)) {
		return nil, invalid("Some area choices are unavailable. Reload the current home options.")
	}
	return homes, nil
}
func communityEvent(ctx context.Context, tx *sql.Tx, p Principal, id string, version, proposal int, action, reason string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO community_events VALUES(?,?,?,?,?,?,?)", id, version, proposal, action, p.ID, reason, time.Now().Unix())
	if e != nil {
		return e
	}
	// Broad audit contains metadata only; the reviewed original and private
	// reasons stay in the community records with their separate access checks.
	return appendAudit(ctx, tx, p.ID, "", "COMMUNITY_"+action, "Community resource decision recorded", map[string]any{}, map[string]any{"id": id, "version": version, "proposal_version": proposal})
}
func (s *Store) ProposeCommunity(ctx context.Context, token, id string, input CommunityInput) (string, error) {
	in, e := normaliseCommunity(input)
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
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "COMMUNITY_PROPOSE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	version := 1
	action := "PROPOSED"
	h := communityHead{}
	var x CommunitySnapshot
	if id == "" {
		if in.Version != 0 || in.Action != "PUBLISH" {
			return "", ErrInvalid
		}
		id = randomToken()
	} else {
		h, e = communityHeadIn(ctx, tx, id)
		if e != nil {
			return "", e
		}
		if h.Version != in.Version {
			return "", ErrConflict
		}
		version = h.Version + 1
		if h.Pending > 0 {
			pending, err := communitySnapshotIn(ctx, tx, id, h.Pending)
			if err != nil {
				return "", err
			}
			if pending.SubmittedBy != p.ID {
				return "", ErrForbidden
			}
			action = "REVISED"
		}
		if in.Action == "PUBLISH" && h.Kind != in.Kind {
			return "", ErrInvalid
		}
		if h.Published > 0 {
			x, e = communitySnapshotIn(ctx, tx, id, h.Published)
			if e != nil {
				return "", e
			}
		}
		if in.Action == "PUBLISH" && h.Kind == "INTERRUPTION" && x.Action == "RESOLVE" {
			return "", invalid("A restored interruption is retained. Propose a new interruption for a new event.")
		}
	}
	if in.Action == "PUBLISH" {
		homes, err := communityHomes(ctx, tx, in)
		if err != nil {
			return "", err
		}
		x = CommunitySnapshot{Kind: in.Kind, Action: in.Action, Title: in.Title, Body: in.Body, Service: in.Service, Phone: in.Phone, Availability: in.Availability, Attestation: in.Attestation, Scope: in.Scope, BuildingCode: in.BuildingCode, Homes: homes, StartAt: in.StartAt, EstimatedEnd: in.EstimatedEnd}
	} else {
		if h.Published == 0 || x.Action == "WITHDRAW" {
			return "", ErrConflict
		}
		if in.Action == "RESOLVE" {
			if h.Kind != "INTERRUPTION" || x.Action == "RESOLVE" || in.ResolvedAt < x.StartAt || in.ResolvedAt > time.Now().Unix() {
				return "", invalid("Restoration must be after the approved start and no later than now.")
			}
			x.ResolvedAt = in.ResolvedAt
			x.UpdateText = in.UpdateText
		} else {
			x.ResolvedAt = 0
			x.UpdateText = ""
		}
		x.Action = in.Action
	}
	x.Version = version
	x.SubmittedBy = p.ID
	x.SubmittedAt = time.Now().Unix()
	x.Reason = in.Reason
	if h.ID == "" {
		_, e = tx.ExecContext(ctx, "INSERT INTO community_resources VALUES(?,?,1,1,1,NULL,'PENDING',?,?)", id, x.Kind, p.ID, x.SubmittedAt)
		if e != nil {
			return "", e
		}
	}
	homes, e := json.Marshal(x.Homes)
	if e != nil {
		return "", e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO community_versions VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, version, x.Kind, x.Action, x.Title, x.Body, x.Service, x.Phone, x.Availability, x.Attestation, x.Scope, x.BuildingCode, string(homes), x.StartAt, x.EstimatedEnd, x.ResolvedAt, x.UpdateText, x.SubmittedBy, x.SubmittedAt, x.Reason)
	if e != nil {
		return "", e
	}
	if e = communityEvent(ctx, tx, p, id, version, version, action, in.Reason); e != nil {
		return "", e
	}
	if h.ID != "" {
		_, e = tx.ExecContext(ctx, "UPDATE community_resources SET version=?,latest_version=?,pending_version=?,decision='PENDING' WHERE id=? AND version=?", version, version, version, id, in.Version)
		if e != nil {
			return "", e
		}
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func (s *Store) DecideCommunity(ctx context.Context, token, id string, in CommunityAction) (string, error) {
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 1 || in.Version > 1000000 || len(id) > 100 || (in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "CANCELLED") {
		return "", invalid("Confirm the exact proposal with a decision and a reason of 10–800 characters.")
	}
	tx, p, e := s.beginCommunityWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "COMMUNITY_DECIDE:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	h, e := communityHeadIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if h.Version != in.Version || h.Pending == 0 {
		return "", ErrConflict
	}
	x, e := communitySnapshotIn(ctx, tx, id, h.Pending)
	if e != nil {
		return "", e
	}
	if (in.Action == "CANCELLED" && p.ID != x.SubmittedBy) || (in.Action != "CANCELLED" && p.ID == x.SubmittedBy) {
		return "", ErrForbidden
	}
	version := h.Version + 1
	if e = communityEvent(ctx, tx, p, id, version, h.Pending, in.Action, in.Reason); e != nil {
		return "", e
	}
	published := optionalCommunityVersion(h.Published)
	if in.Action == "APPROVED" {
		published = h.Pending
	}
	_, e = tx.ExecContext(ctx, "UPDATE community_resources SET version=?,pending_version=NULL,published_version=?,decision=? WHERE id=? AND version=?", version, published, in.Action, id, in.Version)
	if e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
func optionalCommunityVersion(version int) any {
	if version == 0 {
		return nil
	}
	return version
}
