package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type RecoverySeriesRequest struct {
	FormatVersion int                   `json:"format_version"`
	OperationID   string                `json:"operation_id"`
	AttemptKey    string                `json:"attempt_key"`
	EpochID       string                `json:"epoch_id"`
	SourceSHA256  string                `json:"source_sha256"`
	NominationID  string                `json:"nomination_id"`
	PolicyID      string                `json:"policy_id"`
	PolicyVersion int                   `json:"policy_version"`
	Custodians    []RecoveryAttestation `json:"custodians"`
}

type RecoverySeriesRetry struct {
	FormatVersion int                   `json:"format_version"`
	OperationID   string                `json:"operation_id"`
	AttemptKey    string                `json:"attempt_key"`
	Custodians    []RecoveryAttestation `json:"custodians"`
}

type RecoverySeriesManifest struct {
	FormatVersion int                   `json:"format_version"`
	Request       RecoverySeriesRequest `json:"request"`
	SocietyKey    string                `json:"society_key"`
	ConfigHash    string                `json:"config_hash"`
	ObjectKey     string                `json:"object_key"`
	Policy        RecoveryNumberingSpec `json:"policy"`
}

// This is an offline custody boundary. No HTTP route serializes its ciphertext,
// source/configuration/object identities or exact provider version.
type RecoverySeriesIntent struct {
	OperationID, PolicyID, EpochID, NominationID, SourceSHA256 string
	Prefix, SocietyKey, ConfigHash, ObjectKey, ManifestJSON    string
	InputDigest, CipherSHA256, State, AttemptKey, Version      string
	Ciphertext                                                 []byte
	PolicyVersion, Attempts                                    int
	CipherBytes, CreatedAt, UpdatedAt                          int64
}

func ParseRecoverySeriesRequest(body []byte) (RecoverySeriesRequest, error) {
	var in RecoverySeriesRequest
	if len(body) > 8192 || strictJSON(body, &in) != nil || !validSeriesRequest(in) {
		return in, invalid("Supply a bounded strict version-1 reservation request with exact current source, appointment, approved policy and two custodian attestations.")
	}
	return in, nil
}

func validSeriesRequest(in RecoverySeriesRequest) bool {
	return in.FormatVersion == 1 && operationPattern.MatchString(in.OperationID) && operationPattern.MatchString(in.AttemptKey) && in.OperationID != in.AttemptKey &&
		len(in.EpochID) == 43 && custodyDigest.MatchString(in.SourceSHA256) && sourceIdentity.MatchString(in.NominationID) && len(in.PolicyID) == 43 && in.PolicyVersion > 1 && len(in.Custodians) == 2
}

func ParseRecoverySeriesRetry(body []byte) (RecoverySeriesRetry, error) {
	var in RecoverySeriesRetry
	if len(body) > 8192 || strictJSON(body, &in) != nil || in.FormatVersion != 1 || !operationPattern.MatchString(in.OperationID) || !operationPattern.MatchString(in.AttemptKey) || in.OperationID == in.AttemptKey || len(in.Custodians) != 2 {
		return in, invalid("Supply a strict version-1 known-rejection retry identity and two current custodian attestations.")
	}
	return in, nil
}

func recoveryNumberingCustody(ctx context.Context, q identityReader, epoch, nomination string) (Principal, RecoveryOrigin, error) {
	origin, err := currentRecoveryEpoch(ctx, q)
	if err != nil {
		return Principal{}, origin, err
	}
	if origin.ID != epoch {
		return Principal{}, origin, ErrConflict
	}
	var current string
	err = q.QueryRowContext(ctx, `SELECT operation_id FROM recovery_nominations n WHERE epoch_id=? AND sequence=(SELECT MAX(sequence) FROM recovery_nominations WHERE epoch_id=?) AND valid_from<=unixepoch('now') AND valid_until>unixepoch('now') AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)`, epoch, epoch).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrForbidden
	}
	if err != nil {
		return Principal{}, origin, err
	}
	if current != nomination {
		return Principal{}, origin, ErrConflict
	}
	return Principal{recoveryEpoch: epoch, recoveryNomination: nomination}, origin, nil
}

func recoverySocietyKey(ctx context.Context, q identityReader) (string, error) {
	var key string
	err := q.QueryRowContext(ctx, "SELECT society_key FROM workspace_setup WHERE id=1").Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		var kind, fixture string
		err = q.QueryRowContext(ctx, "SELECT (SELECT value FROM app_metadata WHERE key='data_kind'),(SELECT value FROM app_metadata WHERE key='fixture_version')").Scan(&kind, &fixture)
		if err == nil && kind == "synthetic" && fixture == FixtureVersion {
			return "fictional-demo-" + FixtureVersion, nil
		}
		return "", ErrForbidden
	}
	if err == nil && !sourceIdentity.MatchString(key) {
		err = ErrInvalid
	}
	return key, err
}

