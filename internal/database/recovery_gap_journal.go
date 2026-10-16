package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const gapStateSQL = `CASE WHEN p.nomination_id!=? THEN 'STALE' WHEN EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=p.id) THEN 'SUPERSEDED' ELSE COALESCE(d.decision,'PENDING') END`
const gapSelect = `SELECT p.id,` + gapStateSQL + `,p.version+CASE WHEN d.preparation_id IS NULL THEN 0 ELSE 1 END,p.author_id,u.display_name,p.created_at,COALESCE(p.predecessor,''),p.input_json,p.comparison_json,p.nomination_id,p.epoch_id,p.author_identity,COALESCE(du.display_name,''),COALESCE(d.decision,''),COALESCE(d.reason,''),COALESCE(d.attestation,''),COALESCE(d.created_at,0)
 FROM recovery_gap_preparations p JOIN users u ON u.id=p.author_id
 LEFT JOIN recovery_gap_decisions d ON d.preparation_id=p.id LEFT JOIN users du ON du.id=d.actor_id`

func scanGap(row interface{ Scan(...any) error }) (RecoveryGapPreparation, error) {
	var x RecoveryGapPreparation
	var body, comparison string
	var decision RecoveryDecision
	err := row.Scan(&x.ID, &x.State, &x.Version, &x.AuthorID, &x.Author, &x.CreatedAt, &x.PredecessorID, &body, &comparison, &x.nomination, &x.epoch, &x.identity, &decision.Actor, &decision.Decision, &decision.Reason, &decision.Attestation, &decision.At)
	if err != nil {
		return x, err
	}
	if err = json.Unmarshal([]byte(body), &x.Input); err != nil {
		return x, err
	}
	if err = json.Unmarshal([]byte(comparison), &x.Comparison); err != nil {
		return x, err
	}
	x.Title = x.Input.Title
	if decision.Decision != "" {
		x.Decision = &decision
	}
	return x, nil
}

func gapPreparation(ctx context.Context, q identityReader, p Principal, id string) (RecoveryGapPreparation, error) {
	return scanGap(q.QueryRowContext(ctx, gapSelect+" WHERE p.id=? AND p.epoch_id=?", p.recoveryNomination, id, p.recoveryEpoch))
}

