package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"time"
)

// These are supplied differences and evidence, never confirmed ledger entries.
type RecoveryGapItem struct {
	BookRef       string `json:"book_ref"`
	ItemRef       string `json:"item_ref"`
	FlatID        string `json:"flat_id"`
	Kind          string `json:"kind"`
	Amount        string `json:"amount"`
	Date          string `json:"date"`
	EntryRef      string `json:"entry_ref"`
	ReceiptNumber string `json:"receipt_number"`
	Evidence      string `json:"evidence"`
	EvidenceState string `json:"evidence_state"`
}

type RecoveryGapInput struct {
	OperationKey       string            `json:"operation_key"`
	ScopeKey           string            `json:"scope_key"`
	PredecessorID      string            `json:"predecessor_id"`
	PredecessorVersion int               `json:"predecessor_version"`
	Title              string            `json:"title"`
	Reason             string            `json:"reason"`
	CoverageFrom       int64             `json:"coverage_from"`
	CoverageThrough    int64             `json:"coverage_through"`
	VerifiedAt         int64             `json:"verified_at"`
	CoverageEvidence   string            `json:"coverage_evidence"`
	CoverageComplete   bool              `json:"coverage_complete"`
	DeclaredEmpty      bool              `json:"declared_empty"`
	Items              []RecoveryGapItem `json:"items"`
}

type RecoveryGapHome struct {
	FlatID          string `json:"flat_id"`
	Home            string `json:"home"`
	DebitPaise      int64  `json:"debit_paise"`
	CreditPaise     int64  `json:"credit_paise"`
	VoluntaryPaise  int64  `json:"voluntary_paise"`
	CheckpointPaise int64  `json:"checkpoint_paise"`
	DifferencePaise int64  `json:"difference_paise"`
	ComparisonPaise int64  `json:"comparison_paise"`
}

type RecoveryGapMatch struct {
	Item           int    `json:"item"`
	EntryRef       string `json:"entry_ref"`
	EntryKind      string `json:"entry_kind"`
	LinkedReversal bool   `json:"linked_reversal"`
	ReceiptMatched bool   `json:"receipt_matched"`
}

type RecoveryGapComparison struct {
	Homes              []RecoveryGapHome  `json:"homes"`
	Matches            []RecoveryGapMatch `json:"matches"`
	KnownItems         int                `json:"known_items"`
	UnresolvedItems    int                `json:"unresolved_items"`
	CoverageUnresolved bool               `json:"coverage_unresolved"`
	PreparedOnly       bool               `json:"prepared_only"`
}

type RecoveryGapPreparation struct {
	ID            string                `json:"id"`
	Title         string                `json:"title"`
	State         string                `json:"state"`
	Version       int                   `json:"version"`
	AuthorID      string                `json:"author_id"`
	Author        string                `json:"author"`
	CreatedAt     int64                 `json:"created_at"`
	PredecessorID string                `json:"predecessor_id"`
	Input         RecoveryGapInput      `json:"input"`
	Comparison    RecoveryGapComparison `json:"comparison"`
	Decision      *RecoveryDecision     `json:"decision"`
	nomination    string
	epoch         string
	identity      string
}

