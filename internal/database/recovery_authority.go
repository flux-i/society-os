package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrRecoveryAmbiguous = errors.New("the current recovery origin is ambiguous; retain this host and perform a new matching verified restore")

type RecoveryAttestation struct {
	IdentityRef string `json:"identity_ref"`
	EvidenceRef string `json:"evidence_ref"`
	VerifiedAt  int64  `json:"verified_at"`
}
type RecoveryNominee struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	AccountID   string `json:"account_id"`
	IdentityRef string `json:"identity_ref"`
	EvidenceRef string `json:"evidence_ref"`
	VerifiedAt  int64  `json:"verified_at"`
}
type RecoveryNomination struct {
	FormatVersion       int                   `json:"format_version"`
	OperationID         string                `json:"operation_id"`
	EpochID             string                `json:"epoch_id"`
	SourceSHA256        string                `json:"source_sha256"`
	ReplacesOperationID string                `json:"replaces_operation_id"`
	ValidFrom           int64                 `json:"valid_from"`
	ValidUntil          int64                 `json:"valid_until"`
	Custodians          []RecoveryAttestation `json:"custodians"`
	Reviewers           []RecoveryNominee     `json:"reviewers"`
}
type RecoveryPassword struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type RecoveryPasswords struct {
	Passwords []RecoveryPassword `json:"passwords"`
}
type RecoveryRevocation struct {
	FormatVersion int                   `json:"format_version"`
	OperationID   string                `json:"operation_id"`
	EpochID       string                `json:"epoch_id"`
	SourceSHA256  string                `json:"source_sha256"`
	NominationID  string                `json:"nomination_id"`
	Reason        string                `json:"reason"`
	Custodians    []RecoveryAttestation `json:"custodians"`
}
type RecoveryNominationResult struct {
	OperationID string `json:"operation_id"`
	Reviewers   int    `json:"reviewers"`
	ValidUntil  int64  `json:"valid_until"`
	Status      string `json:"status"`
}

func recoveryVerification(identity, evidence string, at int64, now time.Time) bool {
	return validText(identity, 3, 120) && strings.TrimSpace(identity) == identity && validText(evidence, 5, 300) && at > 0 && at <= now.Unix() && at >= now.Add(-30*24*time.Hour).Unix()
}
func recoveryCustodians(items []RecoveryAttestation, now time.Time) bool {
	return len(items) == 2 && !strings.EqualFold(items[0].IdentityRef, items[1].IdentityRef) &&
		recoveryVerification(items[0].IdentityRef, items[0].EvidenceRef, items[0].VerifiedAt, now) &&
		recoveryVerification(items[1].IdentityRef, items[1].EvidenceRef, items[1].VerifiedAt, now)
}
func (in RecoveryNomination) validate(now time.Time, retry bool) error {
	if in.FormatVersion != 1 || !sourceIdentity.MatchString(in.OperationID) || len(in.EpochID) != 43 || !custodyDigest.MatchString(in.SourceSHA256) ||
		(in.ReplacesOperationID != "" && (!sourceIdentity.MatchString(in.ReplacesOperationID) || in.ReplacesOperationID == in.OperationID)) ||
		in.ValidFrom <= 0 || in.ValidUntil <= in.ValidFrom || in.ValidUntil-in.ValidFrom > 30*86400 || len(in.Reviewers) < 2 || len(in.Reviewers) > 8 {
		return invalid("Supply a version-1 current-source nomination, distinct operation identity, bounded term and two to eight reviewers.")
	}
	// An exact expired retry can read the already retained receipt, but never
	// grants access. New appointments must have current supplied verification.
	if !retry && (in.ValidFrom > now.Unix() || in.ValidFrom < now.Add(-5*time.Minute).Unix() || in.ValidUntil <= now.Unix() || !recoveryCustodians(in.Custodians, now)) {
		return invalid("Supply current distinct custodian attestations and an appointment that is active now.")
	}
	people, emails, accounts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range in.Reviewers {
		email, err := normalizeEmail(r.Email)
		if err != nil || email != r.Email || !validText(r.Name, 2, 120) || people[strings.ToLower(r.IdentityRef)] || emails[email] ||
			(r.AccountID != "" && (len(r.AccountID) > 100 || accounts[r.AccountID])) ||
			(!retry && !recoveryVerification(r.IdentityRef, r.EvidenceRef, r.VerifiedAt, now)) {
			return invalid("Supply separately verified people with normalized distinct emails and exact existing account identities where applicable.")
		}
		people[strings.ToLower(r.IdentityRef)], emails[email] = true, true
		if r.AccountID != "" {
			accounts[r.AccountID] = true
		}
	}
	return nil
}
func ParseRecoveryNomination(body []byte) (RecoveryNomination, error) {
	var in RecoveryNomination
	if len(body) > 32768 || strictJSON(body, &in) != nil {
		return in, invalid("Use a bounded strict version-1 nomination JSON file.")
	}
	return in, in.validate(time.Now(), true)
}
func ParseRecoveryPasswords(body []byte) (RecoveryPasswords, error) {
	var in RecoveryPasswords
	if len(body) > 8192 || strictJSON(body, &in) != nil || len(in.Passwords) > 8 {
		return in, ErrInvalid
	}
	seen := map[string]bool{}
	for _, p := range in.Passwords {
		email, err := normalizeEmail(p.Email)
		if err != nil || email != p.Email || seen[email] || !validPassword(p.Password) {
			return in, ErrInvalid
		}
		seen[email] = true
	}
	return in, nil
}
func ParseRecoveryRevocation(body []byte) (RecoveryRevocation, error) {
	var in RecoveryRevocation
	if len(body) > 8192 || strictJSON(body, &in) != nil || in.FormatVersion != 1 || !sourceIdentity.MatchString(in.OperationID) ||
		!sourceIdentity.MatchString(in.NominationID) || in.OperationID == in.NominationID || len(in.EpochID) != 43 || !custodyDigest.MatchString(in.SourceSHA256) || !validText(in.Reason, 10, 300) {
		return in, ErrInvalid
	}
	return in, nil
}