// Random epoch, operation, proposal and ciphertext identities deliberately do
// not contribute to the key. Older restores collide on the same society/series.
func RecoverySeriesObjectKey(prefix, society, series string) string {
	return prefix + "/receipt-series/" + InputDigest([]byte(society)) + "/" + InputDigest([]byte(series)) + ".age"
}

const seriesIntentSelect = `SELECT i.operation_id,i.policy_id,i.policy_version,i.epoch_id,i.nomination_id,i.source_sha256,i.series_prefix,i.society_key,i.config_hash,i.object_key,i.manifest_json,i.input_digest,i.cipher_sha256,i.cipher_bytes,i.ciphertext,i.created_at,COALESCE(e.state,'PREPARED'),COALESCE(e.attempt,0),COALESCE(e.attempt_key,''),COALESCE(e.object_version,''),COALESCE(e.created_at,i.created_at)
 FROM recovery_series_intents i LEFT JOIN recovery_series_events e ON e.sequence=(SELECT MAX(sequence) FROM recovery_series_events WHERE intent_id=i.operation_id)`

func scanSeries(row interface{ Scan(...any) error }) (RecoverySeriesIntent, error) {
	var x RecoverySeriesIntent
	err := row.Scan(&x.OperationID, &x.PolicyID, &x.PolicyVersion, &x.EpochID, &x.NominationID, &x.SourceSHA256, &x.Prefix, &x.SocietyKey, &x.ConfigHash, &x.ObjectKey, &x.ManifestJSON, &x.InputDigest, &x.CipherSHA256, &x.CipherBytes, &x.Ciphertext, &x.CreatedAt, &x.State, &x.Attempts, &x.AttemptKey, &x.Version, &x.UpdatedAt)
	return x, err
}

func seriesIntent(ctx context.Context, q identityReader, id string) (RecoverySeriesIntent, error) {
	return scanSeries(q.QueryRowContext(ctx, seriesIntentSelect+" WHERE i.operation_id=?", id))
}

func (s *Store) RecoverySeriesForCustody(ctx context.Context, id string) (RecoverySeriesIntent, error) {
	if !operationPattern.MatchString(id) {
		return RecoverySeriesIntent{}, ErrInvalid
	}
	return seriesIntent(ctx, s.DB, id)
}

// Seal runs only for the one new intent under the reserved writer, never on an
// exact retry. The callback encrypts locally; it must perform no network I/O.
func (s *Store) RetainRecoverySeries(ctx context.Context, in RecoverySeriesRequest, configHash, prefix string, seal func([]byte) ([]byte, error)) (RecoverySeriesIntent, error) {
	var x RecoverySeriesIntent
	if !validSeriesRequest(in) || !custodyDigest.MatchString(configHash) || seal == nil || len(prefix) > 120 || !recoveryStoragePrefix.MatchString(prefix) || strings.Contains(prefix, "..") || strings.HasPrefix(prefix, "/") || strings.HasSuffix(prefix, "/") {
		return x, ErrInvalid
	}
	tx, err := recoveryWriter(ctx, s)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	p, origin, err := recoveryNumberingCustody(ctx, tx, in.EpochID, in.NominationID)
	if err != nil {
		return x, err
	}
	if origin.SourceSHA256 != in.SourceSHA256 {
		return x, ErrConflict
	}
	policy, err := numberingPolicy(ctx, tx, p, in.PolicyID)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrConflict
	}
	if err != nil {
		return x, err
	}
	if policy.State != "APPROVED" || policy.Version != in.PolicyVersion {
		return x, ErrConflict
	}
	society, err := recoverySocietyKey(ctx, tx)
	if err != nil {
		return x, err
	}
	manifest := RecoverySeriesManifest{1, in, society, configHash, RecoverySeriesObjectKey(prefix, society, policy.Input.Policy.Prefix), policy.Input.Policy}
	body, _ := json.Marshal(manifest)
	digest := InputDigest(body)
	x, err = seriesIntent(ctx, tx, in.OperationID)
	if err == nil {
		if x.InputDigest != digest {
			return x, ErrConflict
		}
		return x, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, err
	}
	if !recoveryCustodians(in.Custodians, time.Now()) {
		return x, ErrInvalid
	}
	var used bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM recovery_series_intents WHERE policy_id=? OR series_prefix=?)", policy.ID, policy.Input.Policy.Prefix).Scan(&used); err != nil {
		return x, err
	}
	if used {
		return x, ErrConflict
	}
	cipher, err := seal(body)
	if err != nil {
		return x, err
	}
	if len(cipher) < 32 || len(cipher) > 65536 {
		return x, ErrInvalid
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO recovery_series_intents VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", in.OperationID, policy.ID, policy.Version, in.EpochID, in.NominationID, in.SourceSHA256, policy.Input.Policy.Prefix, society, configHash, manifest.ObjectKey, string(body), digest, InputDigest(cipher), len(cipher), cipher, time.Now().Unix())
	if err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, policy.AuthorID, policy.AuthorID, "RECOVERY_SERIES_RETAINED", "Independent supplied custodians retained a new encrypted series reservation intent.", map[string]any{"policy_id": policy.ID, "input_digest": digest, "prepared_only": true}); err != nil {
		return x, err
	}
	x, err = seriesIntent(ctx, tx, in.OperationID)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func currentSeriesForHandoff(ctx context.Context, q identityReader, x RecoverySeriesIntent) error {
	p, origin, err := recoveryNumberingCustody(ctx, q, x.EpochID, x.NominationID)
	if err != nil {
		return err
	}
	if origin.SourceSHA256 != x.SourceSHA256 {
		return ErrConflict
	}
	policy, err := numberingPolicy(ctx, q, p, x.PolicyID)
	if err != nil {
		return err
	}
	if policy.State != "APPROVED" || policy.Version != x.PolicyVersion {
		return ErrConflict
	}
	return nil
}

