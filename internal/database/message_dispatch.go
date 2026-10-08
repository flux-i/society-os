package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type MessageClaim struct {
	ID         string `json:"id"`
	DeliveryID string `json:"delivery_id"`
	BatchID    string `json:"batch_id"`
	ActorID    string `json:"actor_id"`
}
type SimulationHandoff struct {
	ProviderID string `json:"provider_id"`
	Outcome    string `json:"outcome"`
	AcceptedAt int64  `json:"accepted_at"`
}

func messageOperatorCurrent(ctx context.Context, q identityReader, id, kind string) (bool, error) {
	roles := "'ADMINISTRATOR','COMMITTEE'"
	if financeMessageKind(kind) {
		roles = "'TREASURER'"
	}
	var current bool
	now := time.Now().Unix()
	e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users u WHERE u.id=? AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id) AND EXISTS(SELECT 1 FROM role_grants WHERE user_id=u.id AND role IN(`+roles+`) AND valid_from<=? AND valid_until>? AND revoked_at IS NULL))`, id, now, now).Scan(&current)
	return current, e
}
func messageSourceProblem(ctx context.Context, q identityReader, x MessageBatch) (string, error) {
	for _, id := range []string{x.ProposedBy, x.ReviewedBy} {
		valid, e := messageOperatorCurrent(ctx, q, id, x.Source.Kind)
		if e != nil {
			return "", e
		}
		if !valid {
			return "APPROVAL_AUTHORITY_ENDED", nil
		}
	}
	source, e := messageSourceIn(ctx, q, x.Source.Kind, x.Source.ID)
	if errors.Is(e, sql.ErrNoRows) {
		return "SOURCE_UNAVAILABLE", nil
	}
	if e != nil {
		return "", e
	}
	if source.Reminder != nil && x.Source.Reminder != nil {
		source.Reminder.Basis = x.Source.Reminder.Basis
		source.Reminder.Target = x.Source.Reminder.Target
	}
	before, _ := json.Marshal(x.Source)
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		return "SOURCE_CHANGED", nil
	}
	return "", nil
}
func suppressMessageDelivery(ctx context.Context, tx *sql.Tx, id, attempt, problem string, people []MessageRecipient) error {
	state := "SKIPPED"
	if problem == "OPTED_OUT" {
		state = "OPTED_OUT"
	}
	if _, e := tx.ExecContext(ctx, "UPDATE message_deliveries SET state=?,reason=?,updated_at=? WHERE id=? AND state IN('QUEUED','FAILED','CLAIMED')", state, problem, time.Now().Unix(), id); e != nil {
		return e
	}
	for _, r := range people {
		disposition := "SKIPPED"
		if r.Reason == "OPTED_OUT" {
			disposition = "OPTED_OUT"
		}
		reason := r.Reason
		if reason == "" {
			reason = problem
		}
		if _, e := tx.ExecContext(ctx, "UPDATE message_recipients SET disposition=?,reason=? WHERE delivery_id=? AND resident_id=? AND disposition='ELIGIBLE'", disposition, reason, id, r.ID); e != nil {
			return e
		}
	}
	return messageDeliveryEvent(ctx, tx, id, attempt, state, problem, "", time.Now().Unix())
}
func messageDeliveryEligibility(ctx context.Context, tx *sql.Tx, x MessageBatch, id string) ([]MessageRecipient, int, string, error) {
	var destination, deliveryState string
	if e := tx.QueryRowContext(ctx, "SELECT destination,state FROM message_deliveries WHERE id=? AND batch_id=? AND snapshot_version=?", id, x.ID, x.SnapshotVersion).Scan(&destination, &deliveryState); e != nil {
		return nil, 0, "", e
	}
	// A definite bounce permits a new decision. Earlier per-attempt recipient
	// handoffs remain immutable; this current row is rechecked before any retry.
	if deliveryState == "FAILED" {
		if _, e := tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='ELIGIBLE',reason='' WHERE delivery_id=? AND disposition='HANDED_OFF'", id); e != nil {
			return nil, 0, "", e
		}
	}
	global, e := messageSourceProblem(ctx, tx, x)
	if e != nil {
		return nil, 0, "", e
	}
	rows, e := tx.QueryContext(ctx, "SELECT resident_id,contact_version FROM message_recipients WHERE delivery_id=? AND disposition='ELIGIBLE' ORDER BY resident_id", id)
	if e != nil {
		return nil, 0, "", e
	}
	frozen := []MessageRecipient{}
	for rows.Next() {
		var r MessageRecipient
		if e = rows.Scan(&r.ID, &r.ContactVersion); e != nil {
			rows.Close()
			return nil, 0, "", e
		}
		frozen = append(frozen, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, 0, "", e
	}
	people, e := messagePeople(ctx, tx)
	if e != nil {
		return nil, 0, "", e
	}
	lookup := map[string]messagePerson{}
	for _, person := range people {
		lookup[person.ID] = person
	}
	eligible := 0
	allOpted := len(frozen) > 0
	for i := range frozen {
		r := &frozen[i]
		if global != "" {
			r.Reason = global
		} else {
			person, exists := lookup[r.ID]
			if !exists {
				r.Reason = "NO_CURRENT_HOME"
			} else {
				reason, dest := messageRecipientReason(person, x.Source, x.Channel, x.Purpose)
				r.Reason = reason
				if r.Reason == "" && x.Source.Reminder != nil {
					r.Reason, e = reminderDispatchReason(ctx, tx, x, person)
					if e != nil {
						return nil, 0, "", e
					}
				}
				if reason == "" && (person.Contact.Version != r.ContactVersion || dest != destination) {
					r.Reason = "CONTACT_CHANGED"
				}
			}
		}
		if r.Reason == "" {
			eligible++
		} else {
			if r.Reason != "OPTED_OUT" {
				allOpted = false
			}
			disposition := "SKIPPED"
			if r.Reason == "OPTED_OUT" {
				disposition = "OPTED_OUT"
			}
			if _, e = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition=?,reason=? WHERE delivery_id=? AND resident_id=? AND disposition='ELIGIBLE'", disposition, r.Reason, id, r.ID); e != nil {
				return nil, 0, "", e
			}
		}
	}
	problem := global
	if eligible == 0 && problem == "" {
		problem = "NO_ELIGIBLE_RECIPIENT"
		if allOpted {
			problem = "OPTED_OUT"
		}
	}
	return frozen, eligible, problem, nil
}
func (s *Store) ClaimMessageDispatch(ctx context.Context, token, id string, in MessageAction) ([]MessageClaim, string, error) {
	if in.Action != "DISPATCH" || in.Version < 1 || !in.Confirmed || !validText(in.Reason, 5, 300) {
		return nil, "", ErrInvalid
	}
	if in.Outcome != "" && in.Outcome != "ACCEPTED" && in.Outcome != "DELIVERED" && in.Outcome != "READ" && in.Outcome != "REJECTED" && in.Outcome != "UNKNOWN" {
		return nil, "", ErrInvalid
	}
	if e := s.RequireDemo(ctx); e != nil {
		return nil, "", e
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return nil, "", e
	}
	defer tx.Rollback()
	x, e := messageBatchIn(ctx, tx, id)
	if e != nil {
		return nil, "", e
	}
	if e = messageAuthority(p, x.Source.Kind, true); e != nil {
		return nil, "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MESSAGE_DISPATCH:"+id, in)
	if e != nil {
		return nil, "", e
	}
	if result != "" {
		return []MessageClaim{}, result, tx.Commit()
	}
	if (x.Provider == nil && in.Outcome == "") || (x.Provider != nil && (in.Outcome != "" || !SameMessageProvider(x.Provider, in.Provider))) {
		return nil, "", ErrInvalid
	}
	if x.Version != in.Version || x.State != "APPROVED" {
		return nil, "", ErrConflict
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,attempts FROM message_deliveries WHERE batch_id=? AND snapshot_version=? AND state IN('QUEUED','FAILED') AND attempts<3 AND NOT EXISTS(SELECT 1 FROM whatsapp_handoffs h JOIN message_attempts a ON a.id=h.attempt_id WHERE a.delivery_id=message_deliveries.id AND a.attempt_number=message_deliveries.attempts AND h.retry_at>?) ORDER BY id LIMIT 25`, id, x.SnapshotVersion, time.Now().Unix())
	if e != nil {
		return nil, "", e
	}
	type item struct {
		id string
		n  int
	}
	pending := []item{}
	for rows.Next() {
		var item item
		if e = rows.Scan(&item.id, &item.n); e != nil {
			rows.Close()
			return nil, "", e
		}
		pending = append(pending, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, "", e
	}
	if len(pending) == 0 {
		return nil, "", ErrConflict
	}
	claims := []MessageClaim{}
	for _, d := range pending {
		people, eligible, problem, e := messageDeliveryEligibility(ctx, tx, x, d.id)
		if e != nil {
			return nil, "", e
		}
		if eligible == 0 {
			if e = suppressMessageDelivery(ctx, tx, d.id, "", problem, people); e != nil {
				return nil, "", e
			}
			continue
		}
		claim := MessageClaim{ID: randomToken(), DeliveryID: d.id, BatchID: id, ActorID: p.ID}
		if _, e = tx.ExecContext(ctx, `INSERT INTO message_attempts(id,delivery_id,attempt_number,actor_id,operation_key,started_at) VALUES(?,?,?,?,?,?)`, claim.ID, d.id, d.n+1, p.ID, in.OperationKey, time.Now().Unix()); e != nil {
			return nil, "", e
		}
		for _, person := range people {
			if person.Reason == "" {
				if _, e = tx.ExecContext(ctx, "INSERT INTO message_attempt_recipients(attempt_id,resident_id,state,reason) VALUES(?,?,'CLAIMED','')", claim.ID, person.ID); e != nil {
					return nil, "", e
				}
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state='CLAIMED',attempts=attempts+1,reason='',provider_id='',updated_at=? WHERE id=?", time.Now().Unix(), d.id); e != nil {
			return nil, "", e
		}
		reason := "Local simulation claim."
		if x.Provider != nil {
			reason = "WHATSAPP_CLAIMED"
		}
		if e = messageDeliveryEvent(ctx, tx, d.id, claim.ID, "CLAIMED", reason, "", time.Now().Unix()); e != nil {
			return nil, "", e
		}
		claims = append(claims, claim)
	}
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	if _, e = tx.ExecContext(ctx, "UPDATE message_batches SET version=?,updated_at=? WHERE id=?", x.Version, x.UpdatedAt, id); e != nil {
		return nil, "", e
	}
	if e = messageBatchEvent(ctx, tx, p, x, "DISPATCH", in.Reason); e != nil {
		return nil, "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return nil, "", e
	}
	return claims, id, tx.Commit()
}
func messageClaimIn(ctx context.Context, q identityReader, id string) (MessageClaim, error) {
	var claim MessageClaim
	e := q.QueryRowContext(ctx, `SELECT a.id,a.delivery_id,d.batch_id,a.actor_id FROM message_attempts a JOIN message_deliveries d ON d.id=a.delivery_id WHERE a.id=?`, id).Scan(&claim.ID, &claim.DeliveryID, &claim.BatchID, &claim.ActorID)
	return claim, e
}
func simulationHandoffIn(ctx context.Context, q identityReader, attempt string) (SimulationHandoff, error) {
	var h SimulationHandoff
	e := q.QueryRowContext(ctx, "SELECT provider_id,outcome,accepted_at FROM simulation_messages WHERE attempt_id=?", attempt).Scan(&h.ProviderID, &h.Outcome, &h.AcceptedAt)
	return h, e
}

// This adapter has no network client and accepts only explicitly marked demo data.
// Its separate durable record models acceptance outside the portal outcome write.
func (s *Store) SyntheticMessageHandoff(ctx context.Context, token, attempt, outcome string) (SimulationHandoff, error) {
	if outcome != "ACCEPTED" && outcome != "DELIVERED" && outcome != "READ" && outcome != "REJECTED" && outcome != "UNKNOWN" {
		return SimulationHandoff{}, ErrInvalid
	}
	if e := s.RequireDemo(ctx); e != nil {
		return SimulationHandoff{}, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return SimulationHandoff{}, e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); e != nil {
		return SimulationHandoff{}, e
	}
	claim, e := messageClaimIn(ctx, tx, attempt)
	if e != nil {
		return SimulationHandoff{}, e
	}
	x, e := messageBatchIn(ctx, tx, claim.BatchID)
	if e != nil {
		return SimulationHandoff{}, e
	}
	if x.Provider != nil {
		return SimulationHandoff{}, ErrForbidden
	}
	var state string
	if e = tx.QueryRowContext(ctx, "SELECT state FROM message_deliveries WHERE id=?", claim.DeliveryID).Scan(&state); e != nil {
		return SimulationHandoff{}, e
	}
	if state != "CLAIMED" {
		return SimulationHandoff{}, ErrConflict
	}
	p, auth := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if auth == nil {
		auth = messageAuthority(p, x.Source.Kind, true)
		if auth == nil && p.ID != claim.ActorID {
			auth = ErrForbidden
		}
	}
	if auth != nil || x.State != "APPROVED" {
		// A known earlier handoff remains true even if its actor subsequently
		// loses authority. Deny replay without relabelling it as unsent.
		if _, prior := simulationHandoffIn(ctx, tx, attempt); prior == nil {
			if auth != nil {
				return SimulationHandoff{}, auth
			}
			return SimulationHandoff{}, ErrConflict
		} else if !errors.Is(prior, sql.ErrNoRows) {
			return SimulationHandoff{}, prior
		}
		problem := "DISPATCH_AUTHORITY_ENDED"
		if x.State != "APPROVED" {
			problem = "PROPOSAL_CANCELLED"
		}
		if e = suppressMessageDelivery(ctx, tx, claim.DeliveryID, attempt, problem, nil); e != nil {
			return SimulationHandoff{}, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='SKIPPED',reason=? WHERE delivery_id=? AND disposition='ELIGIBLE'", problem, claim.DeliveryID); e != nil {
			return SimulationHandoff{}, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='SKIPPED',reason=? WHERE attempt_id=? AND state='CLAIMED'", problem, attempt); e != nil {
			return SimulationHandoff{}, e
		}
		if e = tx.Commit(); e != nil {
			return SimulationHandoff{}, e
		}
		if auth != nil {
			return SimulationHandoff{}, auth
		}
		return SimulationHandoff{}, ErrConflict
	}
	if h, e := simulationHandoffIn(ctx, tx, attempt); e == nil {
		return h, tx.Commit()
	} else if !errors.Is(e, sql.ErrNoRows) {
		return SimulationHandoff{}, e
	}
	people, eligible, problem, e := messageDeliveryEligibility(ctx, tx, x, claim.DeliveryID)
	if e != nil {
		return SimulationHandoff{}, e
	}
	if eligible == 0 {
		if e = suppressMessageDelivery(ctx, tx, claim.DeliveryID, attempt, problem, people); e != nil {
			return SimulationHandoff{}, e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='SKIPPED',reason=? WHERE attempt_id=? AND state='CLAIMED'", problem, attempt); e != nil {
			return SimulationHandoff{}, e
		}
		return SimulationHandoff{Outcome: "SKIPPED"}, tx.Commit()
	}
	h := SimulationHandoff{ProviderID: "simulation-" + randomToken(), Outcome: outcome, AcceptedAt: time.Now().Unix()}
	if outcome == "REJECTED" {
		h.AcceptedAt = 0
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO simulation_messages(provider_id,attempt_id,outcome,accepted_at,simulation) VALUES(?,?,?,?,1)", h.ProviderID, attempt, outcome, h.AcceptedAt); e != nil {
		return h, e
	}
	if outcome != "REJECTED" {
		if _, e = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='HANDED_OFF',reason='' WHERE delivery_id=? AND disposition='ELIGIBLE'", claim.DeliveryID); e != nil {
			return h, e
		}
	}
	for _, person := range people {
		status, reason := "HANDED_OFF", ""
		if outcome == "REJECTED" {
			status, reason = "REJECTED", "The simulation rejected this attempt."
		}
		if person.Reason != "" {
			status, reason = "SKIPPED", person.Reason
			if person.Reason == "OPTED_OUT" {
				status = "OPTED_OUT"
			}
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state=?,reason=? WHERE attempt_id=? AND resident_id=? AND state='CLAIMED'", status, reason, attempt, person.ID); e != nil {
			return h, e
		}
	}
	return h, tx.Commit()
}
func completeSimulationHandoff(ctx context.Context, tx *sql.Tx, claim MessageClaim, reconciled bool) error {
	h, e := simulationHandoffIn(ctx, tx, claim.ID)
	if e != nil {
		return e
	}
	state, reason := "ACCEPTED", "Accepted by the local simulation; delivery has not been reported."
	if h.Outcome == "REJECTED" {
		state, reason = "FAILED", "The local simulation definitively rejected this attempt."
	}
	if h.Outcome == "UNKNOWN" && !reconciled {
		state, reason = "UNKNOWN", "The local simulation response was lost after handoff. Reconcile before any new attempt."
	}
	provider, acceptedAt := h.ProviderID, h.AcceptedAt
	if state == "UNKNOWN" {
		provider, acceptedAt = "", 0
	}
	// Reconciliation knows only acceptance here. Delivery/read need actual callback events.
	if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state=?,reason=?,provider_id=?,accepted_at=?,updated_at=? WHERE id=? AND state IN('CLAIMED','UNKNOWN')", state, reason, provider, acceptedAt, time.Now().Unix(), claim.DeliveryID); e != nil {
		return e
	}
	return messageDeliveryEvent(ctx, tx, claim.DeliveryID, claim.ID, state, reason, provider, time.Now().Unix())
}
func (s *Store) CompleteSyntheticMessage(ctx context.Context, attempt string) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	// Acquire the writer lock before testing the current outcome.
	if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET updated_at=updated_at WHERE id=(SELECT delivery_id FROM message_attempts WHERE id=?)", attempt); e != nil {
		return e
	}
	claim, e := messageClaimIn(ctx, tx, attempt)
	if e != nil {
		return e
	}
	var state string
	if e = tx.QueryRowContext(ctx, "SELECT state FROM message_deliveries WHERE id=?", claim.DeliveryID).Scan(&state); e != nil {
		return e
	}
	if state != "CLAIMED" {
		return tx.Commit()
	}
	if e = completeSimulationHandoff(ctx, tx, claim, false); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) RecoverMessageClaims(ctx context.Context) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET updated_at=updated_at WHERE state='CLAIMED'"); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, `SELECT d.id,a.id FROM message_deliveries d JOIN message_attempts a ON a.delivery_id=d.id AND a.attempt_number=d.attempts WHERE d.state='CLAIMED'`)
	if e != nil {
		return e
	}
	claims := []MessageClaim{}
	for rows.Next() {
		var c MessageClaim
		if e = rows.Scan(&c.DeliveryID, &c.ID); e != nil {
			rows.Close()
			return e
		}
		claims = append(claims, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, c := range claims {
		if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state='UNKNOWN',reason='An unresolved handoff survived restart; reconcile before retry.',updated_at=? WHERE id=?", time.Now().Unix(), c.DeliveryID); e != nil {
			return e
		}
		if e = messageDeliveryEvent(ctx, tx, c.DeliveryID, c.ID, "UNKNOWN", "An unresolved handoff survived restart; reconcile before retry.", "", time.Now().Unix()); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) ReconcileMessage(ctx context.Context, token, id, delivery string, in MessageAction) (string, error) {
	if in.Action != "RECONCILE" || in.Outcome != "" || in.Version < 1 || !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := messageBatchIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if e = messageAuthority(p, x.Source.Kind, true); e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MESSAGE_RECONCILE:"+id+":"+delivery, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	var attempt, state string
	e = tx.QueryRowContext(ctx, `SELECT a.id,d.state FROM message_deliveries d JOIN message_attempts a ON a.delivery_id=d.id AND a.attempt_number=d.attempts WHERE d.id=? AND d.batch_id=? AND d.snapshot_version=?`, delivery, id, x.SnapshotVersion).Scan(&attempt, &state)
	if e != nil {
		return "", e
	}
	if state != "UNKNOWN" {
		return "", ErrConflict
	}
	claim, e := messageClaimIn(ctx, tx, attempt)
	if e != nil {
		return "", e
	}
	if x.Provider != nil {
		if e = reconcileWhatsApp(ctx, tx, claim); e != nil {
			return "", e
		}
	} else if _, e = simulationHandoffIn(ctx, tx, attempt); errors.Is(e, sql.ErrNoRows) {
		if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state='FAILED',reason='Reconciliation proves no local simulation handoff occurred.',updated_at=? WHERE id=?", time.Now().Unix(), delivery); e != nil {
			return "", e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='SKIPPED',reason='Reconciliation proves no local simulation handoff occurred.' WHERE attempt_id=? AND state='CLAIMED'", attempt); e != nil {
			return "", e
		}
		if e = messageDeliveryEvent(ctx, tx, delivery, attempt, "FAILED", "Reconciliation proves no local simulation handoff occurred.", "", time.Now().Unix()); e != nil {
			return "", e
		}
	} else if e != nil {
		return "", e
	} else if e = completeSimulationHandoff(ctx, tx, claim, true); e != nil {
		return "", e
	}
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	if _, e = tx.ExecContext(ctx, "UPDATE message_batches SET version=?,updated_at=? WHERE id=?", x.Version, x.UpdatedAt, id); e != nil {
		return "", e
	}
	if e = messageBatchEvent(ctx, tx, p, x, "RECONCILE", in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