type RecoveryGapPage struct {
	Items    []RecoveryGapPreparation `json:"items"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type RecoveryGapStatus struct {
	Pending         int    `json:"pending"`
	Approved        int    `json:"approved"`
	Known           int    `json:"known"`
	Unresolved      int    `json:"unresolved"`
	StatusKey       string `json:"status_key"`
	CanResume       bool   `json:"can_resume"`
	declined, stale int
}

func validateGap(in RecoveryGapInput, retry bool) error {
	now := time.Now().Unix()
	if !operationPattern.MatchString(in.OperationKey) || !custodyDigest.MatchString(in.ScopeKey) ||
		!validText(in.Title, 5, 120) || !paragraph(in.Reason, 10, 1000) || !paragraph(in.CoverageEvidence, 10, 1000) ||
		(in.PredecessorID == "" && in.PredecessorVersion != 0) || (in.PredecessorID != "" && (len(in.PredecessorID) != 43 || in.PredecessorVersion < 1)) ||
		in.CoverageFrom <= 0 || in.CoverageThrough < in.CoverageFrom || in.VerifiedAt < in.CoverageThrough ||
		(!retry && (in.VerifiedAt > now || in.VerifiedAt < now-30*24*60*60)) || in.Items == nil || len(in.Items) > 24 ||
		(in.DeclaredEmpty != (len(in.Items) == 0)) || (in.DeclaredEmpty && !in.CoverageComplete) {
		return invalid("Supply a bounded covered interval, current independent evidence and up to 24 items. An empty gap requires an explicit complete-coverage attestation.")
	}
	seen := map[string]bool{}
	for _, item := range in.Items {
		identity := item.BookRef + "\x00" + item.ItemRef
		if !sourceIdentity.MatchString(item.BookRef) || !sourceIdentity.MatchString(item.ItemRef) || seen[identity] ||
			!paragraph(item.Evidence, 10, 300) || (item.Kind != "DEBIT" && item.Kind != "RECEIVED" && item.Kind != "EVIDENCE") ||
			(item.EvidenceState != "SUPPORTED" && item.EvidenceState != "UNKNOWN" && item.EvidenceState != "CONTRADICTORY") ||
			(item.ReceiptNumber != "" && !validText(item.ReceiptNumber, 1, 80)) || (item.EntryRef != "" && len(item.EntryRef) != 43) {
			return invalid("Supply distinct bounded book/item identities, an item kind and explicit independent evidence.")
		}
		seen[identity] = true
		if item.Kind != "EVIDENCE" {
			if item.FlatID == "" || !validDate(item.Date) || item.EntryRef != "" || item.EvidenceState != "SUPPORTED" || (item.Kind == "DEBIT" && item.ReceiptNumber != "") {
				return invalid("A known account difference needs its existing home, supplied date and supported evidence. Existing or unresolved records belong in evidence only.")
			}
		} else if item.EntryRef != "" && (item.FlatID == "" || item.Amount == "" || item.EvidenceState != "SUPPORTED") {
			return invalid("An existing-entry reference needs its exact home, amount and checked evidence.")
		}
		if item.Amount != "" {
			if _, err := ParseAmount(item.Amount); err != nil {
				return err
			}
		} else if item.Kind != "EVIDENCE" {
			return invalid("Supply the exact rupee amount already given or received.")
		}
		if item.Date != "" && !validDate(item.Date) {
			return invalid("Supply a valid calendar date.")
		}
	}
	return nil
}

func gapSum(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, invalid("This comparison exceeds exact supported monetary bounds.")
	}
	x := a + b
	// Existing clients display integer paise through exact JavaScript integers.
	if x > 9007199254740991 || x < -9007199254740991 {
		return 0, invalid("This comparison exceeds exact supported monetary bounds.")
	}
	return x, nil
}

func gapBalance(ctx context.Context, q identityReader, home string) (RecoveryGapHome, error) {
	x := RecoveryGapHome{FlatID: home}
	if err := q.QueryRowContext(ctx, "SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?", home).Scan(&x.Home); err != nil {
		return x, err
	}
	if err := q.QueryRowContext(ctx, `SELECT
 COALESCE(SUM(CASE WHEN e.state='POSTED' AND v.entry_id IS NULL AND e.kind IN('CHARGE','OPENING_DEBIT') THEN e.amount_paise ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN e.state='POSTED' AND v.entry_id IS NULL AND e.kind IN('RECEIVED','OPENING_CREDIT') THEN e.amount_paise ELSE 0 END),0),
 COALESCE(SUM((SELECT COALESCE(SUM(fc.amount_paise),0) FROM live_fund_contributions fc WHERE fc.source_id=e.id)),0)
 FROM entries e LEFT JOIN entry_reversals v ON v.entry_id=e.id WHERE e.flat_id=?`, home).Scan(&x.DebitPaise, &x.CreditPaise, &x.VoluntaryPaise); err != nil {
		return x, err
	}
	value, err := gapSum(x.DebitPaise, -x.CreditPaise)
	if err != nil {
		return x, err
	}
	x.CheckpointPaise, err = gapSum(value, x.VoluntaryPaise)
	x.ComparisonPaise = x.CheckpointPaise
	return x, err
}

func gapCompare(ctx context.Context, tx *sql.Tx, p Principal, in RecoveryGapInput) (RecoveryGapComparison, error) {
	x := RecoveryGapComparison{Homes: []RecoveryGapHome{}, Matches: []RecoveryGapMatch{}, PreparedOnly: true, CoverageUnresolved: !in.CoverageComplete}
	if x.CoverageUnresolved {
		x.UnresolvedItems++
	}
	homes := map[string]RecoveryGapHome{}
	for index, item := range in.Items {
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM recovery_gap_preparations g, json_each(g.input_json,'$.items') j
 WHERE g.epoch_id=? AND g.nomination_id=? AND g.id!=?
 AND NOT EXISTS(SELECT 1 FROM recovery_gap_preparations WHERE predecessor=g.id)
 AND json_extract(j.value,'$.book_ref')=? AND json_extract(j.value,'$.item_ref')=?)`, p.recoveryEpoch, p.recoveryNomination, in.PredecessorID, item.BookRef, item.ItemRef).Scan(&duplicate); err != nil {
			return x, err
		}
		if duplicate {
			return x, invalid("This supplied book/item identity already belongs to a current preparation. Open it for a linked correction.")
		}
		// Checking a book does not reconstruct an unmatched evidence-only
		// financial item. Preserve that unresolved disposition explicitly.
		if item.EvidenceState != "SUPPORTED" || (item.Kind == "EVIDENCE" && item.EntryRef == "") {
			x.UnresolvedItems++
		}
		if item.FlatID != "" {
			if _, exists := homes[item.FlatID]; !exists {
				home, err := gapBalance(ctx, tx, item.FlatID)
				if errors.Is(err, sql.ErrNoRows) {
					return x, invalid("Choose an existing checkpoint home for this item.")
				}
				if err != nil {
					return x, err
				}
				homes[item.FlatID] = home
			}
		}
		if item.EntryRef != "" {
			var home, kind, state, number string
			var amount int64
			var reversed bool
			if err := tx.QueryRowContext(ctx, `SELECT e.flat_id,e.kind,e.state,e.amount_paise,COALESCE(r.number,''),EXISTS(SELECT 1 FROM entry_reversals WHERE entry_id=e.id) FROM entries e LEFT JOIN receipts r ON r.entry_id=e.id WHERE e.id=?`, item.EntryRef).Scan(&home, &kind, &state, &amount, &number, &reversed); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return x, invalid("This supplied entry identity does not exist in the checkpoint.")
				}
				return x, err
			}
			supplied, _ := ParseAmount(item.Amount)
			if home != item.FlatID || amount != supplied || state != "POSTED" || (item.ReceiptNumber != "" && item.ReceiptNumber != number) {
				return x, invalid("The supplied existing-entry home, amount or receipt identity does not match its preserved original.")
			}
			x.Matches = append(x.Matches, RecoveryGapMatch{Item: index, EntryRef: item.EntryRef, EntryKind: kind, LinkedReversal: reversed, ReceiptMatched: item.ReceiptNumber != ""})
		}
		if item.Kind == "EVIDENCE" {
			continue
		}
		if item.ReceiptNumber != "" {
			var exists bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM receipts WHERE number=?)", item.ReceiptNumber).Scan(&exists); err != nil {
				return x, err
			}
			if exists {
				return x, invalid("This printed receipt already exists in the checkpoint. Supply its preserved entry as evidence only.")
			}
		}
		amount, _ := ParseAmount(item.Amount)
		if item.Kind == "RECEIVED" {
			amount = -amount
		}
		home := homes[item.FlatID]
		var err error
		home.DifferencePaise, err = gapSum(home.DifferencePaise, amount)
		if err != nil {
			return x, err
		}
		home.ComparisonPaise, err = gapSum(home.CheckpointPaise, home.DifferencePaise)
		if err != nil {
			return x, err
		}
		homes[item.FlatID] = home
		x.KnownItems++
	}
	for _, home := range homes {
		x.Homes = append(x.Homes, home)
	}
	sort.Slice(x.Homes, func(i, j int) bool { return x.Homes[i].Home < x.Homes[j].Home })
	return x, nil
}

func (s *Store) RecoveryGapBalanceFor(ctx context.Context, token, home string) (RecoveryGapHome, error) {
	if !validText(home, 1, 100) {
		return RecoveryGapHome{}, ErrInvalid
	}
	tx, _, err := s.beginRecoveryRead(ctx, token)
	if err != nil {
		return RecoveryGapHome{}, err
	}
	defer tx.Rollback()
	x, err := gapBalance(ctx, tx, home)
	if err != nil {
		return x, err
	}
	return x, tx.Commit()
}