// started=false means a retained attempt/result, never permission to PUT again.
func (s *Store) StartRecoverySeriesHandoff(ctx context.Context, id, key string, retry *RecoverySeriesRetry) (RecoverySeriesIntent, bool, error) {
	var x RecoverySeriesIntent
	if !operationPattern.MatchString(id) || !operationPattern.MatchString(key) || id == key {
		return x, false, ErrInvalid
	}
	tx, err := recoveryWriter(ctx, s)
	if err != nil {
		return x, false, err
	}
	defer tx.Rollback()
	x, err = seriesIntent(ctx, tx, id)
	if err != nil {
		return x, false, err
	}
	if err = currentSeriesForHandoff(ctx, tx, x); err != nil {
		return x, false, err
	}
	var savedID string
	err = tx.QueryRowContext(ctx, "SELECT intent_id FROM recovery_series_events WHERE state='HANDOFF' AND attempt_key=?", key).Scan(&savedID)
	if err == nil {
		if savedID != id {
			return x, false, ErrConflict
		}
		return x, false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, false, err
	}
	var manifest RecoverySeriesManifest
	if err = json.Unmarshal([]byte(x.ManifestJSON), &manifest); err != nil {
		return x, false, err
	}
	if retry == nil {
		if x.State != "PREPARED" || key != manifest.Request.AttemptKey {
			return x, false, ErrConflict
		}
	} else if retry.FormatVersion != 1 || retry.OperationID != id || retry.AttemptKey != key || !recoveryCustodians(retry.Custodians, time.Now()) || x.State != "REJECTED" || x.Attempts >= 3 {
		return x, false, ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO recovery_series_events SELECT COALESCE(MAX(sequence),0)+1,?,'HANDOFF',?,?,'','HANDOFF_RETAINED',? FROM recovery_series_events`, id, x.Attempts+1, key, time.Now().Unix())
	if err != nil {
		return x, false, err
	}
	x, err = seriesIntent(ctx, tx, id)
	if err != nil {
		return x, false, err
	}
	return x, true, tx.Commit()
}

// Exact verified evidence can be appended historically after scope changes;
// it grants no current authority and cannot turn a stale policy into a release.
func (s *Store) FinishRecoverySeriesHandoff(ctx context.Context, id, key, state, code, version string) (RecoverySeriesIntent, error) {
	var x RecoverySeriesIntent
	validCode := map[string]bool{"UPLOAD_DENIED": true, "UPLOAD_REJECTED": true, "UPLOAD_UNCERTAIN": true, "HANDOFF_INTERRUPTED": true, "STORAGE_CONFIRMED": true, "CUSTODY_VERIFIED": true}
	if !validCode[code] || (state != "UNKNOWN" && state != "REJECTED" && state != "VERIFIED") || (state == "VERIFIED" && (version == "" || version == "null" || len(version) > 1024 || strings.ContainsAny(version, "\r\n"))) || (state != "VERIFIED" && version != "") {
		return x, ErrInvalid
	}
	tx, err := recoveryWriter(ctx, s)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	x, err = seriesIntent(ctx, tx, id)
	if err != nil {
		return x, err
	}
	if x.AttemptKey != key {
		return x, ErrConflict
	}
	if x.State == state {
		if x.Version != version {
			return x, ErrConflict
		}
		return x, tx.Commit()
	}
	if x.State != "HANDOFF" && !(x.State == "UNKNOWN" && state == "VERIFIED") {
		return x, ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO recovery_series_events SELECT COALESCE(MAX(sequence),0)+1,?,?,?,?,?,?,? FROM recovery_series_events`, id, state, x.Attempts, key, version, code, time.Now().Unix())
	if err != nil {
		return x, err
	}
	x, err = seriesIntent(ctx, tx, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
