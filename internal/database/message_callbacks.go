package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type MessageCallback struct {
	EventID    string `json:"event_id"`
	ProviderID string `json:"provider_id"`
	State      string `json:"state"`
	At         int64  `json:"at"`
}

func NewMessageEventID() string { return randomToken() }

// Only the verified simulation engine calls this boundary. Browser identities
// cannot manufacture proof of delivery with a normal message action.
func (s *Store) ReceiveMessageCallback(ctx context.Context, in MessageCallback, payloadHash string) error {
	if !operationPattern.MatchString(in.EventID) || len(in.ProviderID) > 100 || in.ProviderID == "" || len(payloadHash) != 64 || in.At <= 0 || (in.State != "ACCEPTED" && in.State != "DELIVERED" && in.State != "READ" && in.State != "FAILED") {
		return ErrInvalid
	}
	if e := s.RequireDemo(ctx); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET updated_at=updated_at WHERE id=(SELECT a.delivery_id FROM simulation_messages p JOIN message_attempts a ON a.id=p.attempt_id WHERE p.provider_id=?)", in.ProviderID); e != nil {
		return e
	}
	var prior string
	e = tx.QueryRowContext(ctx, "SELECT payload_hash FROM message_callbacks WHERE event_id=?", in.EventID).Scan(&prior)
	if e == nil {
		if prior != payloadHash {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	var delivery, attempt, state, outcome string
	var acceptedAt, deliveredAt, readAt, providerAccepted int64
	var number, currentNumber int
	e = tx.QueryRowContext(ctx, `SELECT d.id,a.id,d.state,d.accepted_at,d.delivered_at,d.read_at,p.accepted_at,p.outcome,a.attempt_number,d.attempts FROM simulation_messages p JOIN message_attempts a ON a.id=p.attempt_id JOIN message_deliveries d ON d.id=a.delivery_id WHERE p.provider_id=?`, in.ProviderID).Scan(&delivery, &attempt, &state, &acceptedAt, &deliveredAt, &readAt, &providerAccepted, &outcome, &number, &currentNumber)
	if e != nil {
		return e
	}
	if outcome == "REJECTED" {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO message_callbacks(event_id,provider_id,delivery_id,state,occurred_at,received_at,payload_hash) VALUES(?,?,?,?,?,?,?)", in.EventID, in.ProviderID, delivery, in.State, in.At, time.Now().Unix(), payloadHash); e != nil {
		return e
	}
	reason, eventState := "Simulation callback recorded.", in.State
	if number != currentNumber {
		reason, eventState = "A callback for an earlier attempt was retained without changing the current attempt.", "IGNORED_"+in.State
	} else {
		next := state
		switch in.State {
		case "ACCEPTED":
			if state == "CLAIMED" || state == "UNKNOWN" {
				next = "ACCEPTED"
			}
		case "DELIVERED":
			if state != "READ" {
				next = "DELIVERED"
			}
			if deliveredAt == 0 {
				deliveredAt = in.At
			}
		case "READ":
			next = "READ"
			if readAt == 0 {
				readAt = in.At
			}
		case "FAILED":
			if state == "DELIVERED" || state == "READ" {
				reason, eventState = "The late failure was retained; delivered/read status did not regress.", "IGNORED_FAILED"
			} else {
				next = "FAILED"
				reason = "The simulation explicitly reported a definitive failure or bounce."
			}
		}
		if acceptedAt == 0 {
			acceptedAt = providerAccepted
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state=?,reason=?,provider_id=?,accepted_at=?,delivered_at=?,read_at=?,updated_at=? WHERE id=?", next, reason, in.ProviderID, acceptedAt, deliveredAt, readAt, time.Now().Unix(), delivery); e != nil {
			return e
		}
	}
	if e = messageDeliveryEvent(ctx, tx, delivery, attempt, eventState, reason, in.ProviderID, in.At); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) AbandonMessageClaims(ctx context.Context, claims []MessageClaim) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, claim := range claims {
		result, e := tx.ExecContext(ctx, "UPDATE message_deliveries SET state='UNKNOWN',reason='The handoff could not be resolved; reconcile before retry.',updated_at=? WHERE id=? AND state='CLAIMED' AND attempts=(SELECT attempt_number FROM message_attempts WHERE id=?)", time.Now().Unix(), claim.DeliveryID, claim.ID)
		if e != nil {
			return e
		}
		changed, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if changed > 0 {
			if e = messageDeliveryEvent(ctx, tx, claim.DeliveryID, claim.ID, "UNKNOWN", "The handoff could not be resolved; reconcile before retry.", "", time.Now().Unix()); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}