func currentRecoveryEpoch(ctx context.Context, q queryer) (RecoveryOrigin, error) {
	var x RecoveryOrigin
	err := q.QueryRowContext(ctx, `SELECT e.id,e.format_version,e.source_sha256,e.source_bytes,e.source_schema,e.source_application,e.source_created_at,e.restored_at
	 FROM recovery_epoch_heads h JOIN recovery_epochs e ON e.id=h.epoch_id ORDER BY h.sequence DESC LIMIT 1`).Scan(&x.ID, &x.FormatVersion, &x.SourceSHA256, &x.SourceBytes, &x.SourceSchema, &x.SourceApplication, &x.SourceCreatedAt, &x.RestoredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrRecoveryAmbiguous
	}
	return x, err
}
func (s *Store) RecoveryOriginForCustody(ctx context.Context) (RecoveryOrigin, error) {
	if err := s.VerifySchema(ctx); err != nil {
		return RecoveryOrigin{}, err
	}
	if required, err := s.RecoveryRequired(ctx); err != nil || !required {
		if err == nil {
			err = ErrRecoveryRequired
		}
		return RecoveryOrigin{}, err
	}
	return currentRecoveryEpoch(ctx, s.DB)
}
func recoveryWriter(ctx context.Context, s *Store) (*sql.Tx, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Reserve before any reads, including source/appointment checks (REC-01).
	if _, err = tx.ExecContext(ctx, "UPDATE users SET auth_version=auth_version WHERE 0"); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}
func bindRecoverySource(ctx context.Context, q queryer, epoch, digest string) error {
	x, err := currentRecoveryEpoch(ctx, q)
	if err != nil {
		return err
	}
	if x.ID != epoch || x.SourceSHA256 != digest {
		return ErrConflict
	}
	return nil
}