func (s *Store) ProposeRecoveryGap(ctx context.Context, token string, in RecoveryGapInput) (RecoveryGapPreparation, error) {
	var x RecoveryGapPreparation
	if err := validateGap(in, true); err != nil {
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
	err = tx.QueryRowContext(ctx, "SELECT id,input_digest,author_id FROM recovery_gap_preparations WHERE operation_key=?", in.OperationKey).Scan(&id, &saved, &author)
	if err == nil {
		if saved != digest || author != p.ID {
			return x, ErrConflict
		}
		x, err = gapPreparation(ctx, tx, p, id)
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
	if err = validateGap(in, false); err != nil {
		return x, err
	}
	version := 1
	var predecessor any
	if in.PredecessorID != "" {
		old, e := gapPreparation(ctx, tx, p, in.PredecessorID)
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
	comparison, err := gapCompare(ctx, tx, p, in)
	if err != nil {
		return x, err
	}
	snapshot, _ := json.Marshal(comparison)
	id = randomToken()
	_, err = tx.ExecContext(ctx, `INSERT INTO recovery_gap_preparations SELECT COALESCE(MAX(sequence),0)+1,?,?,?,?,?,?,?,?,?,?,?,?,?,? FROM recovery_gap_preparations`, id, in.OperationKey, p.recoveryEpoch, p.recoveryNomination, p.ID, p.recoveryIdentity, version, predecessor, digest, string(body), string(snapshot), comparison.KnownItems, comparison.UnresolvedItems, time.Now().Unix())
	if err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, p.ID, "RECOVERY_FINANCIAL_GAP_PREPARED", in.Reason, map[string]any{"preparation_id": id, "epoch_id": p.recoveryEpoch, "input_digest": digest, "prepared_only": true}); err != nil {
		return x, err
	}
	x, err = gapPreparation(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) DecideRecoveryGap(ctx context.Context, token, id string, in RecoveryDecisionInput) (RecoveryGapPreparation, error) {
	var x RecoveryGapPreparation
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
	err = tx.QueryRowContext(ctx, "SELECT preparation_id,actor_id,input_digest FROM recovery_gap_decisions WHERE operation_key=?", in.OperationKey).Scan(&savedID, &author, &saved)
	if err == nil {
		if savedID != id || author != p.ID || saved != digest {
			return x, ErrConflict
		}
		x, err = gapPreparation(ctx, tx, p, id)
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
	x, err = gapPreparation(ctx, tx, p, id)
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
	if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_gap_decisions VALUES(?,?,?,?,?,?,?,?,?,?)", id, in.OperationKey, p.ID, p.recoveryIdentity, in.Version, in.Decision, in.Reason, in.Attestation, digest, time.Now().Unix()); err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, x.AuthorID, "RECOVERY_FINANCIAL_GAP_"+in.Decision, in.Reason, map[string]any{"preparation_id": id, "input_digest": digest, "prepared_only": true}); err != nil {
		return x, err
	}
	x, err = gapPreparation(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryGapFor(ctx context.Context, token, id string) (RecoveryGapPreparation, error) {
	if len(id) != 43 {
		return RecoveryGapPreparation{}, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryGapPreparation{}, err
	}
	defer tx.Rollback()
	x, err := gapPreparation(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryGapOperationFor(ctx context.Context, token, key string) (RecoveryGapPreparation, error) {
	if !operationPattern.MatchString(key) {
		return RecoveryGapPreparation{}, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryGapPreparation{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM recovery_gap_preparations WHERE operation_key=? AND author_id=? AND epoch_id=? UNION ALL SELECT d.preparation_id FROM recovery_gap_decisions d JOIN recovery_gap_preparations p ON p.id=d.preparation_id WHERE d.operation_key=? AND d.actor_id=? AND p.epoch_id=?`, key, p.ID, p.recoveryEpoch, key, p.ID, p.recoveryEpoch).Scan(&id)
	if err != nil {
		return RecoveryGapPreparation{}, err
	}
	x, err := gapPreparation(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryGapPreparationsFor(ctx context.Context, token, state string, page int) (RecoveryGapPage, error) {
	x := RecoveryGapPage{Items: []RecoveryGapPreparation{}, Page: page, PageSize: 12}
	if !recoveryPage(page) || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "STALE" && state != "SUPERSEDED") {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	where := ` WHERE p.epoch_id=? AND (?='' OR ` + gapStateSQL + `=?)`
	args := []any{p.recoveryEpoch, state, p.recoveryNomination, state}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_gap_preparations p LEFT JOIN recovery_gap_decisions d ON d.preparation_id=p.id"+where, args...).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, gapSelect+where+" ORDER BY p.sequence DESC LIMIT 12 OFFSET ?", append([]any{p.recoveryNomination}, append(args, (page-1)*12)...)...)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanGap(rows)
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

func (s *Store) RecoveryGapHistoryFor(ctx context.Context, token, id string, page int) (RecoveryGapPage, error) {
	x := RecoveryGapPage{Items: []RecoveryGapPreparation{}, Page: page, PageSize: 12}
	if len(id) != 43 || !recoveryPage(page) {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if _, err = gapPreparation(ctx, tx, p, id); err != nil {
		return x, err
	}
	chain := `WITH RECURSIVE chain(id,predecessor) AS(SELECT id,predecessor FROM recovery_gap_preparations WHERE id=? AND epoch_id=? UNION ALL SELECT p.id,p.predecessor FROM recovery_gap_preparations p JOIN chain c ON p.id=c.predecessor) `
	if err = tx.QueryRowContext(ctx, chain+"SELECT COUNT(*) FROM chain", id, p.recoveryEpoch).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, chain+gapSelect+" JOIN chain c ON c.id=p.id ORDER BY p.version DESC LIMIT 12 OFFSET ?", id, p.recoveryEpoch, p.recoveryNomination, (page-1)*12)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanGap(rows)
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

func gapStatus(ctx context.Context, q identityReader, p Principal) (RecoveryGapStatus, error) {
	var x RecoveryGapStatus
	var last int64
	current := `p.nomination_id=? AND NOT EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=p.id)`
	err := q.QueryRowContext(ctx, `SELECT
 COALESCE(SUM(CASE WHEN `+current+` AND d.preparation_id IS NULL THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND d.decision='APPROVED' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` THEN p.known_count ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` THEN p.unresolved_count ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN `+current+` AND d.decision='DECLINED' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(p.nomination_id!=? OR EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=p.id)),0), COALESCE(MAX(p.sequence),0)
 FROM recovery_gap_preparations p LEFT JOIN recovery_gap_decisions d ON d.preparation_id=p.id WHERE p.epoch_id=?`, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryNomination, p.recoveryEpoch).Scan(&x.Pending, &x.Approved, &x.Known, &x.Unresolved, &x.declined, &x.stale, &last)
	if err != nil {
		return x, err
	}
	body, _ := json.Marshal([]any{p.ScopeKey, x.Pending, x.Approved, x.Known, x.Unresolved, x.declined, x.stale, last})
	x.StatusKey = InputDigest(body)
	return x, nil
}

func (s *Store) RecoveryGapStatusFor(ctx context.Context, token string) (RecoveryGapStatus, error) {
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryGapStatus{}, err
	}
	defer tx.Rollback()
	x, err := gapStatus(ctx, tx, p)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
