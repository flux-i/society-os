package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type RecoveryAccess struct {
	AccountID    string `json:"account_id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	IdentityRef  string `json:"identity_ref"`
	Role         string `json:"role"`
	HomeID       string `json:"home_id"`
	Relationship string `json:"relationship"`
	Finance      bool   `json:"finance"`
	ValidFrom    int64  `json:"valid_from"`
	ValidUntil   int64  `json:"valid_until"`
	VerifiedAt   int64  `json:"verified_at"`
	EvidenceRef  string `json:"evidence_ref"`
}
type RecoveryWork struct {
	Kind          string `json:"kind"`
	JobID         string `json:"job_id"`
	OriginalState string `json:"original_state"`
	Outcome       string `json:"outcome"`
	EvidenceRef   string `json:"evidence_ref"`
}
type RecoveryProposalInput struct {
	OperationKey       string          `json:"operation_key"`
	ScopeKey           string          `json:"scope_key"`
	Kind               string          `json:"kind"`
	PredecessorID      string          `json:"predecessor_id"`
	PredecessorVersion int             `json:"predecessor_version"`
	Title              string          `json:"title"`
	Reason             string          `json:"reason"`
	Access             *RecoveryAccess `json:"access"`
	Work               *RecoveryWork   `json:"work"`
}
type RecoveryDecisionInput struct {
	OperationKey string `json:"operation_key"`
	ScopeKey     string `json:"scope_key"`
	Version      int    `json:"version"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	Attestation  string `json:"attestation"`
	Confirmed    bool   `json:"confirmed"`
}
type RecoveryProposal struct {
	ID            string                `json:"id"`
	Kind          string                `json:"kind"`
	Title         string                `json:"title"`
	State         string                `json:"state"`
	Version       int                   `json:"version"`
	AuthorID      string                `json:"author_id"`
	Author        string                `json:"author"`
	CreatedAt     int64                 `json:"created_at"`
	PredecessorID string                `json:"predecessor_id"`
	Input         RecoveryProposalInput `json:"input"`
	Decision      *RecoveryDecision     `json:"decision"`
	nomination    string
	epoch         string
	identity      string
}
type RecoveryDecision struct {
	Actor       string `json:"actor"`
	Decision    string `json:"decision"`
	Reason      string `json:"reason"`
	Attestation string `json:"attestation"`
	At          int64  `json:"at"`
}
type RecoveryProposalPage struct {
	Items    []RecoveryProposal `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}
type RecoverySummary struct {
	Origin           RecoveryOrigin `json:"origin"`
	AppointmentUntil int64          `json:"appointment_until"`
	Pending          int            `json:"pending"`
	Approved         int            `json:"approved"`
	Declined         int            `json:"declined"`
	Stale            int            `json:"stale"`
	RetainedAccounts int            `json:"retained_accounts"`
	RetainedWork     int            `json:"retained_work"`
	StatusKey        string         `json:"status_key"`
	CanResume        bool           `json:"can_resume"`
}
type RecoveryCandidate struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	State string `json:"state"`
}
type RecoveryCandidatePage struct {
	Items    []RecoveryCandidate `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
type RecoveryWorkPage struct {
	Items    []RecoveryWork `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// A missing result never permits changing an uncertain write's retry identity.
func (s *Store) RecoveryOperationFor(ctx context.Context, token, key string) (RecoveryProposal, error) {
	if !operationPattern.MatchString(key) {
		return RecoveryProposal{}, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryProposal{}, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM recovery_proposals WHERE operation_key=? AND author_id=? AND epoch_id=?
	 UNION ALL SELECT d.proposal_id FROM recovery_decisions d JOIN recovery_proposals p ON p.id=d.proposal_id WHERE d.operation_key=? AND d.actor_id=? AND p.epoch_id=?`, key, p.ID, p.recoveryEpoch, key, p.ID, p.recoveryEpoch).Scan(&id)
	if err != nil {
		return RecoveryProposal{}, err
	}
	x, err := recoveryProposal(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}

func (s *Store) RecoveryHistoryFor(ctx context.Context, token, id string, page int) (RecoveryProposalPage, error) {
	x := RecoveryProposalPage{Items: []RecoveryProposal{}, Page: page, PageSize: 12}
	if len(id) != 43 || !recoveryPage(page) {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if _, err = recoveryProposal(ctx, tx, p, id); err != nil {
		return x, err
	}
	chain := `WITH RECURSIVE chain(id,predecessor) AS(SELECT id,predecessor FROM recovery_proposals WHERE id=? AND epoch_id=? UNION ALL SELECT p.id,p.predecessor FROM recovery_proposals p JOIN chain c ON p.id=c.predecessor) `
	if err = tx.QueryRowContext(ctx, chain+"SELECT COUNT(*) FROM chain", id, p.recoveryEpoch).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, chain+recoveryProposalSelect+" JOIN chain c ON c.id=p.id ORDER BY p.version DESC LIMIT 12 OFFSET ?", id, p.recoveryEpoch, p.recoveryNomination, (page-1)*12)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanRecoveryProposal(rows)
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

func recoveryPrincipal(ctx context.Context, q identityReader, hash string, now time.Time) (Principal, error) {
	p, err := principal(ctx, q, hash, now)
	if err != nil {
		return p, err
	}
	if !p.RecoveryRequired || !p.RecoveryReviewer {
		return p, ErrForbidden
	}
	if p.MFAPending {
		return p, ErrMFARequired
	}
	if !p.CanReviewRecovery {
		return p, ErrForbidden
	}
	return p, nil
}
func (s *Store) beginRecoveryRead(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, Principal{}, err
	}
	p, err := recoveryPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func (s *Store) beginRecoveryWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, p, err := s.beginIdentityWrite(ctx, token)
	if err != nil {
		return nil, p, err
	}
	if !p.RecoveryRequired || !p.CanReviewRecovery {
		err = ErrForbidden
	} else {
		err = requireFresh(p)
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func recoveryPage(page int) bool { return page >= 1 && page <= 10000 }

func (s *Store) RecoverySummaryFor(ctx context.Context, token string) (RecoverySummary, error) {
	var x RecoverySummary
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	x.Origin, err = currentRecoveryEpoch(ctx, tx)
	if err != nil {
		return x, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT valid_until FROM recovery_nominations WHERE operation_id=?`, p.recoveryNomination).Scan(&x.AppointmentUntil); err != nil {
		return x, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT
	 (SELECT COUNT(*) FROM recovery_proposals p WHERE p.epoch_id=? AND p.nomination_id=? AND NOT EXISTS(SELECT 1 FROM recovery_decisions WHERE proposal_id=p.id)),
	 (SELECT COUNT(*) FROM recovery_decisions d JOIN recovery_proposals p ON p.id=d.proposal_id WHERE p.epoch_id=? AND p.nomination_id=? AND d.decision='APPROVED'),
	 (SELECT COUNT(*) FROM recovery_decisions d JOIN recovery_proposals p ON p.id=d.proposal_id WHERE p.epoch_id=? AND p.nomination_id=? AND d.decision='DECLINED'),
	 (SELECT COUNT(*) FROM recovery_proposals WHERE epoch_id=? AND nomination_id!=?),
	 (SELECT COUNT(*) FROM users), (SELECT COUNT(*) FROM recovery_external_jobs WHERE epoch_id=?)`, p.recoveryEpoch, p.recoveryNomination, p.recoveryEpoch, p.recoveryNomination, p.recoveryEpoch, p.recoveryNomination, p.recoveryEpoch, p.recoveryNomination, p.recoveryEpoch).Scan(&x.Pending, &x.Approved, &x.Declined, &x.Stale, &x.RetainedAccounts, &x.RetainedWork); err != nil {
		return x, err
	}
	numbering, err := numberingStatus(ctx, tx, p)
	if err != nil {
		return x, err
	}
	x.Pending += numbering.Pending
	x.Approved += numbering.Approved
	x.Declined += numbering.declined
	x.Stale += numbering.stale
	gap, err := gapStatus(ctx, tx, p)
	if err != nil {
		return x, err
	}
	x.Pending += gap.Pending
	x.Approved += gap.Approved
	x.Declined += gap.declined
	x.Stale += gap.stale
	body, _ := json.Marshal([]any{p.ScopeKey, x.Pending, x.Approved, x.Declined, x.Stale, x.RetainedAccounts, x.RetainedWork, numbering.StatusKey, gap.StatusKey})
	x.StatusKey = InputDigest(body)
	return x, tx.Commit()
}
func (s *Store) RecoveryCandidatesFor(ctx context.Context, token, query string, page int) (RecoveryCandidatePage, error) {
	x := RecoveryCandidatePage{Items: []RecoveryCandidate{}, Page: page, PageSize: 12}
	if len(query) > 100 || !recoveryPage(page) {
		return x, ErrInvalid
	}
	tx, _, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	search := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	where := ` WHERE lower(display_name) LIKE ? OR lower(login) LIKE ?`
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users"+where, search, search).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,login,display_name,CASE WHEN suspended_at IS NOT NULL THEN 'SUSPENDED' ELSE status END FROM users"+where+" ORDER BY display_name,id LIMIT 12 OFFSET ?", search, search, (page-1)*12)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		var item RecoveryCandidate
		if err = rows.Scan(&item.ID, &item.Email, &item.Name, &item.State); err != nil {
			rows.Close()
			return x, err
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
func (s *Store) RecoveryWorkFor(ctx context.Context, token string, page int) (RecoveryWorkPage, error) {
	x := RecoveryWorkPage{Items: []RecoveryWork{}, Page: page, PageSize: 12}
	if !recoveryPage(page) {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_external_jobs WHERE epoch_id=?", p.recoveryEpoch).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT job_kind,job_id,original_state FROM recovery_external_jobs WHERE epoch_id=? ORDER BY job_kind,job_id LIMIT 12 OFFSET ?", p.recoveryEpoch, (page-1)*12)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		var item RecoveryWork
		if err = rows.Scan(&item.Kind, &item.JobID, &item.OriginalState); err != nil {
			rows.Close()
			return x, err
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
func (s *Store) RecoveryHomesFor(ctx context.Context, token string) ([]RecordHome, error) {
	tx, _, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id ORDER BY b.code,f.floor,f.flat_number LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	x := []RecordHome{}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			return nil, err
		}
		x = append(x, h)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return x, tx.Commit()
}

const recoveryProposalSelect = `SELECT p.id,p.kind,CASE WHEN p.nomination_id!=? THEN 'STALE' ELSE COALESCE(d.decision,'PENDING') END,p.version+CASE WHEN d.proposal_id IS NULL THEN 0 ELSE 1 END,p.author_id,u.display_name,p.created_at,COALESCE(p.predecessor,''),p.input_json,p.nomination_id,p.epoch_id,p.author_identity,COALESCE(du.display_name,''),COALESCE(d.decision,''),COALESCE(d.reason,''),COALESCE(d.attestation,''),COALESCE(d.created_at,0)
 FROM recovery_proposals p JOIN users u ON u.id=p.author_id LEFT JOIN recovery_decisions d ON d.proposal_id=p.id LEFT JOIN users du ON du.id=d.actor_id`

func scanRecoveryProposal(row interface{ Scan(...any) error }) (RecoveryProposal, error) {
	var x RecoveryProposal
	var body string
	var decision RecoveryDecision
	err := row.Scan(&x.ID, &x.Kind, &x.State, &x.Version, &x.AuthorID, &x.Author, &x.CreatedAt, &x.PredecessorID, &body, &x.nomination, &x.epoch, &x.identity, &decision.Actor, &decision.Decision, &decision.Reason, &decision.Attestation, &decision.At)
	if err != nil {
		return x, err
	}
	if err = json.Unmarshal([]byte(body), &x.Input); err != nil {
		return x, err
	}
	x.Title = x.Input.Title
	if decision.Decision != "" {
		x.Decision = &decision
	}
	return x, nil
}
func recoveryProposal(ctx context.Context, q identityReader, p Principal, id string) (RecoveryProposal, error) {
	return scanRecoveryProposal(q.QueryRowContext(ctx, recoveryProposalSelect+" WHERE p.id=? AND p.epoch_id=?", p.recoveryNomination, id, p.recoveryEpoch))
}
func (s *Store) RecoveryProposalFor(ctx context.Context, token, id string) (RecoveryProposal, error) {
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryProposal{}, err
	}
	defer tx.Rollback()
	x, err := recoveryProposal(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
func (s *Store) RecoveryProposalsFor(ctx context.Context, token, kind, state string, page int) (RecoveryProposalPage, error) {
	x := RecoveryProposalPage{Items: []RecoveryProposal{}, Page: page, PageSize: 12}
	if !recoveryPage(page) || (kind != "" && kind != "ACCESS" && kind != "EXTERNAL") || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "STALE") {
		return x, ErrInvalid
	}
	tx, p, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	where := ` WHERE p.epoch_id=? AND (?='' OR p.kind=?) AND (?='' OR CASE WHEN p.nomination_id!=? THEN 'STALE' ELSE COALESCE(d.decision,'PENDING') END=?)`
	args := []any{p.recoveryEpoch, kind, kind, state, p.recoveryNomination, state}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM recovery_proposals p LEFT JOIN recovery_decisions d ON d.proposal_id=p.id"+where, args...).Scan(&x.Total); err != nil {
		return x, err
	}
	rows, err := tx.QueryContext(ctx, recoveryProposalSelect+where+" ORDER BY p.created_at DESC,p.rowid DESC LIMIT 12 OFFSET ?", append([]any{p.recoveryNomination}, append(args, (page-1)*12)...)...)
	if err != nil {
		return x, err
	}
	for rows.Next() {
		item, e := scanRecoveryProposal(rows)
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

func validateRecoveryProposal(in RecoveryProposalInput, now time.Time, retry bool) error {
	if !operationPattern.MatchString(in.OperationKey) || !custodyDigest.MatchString(in.ScopeKey) || !validText(in.Title, 5, 120) || !paragraph(in.Reason, 10, 1000) ||
		(in.PredecessorID == "" && in.PredecessorVersion != 0) || (in.PredecessorID != "" && (len(in.PredecessorID) != 43 || in.PredecessorVersion < 1)) {
		return ErrInvalid
	}
	if in.Kind == "ACCESS" {
		if in.Access == nil || in.Work != nil {
			return ErrInvalid
		}
		a := in.Access
		email, err := normalizeEmail(a.Email)
		if err != nil || email != a.Email || !validText(a.Name, 2, 120) || !validText(a.IdentityRef, 3, 120) || !validText(a.EvidenceRef, 5, 300) || len(a.AccountID) > 100 ||
			a.ValidFrom <= 0 || a.ValidUntil <= a.ValidFrom || a.ValidUntil-a.ValidFrom > 365*86400 || (!retry && (a.ValidFrom > now.Unix() || a.ValidUntil <= now.Unix() || !recoveryVerification(a.IdentityRef, a.EvidenceRef, a.VerifiedAt, now))) {
			return ErrInvalid
		}
		if a.Role == "HOUSEHOLD" {
			if len(a.HomeID) == 0 || len(a.HomeID) > 100 || (a.Relationship != "OWNER" && a.Relationship != "TENANT" && a.Relationship != "FAMILY" && a.Relationship != "AUTHORIZED_OCCUPANT") {
				return ErrInvalid
			}
		} else if !validAppointment(a.Role) || a.HomeID != "" || a.Relationship != "" || a.Finance {
			return ErrInvalid
		}
	} else if in.Kind == "EXTERNAL" {
		if in.Work == nil || in.Access != nil {
			return ErrInvalid
		}
		w := in.Work
		if (w.Kind != "OFFSITE" && w.Kind != "MESSAGE") || len(w.JobID) < 1 || len(w.JobID) > 100 || !validText(w.OriginalState, 1, 40) || !validText(w.EvidenceRef, 5, 300) ||
			(w.Outcome != "RETAIN_UNKNOWN" && w.Outcome != "EVIDENCE_CONFIRMED" && w.Outcome != "ABANDONED") {
			return ErrInvalid
		}
	} else {
		return ErrInvalid
	}
	return nil
}
func (s *Store) ProposeRecovery(ctx context.Context, token string, in RecoveryProposalInput) (RecoveryProposal, error) {
	var x RecoveryProposal
	if err := validateRecoveryProposal(in, time.Now(), true); err != nil {
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
	var id, oldDigest, author string
	err = tx.QueryRowContext(ctx, "SELECT id,input_digest,author_id FROM recovery_proposals WHERE operation_key=?", in.OperationKey).Scan(&id, &oldDigest, &author)
	if err == nil {
		if oldDigest != digest || author != p.ID {
			return x, ErrConflict
		}
		x, err = recoveryProposal(ctx, tx, p, id)
		if err != nil {
			return x, err
		}
		return x, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, err
	}
	reused, err := recoveryOperationUsed(ctx, tx, in.OperationKey)
	if err != nil {
		return x, err
	}
	if reused {
		return x, ErrConflict
	}
	if err = validateRecoveryProposal(in, time.Now(), false); err != nil {
		return x, err
	}
	if a := in.Access; a != nil {
		if a.AccountID != "" {
			var email string
			if err = tx.QueryRowContext(ctx, "SELECT login FROM users WHERE id=?", a.AccountID).Scan(&email); errors.Is(err, sql.ErrNoRows) {
				return x, ErrConflict
			}
			if err != nil {
				return x, err
			}
			if email != a.Email {
				return x, ErrConflict
			}
		}
		if a.HomeID != "" {
			var exists bool
			if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)", a.HomeID).Scan(&exists); err != nil {
				return x, err
			}
			if !exists {
				return x, ErrInvalid
			}
		}
	}
	if w := in.Work; w != nil {
		var state string
		err = tx.QueryRowContext(ctx, "SELECT original_state FROM recovery_external_jobs WHERE epoch_id=? AND job_kind=? AND job_id=?", p.recoveryEpoch, w.Kind, w.JobID).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && state != w.OriginalState) {
			return x, ErrConflict
		}
		if err != nil {
			return x, err
		}
	}
	version := 1
	var predecessor any
	if in.PredecessorID != "" {
		previous, e := recoveryProposal(ctx, tx, p, in.PredecessorID)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				e = ErrConflict
			}
			return x, e
		}
		if previous.Version != in.PredecessorVersion || previous.Kind != in.Kind || (previous.State == "PENDING" && previous.nomination == p.recoveryNomination) ||
			(in.Access != nil && previous.Input.Access.IdentityRef != in.Access.IdentityRef) || (in.Work != nil && (previous.Input.Work.Kind != in.Work.Kind || previous.Input.Work.JobID != in.Work.JobID)) {
			return x, ErrConflict
		}
		var successor bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM recovery_proposals WHERE predecessor=?)", previous.ID).Scan(&successor); err != nil {
			return x, err
		}
		if successor {
			return x, ErrConflict
		}
		predecessor = in.PredecessorID
		version = previous.Version + 1
	}
	id = randomToken()
	if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_proposals VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", id, in.OperationKey, p.recoveryEpoch, p.recoveryNomination, p.ID, p.recoveryIdentity, in.Kind, version, predecessor, digest, string(body), time.Now().Unix()); err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, p.ID, "RECOVERY_PROPOSAL_RETAINED", in.Reason, map[string]any{"proposal_id": id, "epoch_id": p.recoveryEpoch, "kind": in.Kind, "version": version, "input_digest": digest, "private_access_withheld": true}); err != nil {
		return x, err
	}
	x, err = recoveryProposal(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
func (s *Store) DecideRecovery(ctx context.Context, token, id string, in RecoveryDecisionInput) (RecoveryProposal, error) {
	var x RecoveryProposal
	if len(id) != 43 || !operationPattern.MatchString(in.OperationKey) || !custodyDigest.MatchString(in.ScopeKey) || in.Version < 1 || !in.Confirmed ||
		(in.Decision != "APPROVED" && in.Decision != "DECLINED") || !paragraph(in.Reason, 10, 1000) || !validText(in.Attestation, 10, 300) {
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
	var savedID, actor, oldDigest string
	err = tx.QueryRowContext(ctx, "SELECT proposal_id,actor_id,input_digest FROM recovery_decisions WHERE operation_key=?", in.OperationKey).Scan(&savedID, &actor, &oldDigest)
	if err == nil {
		if savedID != id || actor != p.ID || oldDigest != digest {
			return x, ErrConflict
		}
		x, err = recoveryProposal(ctx, tx, p, id)
		if err != nil {
			return x, err
		}
		return x, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return x, err
	}
	reused, err := recoveryOperationUsed(ctx, tx, in.OperationKey)
	if err != nil {
		return x, err
	}
	if reused {
		return x, ErrConflict
	}
	x, err = recoveryProposal(ctx, tx, p, id)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrConflict
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
	if _, err = tx.ExecContext(ctx, "INSERT INTO recovery_decisions VALUES(?,?,?,?,?,?,?,?,?,?)", id, in.OperationKey, p.ID, p.recoveryIdentity, in.Version, in.Decision, in.Reason, in.Attestation, digest, time.Now().Unix()); err != nil {
		return x, err
	}
	if err = accountAudit(ctx, tx, p.ID, x.AuthorID, "RECOVERY_PROPOSAL_"+in.Decision, in.Reason, map[string]any{"proposal_id": id, "epoch_id": p.recoveryEpoch, "version": in.Version, "input_digest": digest, "prepared_only": true, "private_access_withheld": true}); err != nil {
		return x, err
	}
	x, err = recoveryProposal(ctx, tx, p, id)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
