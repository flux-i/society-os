package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

func (s *Store) ReceiveWhatsAppStatuses(ctx context.Context, statuses []WhatsAppStatus) error {
	if len(statuses) == 0 || len(statuses) > 200 {
		return ErrInvalid
	}
	if err := s.RequireDemo(ctx); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE app_metadata SET value=value WHERE key='message_key_fingerprint'"); err != nil {
		return err
	}
	for _, proof := range statuses {
		if len(proof.EventHash) != 64 || proof.ProviderID == "" || len(proof.ProviderID) > 512 || len(proof.Metadata) > 16384 || !json.Valid([]byte(proof.Metadata)) || proof.At <= 0 || (proof.State != "ACCEPTED" && proof.State != "DELIVERED" && proof.State != "READ" && proof.State != "FAILED") {
			return ErrInvalid
		}
		var exists bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM whatsapp_status_events WHERE event_hash=?)", proof.EventHash).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		var attempt, provider, binding, destination, delivery, currentState string
		var number, currentNumber int
		var acceptedAt, deliveredAt, readAt int64
		where, value := "h.provider_id=?", proof.ProviderID
		if proof.AttemptID != "" {
			where, value = "h.attempt_id=?", proof.AttemptID
		}
		err = tx.QueryRowContext(ctx, `SELECT h.attempt_id,h.provider_id,h.provider_json,d.destination,d.id,d.state,a.attempt_number,d.attempts,d.accepted_at,d.delivered_at,d.read_at FROM whatsapp_handoffs h JOIN message_attempts a ON a.id=h.attempt_id JOIN message_deliveries d ON d.id=a.delivery_id WHERE `+where, value).Scan(&attempt, &provider, &binding, &destination, &delivery, &currentState, &number, &currentNumber, &acceptedAt, &deliveredAt, &readAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrForbidden
		}
		if err != nil {
			return err
		}
		var frozen MessageProvider
		if json.Unmarshal([]byte(binding), &frozen) != nil || frozen.AccountID != proof.AccountID || frozen.PhoneID != proof.PhoneID || destination != proof.Destination || (provider != "" && provider != proof.ProviderID) || (proof.AttemptID != "" && proof.AttemptID != attempt) {
			return ErrForbidden
		}
		if _, err = tx.ExecContext(ctx, "UPDATE whatsapp_handoffs SET provider_id=? WHERE attempt_id=?", proof.ProviderID, attempt); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO whatsapp_status_events(event_hash,attempt_id,provider_id,state,occurred_at,received_at,status_json) VALUES(?,?,?,?,?,?,?)`, proof.EventHash, attempt, proof.ProviderID, proof.State, proof.At, time.Now().Unix(), proof.Metadata); err != nil {
			return err
		}
		eventState, reason := proof.State, "WHATSAPP_SIGNED_STATUS"
		if number != currentNumber {
			eventState, reason = "IGNORED_"+proof.State, "WHATSAPP_EARLIER_ATTEMPT"
		} else {
			next := currentState
			switch proof.State {
			case "ACCEPTED":
				if currentState == "CLAIMED" || currentState == "UNKNOWN" {
					next = "ACCEPTED"
				}
				if acceptedAt == 0 {
					acceptedAt = proof.At
				}
			case "DELIVERED":
				if currentState != "READ" {
					next = "DELIVERED"
				}
				if deliveredAt == 0 {
					deliveredAt = proof.At
				}
			case "READ":
				next = "READ"
				if readAt == 0 {
					readAt = proof.At
				}
			case "FAILED":
				if currentState == "READ" || currentState == "DELIVERED" {
					eventState, reason = "IGNORED_FAILED", "WHATSAPP_LATE_FAILURE"
				} else {
					next = "FAILED"
				}
			}
			if _, err = tx.ExecContext(ctx, `UPDATE message_deliveries SET state=?,reason=?,provider_id=?,accepted_at=?,delivered_at=?,read_at=?,updated_at=? WHERE id=?`, next, reason, proof.ProviderID, acceptedAt, deliveredAt, readAt, time.Now().Unix(), delivery); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='HANDED_OFF',reason='WHATSAPP_SIGNED_STATUS' WHERE attempt_id=? AND state='CLAIMED'", attempt); err != nil {
			return err
		}
		if err = messageDeliveryEvent(ctx, tx, delivery, attempt, eventState, reason, proof.ProviderID, proof.At); err != nil {
			return err
		}
	}
	return tx.Commit()
}
