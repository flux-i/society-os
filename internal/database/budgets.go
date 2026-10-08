package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrDuplicatePaidExpense = errors.New("duplicate paid expense identity")

type BudgetInput struct {
	OperationKey       string `json:"operation_key"`
	Version            int    `json:"version"`
	Action             string `json:"action"`
	Title              string `json:"title"`
	PeriodStart        string `json:"period_start"`
	PeriodEnd          string `json:"period_end"`
	SourceReference    string `json:"source_reference"`
	PlannedCollections string `json:"planned_collections"`
	PlannedExpenses    string `json:"planned_expenses"`
	Reason             string `json:"reason"`
	Confirmed          bool   `json:"confirmed"`
}
type BudgetAction = CommunityAction
type BudgetSnapshot struct {
	PlanVersion      int    `json:"plan_version"`
	Version          int    `json:"version"`
	Action           string `json:"action"`
	Title            string `json:"title"`
	PeriodStart      string `json:"period_start"`
	PeriodEnd        string `json:"period_end"`
	SourceReference  string `json:"source_reference"`
	CollectionsPaise int64  `json:"collections_paise"`
	ExpensesPaise    int64  `json:"expenses_paise"`
	SubmittedBy      string `json:"submitted_by"`
	SubmittedAt      int64  `json:"submitted_at"`
	Reason           string `json:"reason"`
}
type BudgetResource struct {
	ID            string          `json:"id"`
	Version       int             `json:"version"`
	Decision      string          `json:"decision"`
	State         string          `json:"state"`
	Snapshot      BudgetSnapshot  `json:"snapshot"`
	Approved      *BudgetSnapshot `json:"approved,omitempty"`
	CanRevise     bool            `json:"can_revise"`
	CanDecide     bool            `json:"can_decide"`
	CanCancel     bool            `json:"can_cancel"`
	CanClose      bool            `json:"can_close"`
	CanAddExpense bool            `json:"can_add_expense"`
}
type BudgetEvent struct {
	Version         int            `json:"version"`
	ProposalVersion int            `json:"proposal_version"`
	Action          string         `json:"action"`
	Actor           string         `json:"actor"`
	Reason          string         `json:"reason"`
	At              int64          `json:"at"`
	Snapshot        BudgetSnapshot `json:"snapshot"`
}
type BudgetComparison struct {
	Available                bool   `json:"available"`
	PeriodStart              string `json:"period_start"`
	PeriodEnd                string `json:"period_end"`
	PlanVersion              int    `json:"plan_version"`
	PlannedCollectionsPaise  int64  `json:"planned_collections_paise"`
	PlannedExpensesPaise     int64  `json:"planned_expenses_paise"`
	OriginalCollectionsPaise int64  `json:"original_collections_paise"`
	ReversedCollectionsPaise int64  `json:"reversed_collections_paise"`
	CollectionsPaise         int64  `json:"collections_paise"`
	RecordedExpensesPaise    int64  `json:"recorded_expenses_paise"`
	CollectionVariancePaise  int64  `json:"collection_variance_paise"`
	ExpenseHeadroomPaise     int64  `json:"expense_headroom_paise"`
	DifferencePaise          int64  `json:"difference_paise"`
	ReceiptCount             int    `json:"receipt_count"`
	ExpenseCount             int    `json:"expense_count"`
	PendingExpenses          int    `json:"pending_expenses"`
	CurrentKey               string `json:"current_key"`
	CheckedAt                int64  `json:"checked_at"`
}
type BudgetDetail struct {
	BudgetResource
	Events     []BudgetEvent `json:"events"`
	EventTotal int           `json:"event_total"`
	EventPage  int           `json:"event_page"`
	PageSize   int           `json:"page_size"`
	CurrentKey string        `json:"current_key"`
}
type BudgetPage struct {
	Items      []BudgetResource `json:"items"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Counts     map[string]int64 `json:"counts"`
	CanPrepare bool             `json:"can_prepare"`
	CurrentKey string           `json:"current_key"`
}
type budgetHead struct {
	ID, Decision                       string
	Version, Latest, Pending, Approved int
}

// Committee ledger visibility does not grant private budget access.
func budgetReader(p Principal) bool {
	if p.MFAPending {
		return false
	}
	if p.CanManageRecords {
		return true
	}
	for _, role := range p.Roles {
		if role == "AUDITOR" {
			return true
		}
	}
	return false
}
func (s *Store) beginBudgetRead(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, Principal{}, e
	}
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e == nil && !budgetReader(p) {
		e = ErrForbidden
	}
	if e != nil {
		tx.Rollback()
		return nil, p, e
	}
	return tx, p, nil
}
func (s *Store) beginBudgetWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, p, e := s.beginRecordWrite(ctx, token)
	if e != nil {
		return nil, p, e
	}
	if !p.Fresh {
		tx.Rollback()
		return nil, p, ErrReauthRequired
	}
	return tx, p, nil
}
func budgetHeadIn(ctx context.Context, q identityReader, id string) (budgetHead, error) {
	var h budgetHead
	e := q.QueryRowContext(ctx, "SELECT id,decision,version,latest_version,COALESCE(pending_version,0),COALESCE(approved_version,0) FROM budget_resources WHERE id=?", id).Scan(&h.ID, &h.Decision, &h.Version, &h.Latest, &h.Pending, &h.Approved)
	return h, e
}
func budgetSnapshotIn(ctx context.Context, q identityReader, id string, version int) (BudgetSnapshot, error) {
	var x BudgetSnapshot
	var raw string
	e := q.QueryRowContext(ctx, "SELECT snapshot_json FROM budget_versions WHERE resource_id=? AND version=?", id, version).Scan(&raw)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &x)
	}
	return x, e
}
func plannedAmount(value string) (int64, error) {
	if value == "0" || value == "0.0" || value == "0.00" {
		return 0, nil
	}
	return ParseAmount(value)
}
func normaliseBudget(in BudgetInput) (BudgetInput, int64, int64, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.SourceReference = strings.TrimSpace(in.SourceReference)
	in.Reason = strings.TrimSpace(in.Reason)
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 0 || in.Version > 1000000 {
		return in, 0, 0, invalid("Confirm the supplied plan with a reason of 10–800 characters.")
	}
	if in.Action == "CLOSE" {
		if in.Title != "" || in.PeriodStart != "" || in.PeriodEnd != "" || in.SourceReference != "" || in.PlannedCollections != "" || in.PlannedExpenses != "" {
			return in, 0, 0, invalid("Closing retains the exact approved plan and period.")
		}
		return in, 0, 0, nil
	}
	if in.Action != "PLAN" || !validText(in.Title, 5, 120) || !validText(in.SourceReference, 5, 300) || !validMaintenanceDate(in.PeriodStart) || !validMaintenanceDate(in.PeriodEnd) || in.PeriodEnd < in.PeriodStart {
		return in, 0, 0, invalid("Supply a title, source reference and valid inclusive budget period.")
	}
	start, _ := time.Parse("2006-01-02", in.PeriodStart)
	end, _ := time.Parse("2006-01-02", in.PeriodEnd)
	if end.Sub(start) > 365*24*time.Hour {
		return in, 0, 0, invalid("A budget period may cover at most 366 inclusive days.")
	}
	collections, e := plannedAmount(in.PlannedCollections)
	if e != nil {
		return in, 0, 0, e
	}
	expenses, e := plannedAmount(in.PlannedExpenses)
	return in, collections, expenses, e
}
func validBudgetDecision(in BudgetAction) bool {
	return in.Confirmed && paragraph(in.Reason, 10, 800) && in.Version >= 1 && in.Version <= 1000000 && (in.Action == "APPROVED" || in.Action == "DECLINED" || in.Action == "CANCELLED")
}
func budgetResourceIn(ctx context.Context, q identityReader, p Principal, h budgetHead) (BudgetResource, error) {
	x := BudgetResource{ID: h.ID, Version: h.Version, Decision: h.Decision, State: "UNAPPROVED"}
	var e error
	x.Snapshot, e = budgetSnapshotIn(ctx, q, h.ID, h.Latest)
	if e != nil {
		return x, e
	}
	if h.Approved > 0 {
		approved, err := budgetSnapshotIn(ctx, q, h.ID, h.Approved)
		if err != nil {
			return x, err
		}
		x.Approved = &approved
		x.State = "OPEN"
		if approved.Action == "CLOSE" {
			x.State = "CLOSED"
		}
	}
	own := h.Pending > 0 && x.Snapshot.SubmittedBy == p.ID
	x.CanRevise = p.CanManageRecords && x.State != "CLOSED" && (h.Pending == 0 || own)
	x.CanDecide = p.CanManageRecords && h.Pending > 0 && !own
	x.CanCancel = p.CanManageRecords && own
	x.CanClose = p.CanManageRecords && x.State == "OPEN" && h.Pending == 0
	x.CanAddExpense = p.CanManageRecords && x.State == "OPEN"
	return x, nil
}