// Explicit host-custodian input, never a serving route. It records supplied
// attestations; it does not independently prove that a person holds authority.
func (s *Store) NominateRecovery(ctx context.Context, in RecoveryNomination, passwords RecoveryPasswords) (RecoveryNominationResult, error) {
	result := RecoveryNominationResult{in.OperationID, len(in.Reviewers), in.ValidUntil, "reviewers_nominated_private_access_withheld"}
	if err := in.validate(time.Now(), true); err != nil {
		return result, err
	}
	if err := s.RequireRecoverableWorkspace(ctx); err != nil {
		return result, err
	}
	secrets, hashes := map[string]string{}, map[string]string{}
	for _, p := range passwords.Passwords {
		if secrets[p.Email] != "" || !validPassword(p.Password) {
			return result, ErrInvalid
		}
		secrets[p.Email] = p.Password
	}
	newCount := 0
	for _, r := range in.Reviewers {
		if r.AccountID == "" {
			newCount++
			if secrets[r.Email] == "" {
				return result, invalid("Every new reviewer needs an explicit separately held password.")
			}
			hashes[r.Email] = HashPassword(secrets[r.Email])
		}
	}
	if newCount != len(secrets) {
		return result, invalid("Passwords apply only to the explicitly supplied new reviewers.")
	}
	body, _ := json.Marshal(in)
	digest := InputDigest(body)
	tx, err := recoveryWriter(ctx, s)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = bindRecoverySource(ctx, tx, in.EpochID, in.SourceSHA256); err != nil {
		return result, err
	}
	var old string
	err = tx.QueryRowContext(ctx, "SELECT input_digest FROM recovery_nominations WHERE operation_id=?", in.OperationID).Scan(&old)
	if err == nil {
		if old != digest {
			return result, ErrConflict
		}
		for _, r := range in.Reviewers {
			if r.AccountID != "" {
				continue
			}
			var encoded string
			if err = tx.QueryRowContext(ctx, `SELECT rr.initial_password_hash FROM recovery_reviewers rr JOIN users u ON u.id=rr.user_id WHERE rr.nomination_id=? AND u.login=?`, in.OperationID, r.Email).Scan(&encoded); err != nil {
				return result, err
			}
			if !verifyPassword(secrets[r.Email], encoded) {
				return result, ErrConflict
			}
		}
		return result, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if err = in.validate(time.Now(), false); err != nil {
		return result, err
	}
	var reused bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE operation_id=?)", in.OperationID).Scan(&reused); err != nil {
		return result, err
	}
	if reused {
		return result, ErrConflict
	}
	var predecessor string
	err = tx.QueryRowContext(ctx, "SELECT operation_id FROM recovery_nominations WHERE epoch_id=? ORDER BY sequence DESC LIMIT 1", in.EpochID).Scan(&predecessor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if in.ReplacesOperationID != predecessor {
		return result, ErrConflict
	}
	ids := make([]string, len(in.Reviewers))
	for i, r := range in.Reviewers {
		var id, status string
		var suspended sql.NullInt64
		var verified sql.NullInt64
		err = tx.QueryRowContext(ctx, "SELECT id,status,suspended_at,verified_at FROM users WHERE login=?", r.Email).Scan(&id, &status, &suspended, &verified)
		if r.AccountID != "" {
			if errors.Is(err, sql.ErrNoRows) || (err == nil && (id != r.AccountID || status != "ACTIVE" || suspended.Valid || !verified.Valid)) {
				return result, ErrConflict
			}
			if err != nil {
				return result, err
			}
			ids[i] = id
		} else {
			if err == nil {
				return result, ErrConflict
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return result, err
			}
			ids[i] = "recovery-reviewer-" + randomToken()
			if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,login,display_name,password_hash,status,created_at,verified_at,password_changed_at,is_demo) VALUES(?,?,?,?,'ACTIVE',?,?,?,0)`, ids[i], r.Email, r.Name, hashes[r.Email], time.Now().Unix(), r.VerifiedAt, time.Now().Unix()); err != nil {
				return result, err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO recovery_nominations SELECT COALESCE(MAX(sequence),0)+1,?,?,?,?,?,?,?,?,? FROM recovery_nominations`, in.OperationID, in.EpochID, in.SourceSHA256, in.ReplacesOperationID, digest, string(body), in.ValidFrom, in.ValidUntil, time.Now().Unix()); err != nil {
		return result, err
	}
	for i, r := range in.Reviewers {
		if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_reviewers VALUES(?,?,?,?,?,?)", in.OperationID, ids[i], r.IdentityRef, r.EvidenceRef, r.VerifiedAt, hashes[r.Email]); err != nil {
			return result, err
		}
	}
	// One accepted operation revokes only the new and replaced nominee sets.
	if err = invalidateRecoverySessions(ctx, tx, in.OperationID, predecessor); err != nil {
		return result, err
	}
	// The immutable nomination itself is the offline audit, with both supplied
	// custodians. Never fabricate a signed-in user as this operation's actor.
	return result, tx.Commit()
}
func invalidateRecoverySessions(ctx context.Context, tx *sql.Tx, current, previous string) error {
	for _, query := range []string{
		`DELETE FROM sessions WHERE user_id IN(SELECT user_id FROM recovery_reviewers WHERE nomination_id IN(?,?))`,
		`DELETE FROM mfa_pending WHERE user_id IN(SELECT user_id FROM recovery_reviewers WHERE nomination_id IN(?,?))`,
		`UPDATE users SET auth_version=auth_version+1 WHERE id IN(SELECT user_id FROM recovery_reviewers WHERE nomination_id IN(?,?))`,
		`UPDATE account_tokens SET revoked_at=unixepoch('now') WHERE user_id IN(SELECT user_id FROM recovery_reviewers WHERE nomination_id IN(?,?)) AND consumed_at IS NULL AND revoked_at IS NULL`,
	} {
		if _, err := tx.ExecContext(ctx, query, current, previous); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) RevokeRecoveryNomination(ctx context.Context, in RecoveryRevocation) error {
	body, _ := json.Marshal(in)
	if _, err := ParseRecoveryRevocation(body); err != nil {
		return err
	}
	if err := s.RequireRecoverableWorkspace(ctx); err != nil {
		return err
	}
	digest := InputDigest(body)
	tx, err := recoveryWriter(ctx, s)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = bindRecoverySource(ctx, tx, in.EpochID, in.SourceSHA256); err != nil {
		return err
	}
	var previous string
	err = tx.QueryRowContext(ctx, "SELECT input_digest FROM recovery_nomination_revocations WHERE operation_id=?", in.OperationID).Scan(&previous)
	if err == nil {
		if previous != digest {
			return ErrConflict
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if !recoveryCustodians(in.Custodians, time.Now()) {
		return ErrInvalid
	}
	var latest string
	if err = tx.QueryRowContext(ctx, "SELECT operation_id FROM recovery_nominations WHERE epoch_id=? ORDER BY sequence DESC LIMIT 1", in.EpochID).Scan(&latest); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	if latest != in.NominationID {
		return ErrConflict
	}
	var conflict bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM recovery_nominations WHERE operation_id=?) OR EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=?)`, in.OperationID, in.NominationID).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_nomination_revocations VALUES(?,?,?,?,?)", in.OperationID, in.NominationID, digest, string(body), time.Now().Unix()); err != nil {
		return err
	}
	if err = invalidateRecoverySessions(ctx, tx, in.NominationID, ""); err != nil {
		return err
	}
	// The append-only revocation retains its supplied custodians and reason.
	return tx.Commit()
}

func recoveryReviewer(ctx context.Context, q queryer, p *Principal, now time.Time) error {
	epoch, err := currentRecoveryEpoch(ctx, q)
	if errors.Is(err, ErrRecoveryAmbiguous) {
		return nil
	}
	if err != nil {
		return err
	}
	// Include the epoch even for a non-nominee: held results from an earlier
	// restore cannot remain attached to this new authority boundary.
	p.ScopeKey = TokenHash(p.ID + ":recovery:" + epoch.ID)
	var nomination, person string
	err = q.QueryRowContext(ctx, `SELECT n.operation_id,rr.identity_ref FROM recovery_nominations n JOIN recovery_reviewers rr ON rr.nomination_id=n.operation_id
	 WHERE n.epoch_id=? AND n.sequence=(SELECT MAX(sequence) FROM recovery_nominations WHERE epoch_id=?) AND rr.user_id=?
	 AND n.valid_from<=? AND n.valid_until>? AND NOT EXISTS(SELECT 1 FROM recovery_nomination_revocations WHERE nomination_id=n.operation_id)`, epoch.ID, epoch.ID, p.ID, now.Unix(), now.Unix()).Scan(&nomination, &person)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	p.recoveryEpoch, p.recoveryNomination, p.recoveryIdentity = epoch.ID, nomination, person
	p.RecoveryReviewer, p.MFARequired = true, true
	p.MFAPending = !p.MFAEnrolled || p.factorAt == 0
	p.Fresh = !p.MFAPending && p.passwordAt > now.Add(-5*time.Minute).Unix() && p.factorAt > now.Add(-5*time.Minute).Unix()
	p.CanReviewRecovery = !p.MFAPending
	p.ScopeKey = TokenHash(p.ID + ":recovery:" + epoch.ID + ":" + nomination + ":" + person)
	return nil
}
