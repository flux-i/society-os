package database

import (
	"context"
	"encoding/json"
	"strings"
)

type PaidExpenseInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	BudgetID     string `json:"budget_id"`
	Amount       string `json:"amount"`
	PaidDate     string `json:"paid_date"`
	Payee        string `json:"payee"`
	Category     string `json:"category"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	SourceNote   string `json:"source_note"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type PaidExpenseSnapshot struct {
	Version         int    `json:"version"`
	Action          string `json:"action"`
	BudgetID        string `json:"budget_id"`
	BudgetVersion   int    `json:"budget_version"`
	PreviousVersion int    `json:"previous_version"`
	AmountPaise     int64  `json:"amount_paise"`
	PaidDate        string `json:"paid_date"`
	Payee           string `json:"payee"`
	Category        string `json:"category"`
	Method          string `json:"method"`
	Reference       string `json:"reference"`
	SourceNote      string `json:"source_note"`
	SubmittedBy     string `json:"submitted_by"`
	SubmittedAt     int64  `json:"submitted_at"`
	Reason          string `json:"reason"`
}
type PaidExpenseResource struct {
	ID         string               `json:"id"`
	BudgetID   string               `json:"budget_id"`
	Version    int                  `json:"version"`
	Decision   string               `json:"decision"`
	State      string               `json:"state"`
	Snapshot   PaidExpenseSnapshot  `json:"snapshot"`
	Approved   *PaidExpenseSnapshot `json:"approved,omitempty"`
	CanRevise  bool                 `json:"can_revise"`
	CanDecide  bool                 `json:"can_decide"`
	CanCancel  bool                 `json:"can_cancel"`
	CanCorrect bool                 `json:"can_correct"`
	CanVoid    bool                 `json:"can_void"`
}
type PaidExpenseEvent struct {
	Version         int                 `json:"version"`
	ProposalVersion int                 `json:"proposal_version"`
	Action          string              `json:"action"`
	Actor           string              `json:"actor"`
	Reason          string              `json:"reason"`
	At              int64               `json:"at"`
	Snapshot        PaidExpenseSnapshot `json:"snapshot"`
}
type PaidExpenseDetail struct {
	PaidExpenseResource
	BudgetTitle string             `json:"budget_title"`
	Events      []PaidExpenseEvent `json:"events"`
	EventTotal  int                `json:"event_total"`
	EventPage   int                `json:"event_page"`
	PageSize    int                `json:"page_size"`
	CurrentKey  string             `json:"current_key"`
}
type PaidExpensePage struct {
	Items      []PaidExpenseResource `json:"items"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"page_size"`
	Counts     map[string]int64      `json:"counts"`
	CurrentKey string                `json:"current_key"`
}

func expenseHeadIn(ctx context.Context, q identityReader, id string) (budgetHead, string, error) {
	var h budgetHead
	var budget string
	e := q.QueryRowContext(ctx, "SELECT id,budget_id,decision,version,latest_version,COALESCE(pending_version,0),COALESCE(approved_version,0) FROM paid_expense_resources WHERE id=?", id).Scan(&h.ID, &budget, &h.Decision, &h.Version, &h.Latest, &h.Pending, &h.Approved)
	return h, budget, e
}
func expenseSnapshotIn(ctx context.Context, q identityReader, id string, version int) (PaidExpenseSnapshot, error) {
	var x PaidExpenseSnapshot
	var raw string
	e := q.QueryRowContext(ctx, "SELECT snapshot_json FROM paid_expense_versions WHERE resource_id=? AND version=?", id, version).Scan(&raw)
	if e == nil {
		e = json.Unmarshal([]byte(raw), &x)
	}
	return x, e
}
func normaliseExpense(in PaidExpenseInput) (PaidExpenseInput, int64, error) {
	in.Payee = strings.TrimSpace(in.Payee)
	in.Category = strings.TrimSpace(in.Category)
	in.Reference = strings.TrimSpace(in.Reference)
	in.SourceNote = strings.TrimSpace(in.SourceNote)
	in.Reason = strings.TrimSpace(in.Reason)
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 0 || in.Version > 1000000 {
		return in, 0, invalid("Confirm the already-paid record and give a reason of 10–800 characters.")
	}
	if in.Action == "VOID" {
		if in.BudgetID != "" || in.Amount != "" || in.PaidDate != "" || in.Payee != "" || in.Category != "" || in.Method != "" || in.Reference != "" || in.SourceNote != "" {
			return in, 0, invalid("A void correction retains its exact accepted original.")
		}
		return in, 0, nil
	}
	if (in.Action != "PAID" && in.Action != "CORRECTION") || !validMaintenanceDate(in.PaidDate) || in.PaidDate > today() || !validText(in.Payee, 2, 120) || !validText(in.Category, 2, 80) || !validText(in.Reference, 1, 120) || !validText(in.SourceNote, 5, 600) || (in.Method != "CASH" && in.Method != "BANK_TRANSFER" && in.Method != "CHEQUE" && in.Method != "UPI") {
		return in, 0, invalid("Supply who was paid, category, a past or current paid date, method, payment/voucher reference and source note.")
	}
	if len(in.BudgetID) < 1 || len(in.BudgetID) > 100 {
		return in, 0, ErrInvalid
	}
	amount, e := ParseAmount(in.Amount)
	return in, amount, e
}
func expenseIdentity(method, reference string) string {
	return TokenHash(method + ":" + strings.ToUpper(strings.Join(strings.Fields(reference), " ")))
}
func expenseResourceIn(ctx context.Context, q identityReader, p Principal, h budgetHead, budget string) (PaidExpenseResource, error) {
	x := PaidExpenseResource{ID: h.ID, BudgetID: budget, Version: h.Version, Decision: h.Decision, State: "UNCONFIRMED"}
	var e error
	x.Snapshot, e = expenseSnapshotIn(ctx, q, h.ID, h.Latest)
	if e != nil {
		return x, e
	}
	if h.Approved > 0 {
		approved, err := expenseSnapshotIn(ctx, q, h.ID, h.Approved)
		if err != nil {
			return x, err
		}
		x.Approved = &approved
		x.State = "CONFIRMED"
		if approved.Action == "VOID" {
			x.State = "VOID"
		}
	}
	own := h.Pending > 0 && x.Snapshot.SubmittedBy == p.ID
	x.CanRevise = p.CanManageRecords && ((h.Pending > 0 && own) || (h.Pending == 0 && h.Approved == 0))
	x.CanDecide = p.CanManageRecords && h.Pending > 0 && !own
	x.CanCancel = p.CanManageRecords && own
	x.CanCorrect = p.CanManageRecords && h.Approved > 0 && h.Pending == 0
	x.CanVoid = x.CanCorrect && x.State != "VOID"
	return x, nil
}
