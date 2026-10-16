package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var recoverySeriesPrefix = regexp.MustCompile(`^[A-Z0-9][A-Z0-9/-]{1,38}[A-Z0-9]$`)
var recoveryStoragePrefix = regexp.MustCompile(`^[A-Za-z0-9_/-]+(?:\.[A-Za-z0-9.]+)?$`)

type RecoveryNumberingSpec struct {
	Prefix       string `json:"prefix"`
	Digits       int    `json:"digits"`
	First        int    `json:"first"`
	Last         int    `json:"last"`
	Period       string `json:"period"`
	AuthorityRef string `json:"authority_ref"`
	NamespaceRef string `json:"namespace_ref"`
	VerifiedAt   int64  `json:"verified_at"`
}

func (x RecoveryNumberingSpec) Example(sequence int) string {
	return fmt.Sprintf("%s-%0*d", x.Prefix, x.Digits, sequence)
}

type RecoveryNumberingInput struct {
	OperationKey       string                `json:"operation_key"`
	ScopeKey           string                `json:"scope_key"`
	PredecessorID      string                `json:"predecessor_id"`
	PredecessorVersion int                   `json:"predecessor_version"`
	Title              string                `json:"title"`
	Reason             string                `json:"reason"`
	Policy             RecoveryNumberingSpec `json:"policy"`
}

type RecoverySeriesProgress struct {
	OperationID      string `json:"operation_id"`
	State            string `json:"state"`
	OriginalState    string `json:"original_state"`
	Attempts         int    `json:"attempts"`
	UpdatedAt        int64  `json:"updated_at"`
	ChecksumVerified bool   `json:"checksum_verified"`
}

type RecoveryNumberingPolicy struct {
	ID            string                  `json:"id"`
	Title         string                  `json:"title"`
	State         string                  `json:"state"`
	Version       int                     `json:"version"`
	AuthorID      string                  `json:"author_id"`
	Author        string                  `json:"author"`
	CreatedAt     int64                   `json:"created_at"`
	PredecessorID string                  `json:"predecessor_id"`
	FirstExample  string                  `json:"first_example"`
	LastExample   string                  `json:"last_example"`
	Input         RecoveryNumberingInput  `json:"input"`
	Decision      *RecoveryDecision       `json:"decision"`
	Reservation   *RecoverySeriesProgress `json:"reservation"`
	nomination    string
	epoch         string
	identity      string
}

