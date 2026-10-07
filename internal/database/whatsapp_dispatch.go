package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// CheckMessageWrite returns only an already accepted operation or current
// authority. Provider reads happen after this transaction, never under its
// writer lock. Replay cannot cause a second external request.
func (s *Store) CheckMessageWrite(ctx context.Context, token, id, kind string, input any, sourceKind string) (string, error) {
	// Proposals canonicalise selected IDs before hashing. Check the same
	// canonical payload here so an exact retry of an unsorted HTTP selection
	// can return its accepted result before any new provider request.
	if in, ok := input.(MessageInput); ok {
		clean, err := validateMessageInput(in)
		if err != nil {
			return "", err
		}
		input = clean
		sourceKind = clean.SourceKind
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if id != "" {
		x, failure := messageBatchIn(ctx, tx, id)
		if failure != nil {
			return "", failure
		}
		sourceKind = x.Source.Kind
	}
	if sourceKind != "NOTICE" && sourceKind != "RECEIPT" && sourceKind != "STATEMENT" {
		return "", ErrInvalid
	}
	if err = messageAuthority(p, sourceKind, true); err != nil {
		return "", err
	}
	key := ""
	switch in := input.(type) {
	case MessageInput:
		key = in.OperationKey
	case MessageAction:
		key = in.OperationKey
	default:
		return "", ErrInvalid
	}
	if key == "" && kind == "PREVIEW" {
		return "", tx.Commit()
	}
	result, _, err := replayOperation(ctx, tx, p, key, kind, input)
	if err != nil {
		return "", err
	}
	return result, tx.Commit()
}

func (s *Store) PrepareWhatsAppHandoff(ctx context.Context, token, attempt string, provider *MessageProvider) (WhatsAppAttempt, error) {
	if err := s.RequireDemo(ctx); err != nil {
		return WhatsAppAttempt{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return WhatsAppAttempt{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); err != nil {
		return WhatsAppAttempt{}, err
	}
	claim, err := messageClaimIn(ctx, tx, attempt)
	if err != nil {
		return WhatsAppAttempt{}, err
	}
	x, err := messageBatchIn(ctx, tx, claim.BatchID)
	if err != nil {
		return WhatsAppAttempt{}, err
	}
	if x.Provider == nil || !SameMessageProvider(x.Provider, provider) {
		return WhatsAppAttempt{}, ErrConflict
	}
	var prior bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM whatsapp_handoffs WHERE attempt_id=?)", attempt).Scan(&prior); err != nil {
		return WhatsAppAttempt{}, err
	}
	if prior {
		return WhatsAppAttempt{}, ErrConflict
	}
	var destination, state string
	if err = tx.QueryRowContext(ctx, `SELECT d.destination,d.state FROM message_deliveries d JOIN message_attempts a ON a.delivery_id=d.id AND a.attempt_number=d.attempts WHERE a.id=?`, attempt).Scan(&destination, &state); err != nil {
		return WhatsAppAttempt{}, err
	}
	if state != "CLAIMED" {
		return WhatsAppAttempt{}, ErrConflict
	}
	p, auth := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if auth == nil {
		auth = messageAuthority(p, x.Source.Kind, true)
	}
	if auth == nil && p.ID != claim.ActorID {
		auth = ErrForbidden
	}
	if auth != nil || x.State != "APPROVED" {
		if err = suppressMessageDelivery(ctx, tx, claim.DeliveryID, attempt, "DISPATCH_AUTHORITY_ENDED", nil); err != nil {
			return WhatsAppAttempt{}, err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='SKIPPED',reason='DISPATCH_AUTHORITY_ENDED' WHERE attempt_id=? AND state='CLAIMED'", attempt); err != nil {
			return WhatsAppAttempt{}, err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='SKIPPED',reason='DISPATCH_AUTHORITY_ENDED' WHERE delivery_id=? AND disposition='ELIGIBLE'", claim.DeliveryID); err != nil {
			return WhatsAppAttempt{}, err
		}
		if err = tx.Commit(); err != nil {
			return WhatsAppAttempt{}, err
		}
		return WhatsAppAttempt{}, ErrForbidden
	}
	people, eligible, problem, err := messageDeliveryEligibility(ctx, tx, x, claim.DeliveryID)
	if err != nil {
		return WhatsAppAttempt{}, err
	}
	for _, person := range people {
		if person.Reason == "" {
			continue
		}
		status := "SKIPPED"
		if person.Reason == "OPTED_OUT" {
			status = "OPTED_OUT"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state=?,reason=? WHERE attempt_id=? AND resident_id=? AND state='CLAIMED'", status, person.Reason, attempt, person.ID); err != nil {
			return WhatsAppAttempt{}, err
		}
	}
	if eligible == 0 {
		if err = suppressMessageDelivery(ctx, tx, claim.DeliveryID, attempt, problem, people); err != nil {
			return WhatsAppAttempt{}, err
		}
		return WhatsAppAttempt{}, tx.Commit()
	}
	// Persist the uncertain external boundary before leaving the transaction.
	// A crash after this point cannot prove that nothing reached the provider.
	blob, _ := json.Marshal(provider)
	if _, err = tx.ExecContext(ctx, "INSERT INTO whatsapp_handoffs(attempt_id,provider_json,started_at) VALUES(?,?,?)", attempt, string(blob), time.Now().Unix()); err != nil {
		return WhatsAppAttempt{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='HANDED_OFF',reason='' WHERE delivery_id=? AND disposition='ELIGIBLE'", claim.DeliveryID); err != nil {
		return WhatsAppAttempt{}, err
	}
	out := WhatsAppAttempt{ID: attempt, Destination: destination, Link: x.PortalOrigin + x.Source.Link, Provider: provider}
	return out, tx.Commit()
}

func (s *Store) CompleteWhatsAppHandoff(ctx context.Context, attempt string, result WhatsAppResult) error {
	if (result.State != "ACCEPTED" && result.State != "FAILED" && result.State != "UNKNOWN") || result.RetryAfter < 0 || result.RetryAfter > 3600 || len(result.ProviderID) > 512 || (result.State == "ACCEPTED" && result.ProviderID == "") || (result.State != "ACCEPTED" && result.ProviderID != "") {
		return ErrInvalid
	}
	allowed := map[string]bool{"WHATSAPP_ACCEPTED": true, "WHATSAPP_REJECTED": true, "WHATSAPP_RATE_LIMITED": true, "WHATSAPP_UNCERTAIN": true}
	if !allowed[result.Reason] {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE whatsapp_handoffs SET provider_id=provider_id WHERE attempt_id=?", attempt); err != nil {
		return err
	}
	var storedID string
	var previous sql.NullString
	if err = tx.QueryRowContext(ctx, "SELECT provider_id,result_json FROM whatsapp_handoffs WHERE attempt_id=?", attempt).Scan(&storedID, &previous); err != nil {
		return err
	}
	blob, _ := json.Marshal(result)
	if previous.Valid {
		if previous.String != string(blob) {
			return ErrConflict
		}
		return tx.Commit()
	}
	if storedID != "" && result.ProviderID != "" && storedID != result.ProviderID {
		return ErrConflict
	}
	providerID := storedID
	if result.ProviderID != "" {
		providerID = result.ProviderID
	}
	now := time.Now().Unix()
	retryAt := int64(0)
	if result.RetryAfter > 0 {
		retryAt = now + result.RetryAfter
	}
	if _, err = tx.ExecContext(ctx, "UPDATE whatsapp_handoffs SET provider_id=?,result_json=?,completed_at=?,retry_at=? WHERE attempt_id=?", providerID, string(blob), now, retryAt, attempt); err != nil {
		return err
	}
	claim, err := messageClaimIn(ctx, tx, attempt)
	if err != nil {
		return err
	}
	// Signed callbacks may already have advanced this attempt. The HTTP
	// response cannot regress those facts or replace a newer attempt.
	acceptedAt := int64(0)
	if result.State == "ACCEPTED" {
		acceptedAt = now
	}
	if _, err = tx.ExecContext(ctx, `UPDATE message_deliveries SET state=?,reason=?,provider_id=?,accepted_at=CASE WHEN accepted_at=0 THEN ? ELSE accepted_at END,updated_at=? WHERE id=? AND state IN('CLAIMED','UNKNOWN') AND attempts=(SELECT attempt_number FROM message_attempts WHERE id=?)`, result.State, result.Reason, providerID, acceptedAt, now, claim.DeliveryID, attempt); err != nil {
		return err
	}
	if result.State != "UNKNOWN" {
		state := "HANDED_OFF"
		if result.State == "FAILED" {
			state = "REJECTED"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state=?,reason=? WHERE attempt_id=? AND state='CLAIMED'", state, result.Reason, attempt); err != nil {
			return err
		}
	}
	if err = messageDeliveryEvent(ctx, tx, claim.DeliveryID, attempt, "WHATSAPP_RESPONSE_"+result.State, result.Reason, providerID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func reconcileWhatsApp(ctx context.Context, tx *sql.Tx, claim MessageClaim) error {
	var result sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT result_json FROM whatsapp_handoffs WHERE attempt_id=?", claim.ID).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, "UPDATE message_deliveries SET state='FAILED',reason='WHATSAPP_PROVED_UNSENT',updated_at=? WHERE id=? AND state='UNKNOWN'", time.Now().Unix(), claim.DeliveryID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE message_attempt_recipients SET state='SKIPPED',reason='WHATSAPP_PROVED_UNSENT' WHERE attempt_id=? AND state='CLAIMED'", claim.ID); err != nil {
			return err
		}
		return messageDeliveryEvent(ctx, tx, claim.DeliveryID, claim.ID, "FAILED", "WHATSAPP_PROVED_UNSENT", "", time.Now().Unix())
	}
	if err != nil {
		return err
	}
	// No documented query endpoint can prove what happened after an uncertain
	// start. Keep the job unknown until a signed callback supplies evidence.
	return messageDeliveryEvent(ctx, tx, claim.DeliveryID, claim.ID, "UNKNOWN", "WHATSAPP_AWAITING_PROOF", "", time.Now().Unix())
}
