package messaging

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"society.local/portal/internal/database"
)

// Provider is the handoff boundary. The only configured implementation in this
// release is local and persistent; it has no network client or live credentials.
type Provider interface {
	Handoff(context.Context, string, string, string) (database.SimulationHandoff, error)
}
type SyntheticProvider struct{ Store *database.Store }

func (p SyntheticProvider) Handoff(ctx context.Context, token, attempt, outcome string) (database.SimulationHandoff, error) {
	return p.Store.SyntheticMessageHandoff(ctx, token, attempt, outcome)
}

type Engine struct {
	Store    *database.Store
	provider Provider
	key      []byte
}

func New(store *database.Store, key []byte) (*Engine, error) {
	if len(key) != 32 || store == nil {
		return nil, errors.New("A private simulation signing key and store are required.")
	}
	return &Engine{Store: store, provider: SyntheticProvider{Store: store}, key: append([]byte{}, key...)}, nil
}
func (e *Engine) VerifyKey(ctx context.Context, create bool) error {
	sum := sha256.Sum256(e.key)
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := e.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if create {
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO app_metadata(key,value) VALUES('message_key_fingerprint',?)", fingerprint); err != nil {
			return err
		}
	}
	var stored string
	err = tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key='message_key_fingerprint'").Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("restore the separately held message signing key and its database fingerprint")
	}
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(stored), []byte(fingerprint)) {
		return errors.New("message signing key does not match this database")
	}
	return tx.Commit()
}
func (e *Engine) Signature(payload []byte) string {
	mac := hmac.New(sha256.New, e.key)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
func (e *Engine) Callback(ctx context.Context, payload []byte, signature string, now time.Time) error {
	if len(payload) == 0 || len(payload) > 8192 || len(signature) != 64 {
		return database.ErrForbidden
	}
	received, err := hex.DecodeString(signature)
	if err != nil {
		return database.ErrForbidden
	}
	expected, _ := hex.DecodeString(e.Signature(payload))
	if !hmac.Equal(received, expected) {
		return database.ErrForbidden
	}
	var proof database.MessageCallback
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&proof) != nil || decoder.Decode(new(any)) != io.EOF {
		return database.ErrInvalid
	}
	if proof.At < now.Add(-5*time.Minute).Unix() || proof.At > now.Add(5*time.Minute).Unix() {
		return database.ErrForbidden
	}
	hash := sha256.Sum256(payload)
	return e.Store.ReceiveMessageCallback(ctx, proof, hex.EncodeToString(hash[:]))
}
func (e *Engine) callback(ctx context.Context, provider, state string) error {
	// The random event identity is public metadata, never a bearer credential.
	proof := database.MessageCallback{EventID: database.NewMessageEventID(), ProviderID: provider, State: state, At: time.Now().Unix()}
	payload, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	return e.Callback(ctx, payload, e.Signature(payload), time.Now())
}
func (e *Engine) Dispatch(ctx context.Context, token, id string, in database.MessageAction) (result string, err error) {
	claims, result, err := e.Store.ClaimMessageDispatch(ctx, token, id, in)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			if abandoned := e.Store.AbandonMessageClaims(recovery, claims); abandoned != nil {
				err = fmt.Errorf("dispatch interrupted; retained claims need reconciliation: %w", err)
			}
		}
	}()
	for _, claim := range claims {
		h, fail := e.provider.Handoff(ctx, token, claim.ID, in.Outcome)
		if fail != nil {
			return "", fail
		}
		if h.Outcome == "SKIPPED" {
			continue
		}
		if fail = e.Store.CompleteSyntheticMessage(ctx, claim.ID); fail != nil {
			return "", fail
		}
		if in.Outcome == "DELIVERED" || in.Outcome == "READ" {
			if fail = e.callback(ctx, h.ProviderID, "DELIVERED"); fail != nil {
				return "", fail
			}
		}
		if in.Outcome == "READ" {
			if fail = e.callback(ctx, h.ProviderID, "READ"); fail != nil {
				return "", fail
			}
		}
	}
	return result, nil
}