type RecoveryNumberingPage struct {
	Items    []RecoveryNumberingPolicy `json:"items"`
	Total    int                       `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type RecoveryNumberingStatus struct {
	Pending         int    `json:"pending"`
	Approved        int    `json:"approved"`
	Verified        int    `json:"verified"`
	Unresolved      int    `json:"unresolved"`
	StatusKey       string `json:"status_key"`
	CanResume       bool   `json:"can_resume"`
	declined, stale int
}

const numberingStateSQL = `CASE WHEN p.nomination_id!=? THEN 'STALE' WHEN EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id) THEN 'SUPERSEDED' ELSE COALESCE(d.decision,'PENDING') END`
const numberingSelect = `SELECT p.id,` + numberingStateSQL + `,p.version+CASE WHEN d.policy_id IS NULL THEN 0 ELSE 1 END,p.author_id,u.display_name,p.created_at,COALESCE(p.predecessor,''),p.input_json,p.nomination_id,p.epoch_id,p.author_identity,COALESCE(du.display_name,''),COALESCE(d.decision,''),COALESCE(d.reason,''),COALESCE(d.attestation,''),COALESCE(d.created_at,0),COALESCE(i.operation_id,''),COALESCE(e.state,'PREPARED'),COALESCE(e.attempt,0),COALESCE(e.created_at,i.created_at,0)
 FROM recovery_numbering_policies p JOIN users u ON u.id=p.author_id
 LEFT JOIN recovery_numbering_decisions d ON d.policy_id=p.id LEFT JOIN users du ON du.id=d.actor_id
 LEFT JOIN recovery_series_intents i ON i.policy_id=p.id
 LEFT JOIN recovery_series_events e ON e.sequence=(SELECT MAX(sequence) FROM recovery_series_events WHERE intent_id=i.operation_id)`

func scanNumbering(row interface{ Scan(...any) error }) (RecoveryNumberingPolicy, error) {
	var x RecoveryNumberingPolicy
	var body string
	var decision RecoveryDecision
	var progress RecoverySeriesProgress
	err := row.Scan(&x.ID, &x.State, &x.Version, &x.AuthorID, &x.Author, &x.CreatedAt, &x.PredecessorID, &body, &x.nomination, &x.epoch, &x.identity, &decision.Actor, &decision.Decision, &decision.Reason, &decision.Attestation, &decision.At, &progress.OperationID, &progress.OriginalState, &progress.Attempts, &progress.UpdatedAt)
	if err != nil {
		return x, err
	}
	if err = json.Unmarshal([]byte(body), &x.Input); err != nil {
		return x, err
	}
	x.Title, x.FirstExample, x.LastExample = x.Input.Title, x.Input.Policy.Example(x.Input.Policy.First), x.Input.Policy.Example(x.Input.Policy.Last)
	if decision.Decision != "" {
		x.Decision = &decision
	}
	if progress.OperationID != "" {
		progress.State = progress.OriginalState
		if x.State == "STALE" || x.State == "SUPERSEDED" {
			progress.State = "STALE"
		}
		progress.ChecksumVerified = progress.State == "VERIFIED"
		x.Reservation = &progress
	}
	return x, nil
}

func numberingPolicy(ctx context.Context, q identityReader, p Principal, id string) (RecoveryNumberingPolicy, error) {
	return scanNumbering(q.QueryRowContext(ctx, numberingSelect+" WHERE p.id=? AND p.epoch_id=?", p.recoveryNomination, id, p.recoveryEpoch))
}

func validateNumbering(in RecoveryNumberingInput, retry bool) error {
	x := in.Policy
	maximum := 1
	for i := 0; i < x.Digits && i < 10; i++ {
		maximum *= 10
	}
	if !operationPattern.MatchString(in.OperationKey) || !custodyDigest.MatchString(in.ScopeKey) || !validText(in.Title, 5, 120) || !paragraph(in.Reason, 10, 1000) ||
		(in.PredecessorID == "" && in.PredecessorVersion != 0) || (in.PredecessorID != "" && (len(in.PredecessorID) != 43 || in.PredecessorVersion < 1)) ||
		!recoverySeriesPrefix.MatchString(x.Prefix) || x.Digits < 3 || x.Digits > 9 || x.First < 1 || x.Last < x.First || x.Last >= maximum ||
		!validText(x.Period, 3, 120) || !validText(x.AuthorityRef, 5, 300) || !sourceIdentity.MatchString(x.NamespaceRef) ||
		x.VerifiedAt <= 0 || (!retry && (x.VerifiedAt > time.Now().Unix() || x.VerifiedAt < time.Now().Add(-30*24*time.Hour).Unix())) {
		return invalid("Supply a bounded prefix, 3–9 digits, exact sequence range, period, current policy evidence and independently verified namespace reference.")
	}
	return nil
}

func recoveryOperationUsed(ctx context.Context, q identityReader, key string) (bool, error) {
	var used bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM recovery_proposals WHERE operation_key=? UNION ALL SELECT 1 FROM recovery_decisions WHERE operation_key=? UNION ALL SELECT 1 FROM recovery_numbering_policies WHERE operation_key=? UNION ALL SELECT 1 FROM recovery_numbering_decisions WHERE operation_key=? UNION ALL SELECT 1 FROM recovery_gap_preparations WHERE operation_key=? UNION ALL SELECT 1 FROM recovery_gap_decisions WHERE operation_key=?)`, key, key, key, key, key, key).Scan(&used)
	return used, err
}

func (s *Store) ProposeRecoveryNumbering(ctx context.Context, token string, in RecoveryNumberingInput) (RecoveryNumberingPolicy, error) {
	var x RecoveryNumberingPolicy
	if err := validateNumbering(in, true); err != nil {
		return x, err
	}
	tx, p, err := s.beginRecoveryWrite(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if in.ScopeKey != p.ScopeKey {
		return x, ErrConflict
	}
	body, _ := json.Marshal(in)
	digest := InputDigest(body)
	var id, saved, author string
	err = tx.QueryRowContext(ctx, "SELECT id,input_digest,author_id FROM recovery_numbering_policies WHERE operation_key=?", in.OperationKey).Scan(&id, &saved, &author)
	if err == nil {
		if saved != digest || author != p.ID {
			return x, ErrConflict
		}
		x, err = numberingPolicy(ctx, tx, p, id)
		if err != nil {
			return x, err
		}
		return x, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, err
	}
	if used, e := recoveryOperationUsed(ctx, tx, in.OperationKey); e != nil || used {
		if e != nil {
			return x, e
		}
		return x, ErrConflict
	}
	if err = validateNumbering(in, false); err != nil {
		return x, err
	}
	var attempted bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM recovery_series_intents WHERE series_prefix=?)", in.Policy.Prefix).Scan(&attempted); err != nil {
		return x, err
	}
	if attempted {
		return x, invalid("This prefix already has a retained reservation intent. Supply a new prefix; the earlier handoff remains intact.")
	}
	version := 1
	var predecessor any
	if in.PredecessorID != "" {
		old, e := numberingPolicy(ctx, tx, p, in.PredecessorID)
		if errors.Is(e, sql.ErrNoRows) {
			e = ErrConflict
		}
		if e != nil {
			return x, e
		}
		if old.Version != in.PredecessorVersion || old.State == "PENDING" || old.State == "SUPERSEDED" {
			return x, ErrConflict
		}
		predecessor, version = old.ID, old.Version+1
	}
	id = randomToken()
	_, err = tx.ExecContext(ctx, `INSERT INTO recovery_numbering_policies SELECT COALESCE(MAX(sequence),0)+1,?,?,?,?,?,?,?,?,?,?,?,? FROM recovery_numbering_policies`, id, in.OperationKey, p.recoveryEpoch, p.recoveryNomination, p.ID, p.recoveryIdentity, version, predecessor, digest, string(body), in.Policy.Prefix, time.Now().Unix())
	if err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, p.ID, "RECOVERY_NUMBERING_PREPARED", in.Reason, map[string]any{"policy_id": id, "epoch_id": p.recoveryEpoch, "input_digest": digest, "prepared_only": true}); err != nil {
		return x, err
	}
	x, err = numberingPolicy(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) DecideRecoveryNumbering(ctx context.Context, token, id string, in RecoveryDecisionInput) (RecoveryNumberingPolicy, error) {
	var x RecoveryNumberingPolicy
	if len(id) != 43 || !operationPattern.MatchString(in.OperationKey) || !custodyDigest.MatchString(in.ScopeKey) || in.Version < 1 || !in.Confirmed || (in.Decision != "APPROVED" && in.Decision != "DECLINED") || !paragraph(in.Reason, 10, 1000) || !validText(in.Attestation, 10, 300) {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryWrite(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if in.ScopeKey != p.ScopeKey {
		return x, ErrConflict
	}
	body, _ := json.Marshal(struct {
		ID    string
		Input RecoveryDecisionInput
	}{id, in})
	digest := InputDigest(body)
	var savedID, author, saved string
	err = tx.QueryRowContext(ctx, "SELECT policy_id,actor_id,input_digest FROM recovery_numbering_decisions WHERE operation_key=?", in.OperationKey).Scan(&savedID, &author, &saved)
	if err == nil {
		if savedID != id || author != p.ID || saved != digest {
			return x, ErrConflict
		}
		x, err = numberingPolicy(ctx, tx, p, id)
		if err != nil {
			return x, err
		}
		return x, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, err
	}
	if used, e := recoveryOperationUsed(ctx, tx, in.OperationKey); e != nil || used {
		if e != nil {
			return x, e
		}
		return x, ErrConflict
	}
	x, err = numberingPolicy(ctx, tx, p, id)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrConflict
	}
	if err != nil {
		return x, err
	}
	if x.AuthorID == p.ID || x.identity == p.recoveryIdentity {
		return x, ErrForbidden
	}
	if x.Version != in.Version || x.State != "PENDING" || x.nomination != p.recoveryNomination {
		return x, ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_numbering_decisions VALUES(?,?,?,?,?,?,?,?,?,?)", id, in.OperationKey, p.ID, p.recoveryIdentity, in.Version, in.Decision, in.Reason, in.Attestation, digest, time.Now().Unix()); err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, x.AuthorID, "RECOVERY_NUMBERING_"+in.Decision, in.Reason, map[string]any{"policy_id": id, "input_digest": digest, "prepared_only": true}); err != nil {
		return x, err
	}
	x, err = numberingPolicy(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryNumberingPolicyFor(ctx context.Context, token, id string) (RecoveryNumberingPolicy, error) {
	if len(id) != 43 {
		return RecoveryNumberingPolicy{}, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryNumberingPolicy{}, err
	}
	defer tx.Rollback()
	x, err := numberingPolicy(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryNumberingOperationFor(ctx context.Context, token, key string) (RecoveryNumberingPolicy, error) {
	if !operationPattern.MatchString(key) {
		return RecoveryNumberingPolicy{}, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryNumberingPolicy{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM recovery_numbering_policies WHERE operation_key=? AND author_id=? AND epoch_id=? UNION ALL SELECT d.policy_id FROM recovery_numbering_decisions d JOIN recovery_numbering_policies p ON p.id=d.policy_id WHERE d.operation_key=? AND d.actor_id=? AND p.epoch_id=?`, key, p.ID, p.recoveryEpoch, key, p.ID, p.recoveryEpoch).Scan(&id)
	if err != nil {
		return RecoveryNumberingPolicy{}, err
	}
	x, err := numberingPolicy(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryNumberingPoliciesFor(ctx context.Context, token, state string, page int) (RecoveryNumberingPage, error) {
	x := RecoveryNumberingPage{Items: []RecoveryNumberingPolicy{}, Page: page, PageSize: 12}
	if !recoveryPage(page) || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "STALE" && state != "SUPERSEDED") {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	where := ` WHERE p.epoch_id=? AND (?='' OR ` + numberingStateSQL + `=?)`
	args := []any{p.recoveryEpoch, state, p.recoveryNomination, state}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_numbering_policies p LEFT JOIN recovery_numbering_decisions d ON d.policy_id=p.id"+where, args...).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, numberingSelect+where+" ORDER BY p.sequence DESC LIMIT 12 OFFSET ?", append([]any{p.recoveryNomination}, append(args, (page-1)*12)...)...)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanNumbering(rows)
		if e != nil {
			rows.Close()
			return x, e
		}
		x.Items = append(x.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryNumberingHistoryFor(ctx context.Context, token, id string, page int) (RecoveryNumberingPage, error) {
	x := RecoveryNumberingPage{Items: []RecoveryNumberingPolicy{}, Page: page, PageSize: 12}
	if len(id) != 43 || !recoveryPage(page) {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if _, err = numberingPolicy(ctx, tx, p, id); err != nil {
		return x, err
	}
	chain := `WITH RECURSIVE chain(id,predecessor) AS(SELECT id,predecessor FROM recovery_numbering_policies WHERE id=? AND epoch_id=? UNION ALL SELECT p.id,p.predecessor FROM recovery_numbering_policies p JOIN chain c ON p.id=c.predecessor) `
	if err = tx.QueryRowContext(ctx, chain+"SELECT COUNT(*) FROM chain", id, p.recoveryEpoch).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, chain+numberingSelect+" JOIN chain c ON c.id=p.id ORDER BY p.version DESC LIMIT 12 OFFSET ?", id, p.recoveryEpoch, p.recoveryNomination, (page-1)*12)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanNumbering(rows)
		if e != nil {
			rows.Close()
			return x, e
		}
		x.Items = append(x.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func numberingStatus(ctx context.Context, q identityReader, p Principal) (RecoveryNumberingStatus, error) {
	var x RecoveryNumberingStatus
	var lastEvent int64
	current := `p.nomination_id=? AND NOT EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id)`
	err := q.QueryRowContext(ctx, `SELECT
 COALESCE(SUM(CASE WHEN `+current+` AND d.policy_id IS NULL THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND d.decision='APPROVED' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND e.state='VERIFIED' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND e.state IN('HANDOFF','UNKNOWN') THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND d.decision='DECLINED' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(p.nomination_id!=? OR EXISTS(SELECT 1 FROM recovery_numbering_policies WHERE predecessor=p.id)),0),
 COALESCE(MAX(CASE WHEN p.nomination_id=? THEN e.sequence ELSE 0 END),0)
 FROM recovery_numbering_policies p LEFT JOIN recovery_numbering_decisions d ON d.policy_id=p.id LEFT JOIN recovery_series_intents i ON i.policy_id=p.id
 LEFT JOIN recovery_series_events e ON e.sequence=(SELECT MAX(sequence) FROM recovery_series_events WHERE intent_id=i.operation_id)
 WHERE p.epoch_id=?`, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryEpoch).Scan(&x.Pending, &x.Approved, &x.Verified, &x.Unresolved, &x.declined, &x.stale, &lastEvent)
	if err != nil {
		return x, err
	}
	body, _ := json.Marshal([]any{p.ScopeKey, x.Pending, x.Approved, x.Verified, x.Unresolved, x.declined, x.stale, lastEvent})
	x.StatusKey = InputDigest(body)
	return x, nil
}

func (s *Store) RecoveryNumberingStatusFor(ctx context.Context, token string) (RecoveryNumberingStatus, error) {
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryNumberingStatus{}, err
	}
	defer tx.Rollback()
	x, err := numberingStatus(ctx, tx, p)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
