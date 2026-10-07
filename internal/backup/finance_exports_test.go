package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

func TestFinanceExportRecoveryRetainsReviewedBytesOperationAuditAndOriginalReceiptInvalidatesSessions(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	for _, seed := range []func(context.Context) error{s.SeedDemoAccounts, s.SeedDemoTreasury} {
		if err := seed(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	a := upkeepRecoveryLogin(t, s, "admin@demo.society")
	owner := upkeepRecoveryLogin(t, s, "owner@demo.society")
	id, err := s.CreateEntry(ctx, a, database.EntryInput{OperationKey: "export-recovery-entry-12345", FlatID: "demo-flat-A-101", Kind: "RECEIVED", Amount: "432.19", Date: "2026-01-01", Description: "Fictional prior recovery cash", Payer: "Fictional owner", Method: "CASH"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PostEntry(ctx, a, id, database.EntryAction{OperationKey: "export-recovery-post-12345", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseEntry(ctx, a, id, database.EntryAction{OperationKey: "export-recovery-reverse-12345", Confirmed: true, Reason: "Supplied fictional reference needs a linked correction"}); err != nil {
		t.Fatal(err)
	}
	entry, err := s.EntryFor(ctx, a, id)
	if err != nil || entry.AmountPaise != 43219 || entry.ReceiptID == "" {
		t.Fatal(entry, err)
	}
	filter := database.FinanceExportFilter{Report: "RECEIPTS", Scope: "OWN", From: "2026-01-01", To: "2026-01-31"}
	preview, err := s.PreviewFinanceExport(ctx, owner, filter)
	if err != nil {
		t.Fatal(err)
	}
	in := database.FinanceExportInput{FinanceExportFilter: filter, OperationKey: "export-recovery-snapshot-12345", PreviewHash: preview.ContentHash, Confirmed: true}
	accepted, err := s.CreateFinanceExport(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	_, original, err := s.DownloadFinanceExport(ctx, owner, accepted.ID)
	if err != nil {
		t.Fatal(err)
	}
	var operation, audit, receipt string
	for _, q := range []struct {
		query string
		value *string
	}{{"SELECT request_hash FROM record_operations WHERE actor_id='demo-user-owner' AND operation_key='export-recovery-snapshot-12345'", &operation}, {"SELECT after_json FROM audit_events WHERE action='FINANCE_EXPORTED'", &audit}, {"SELECT snapshot_json FROM receipts WHERE entry_id='" + id + "'", &receipt}} {
		if err = s.DB.QueryRow(q.query).Scan(q.value); err != nil {
			t.Fatal(err)
		}
	}
	bundle := filepath.Join(root, "exports-checkpoint")
	if _, err = Snapshot(ctx, s, bundle, "0.19.0-dev"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "restored", "society.db")
	if _, err = Restore(ctx, bundle, target); err != nil {
		t.Fatal(err)
	}
	recovered, err := database.Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	recovered.MFA = s.MFA
	if err = recovered.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = recovered.FinanceExportFor(ctx, owner, accepted.ID); !errors.Is(err, database.ErrUnauthenticated) {
		t.Fatal("restored session still valid", err)
	}
	owner = upkeepRecoveryLogin(t, recovered, "owner@demo.society")
	metadata, body, err := recovered.DownloadFinanceExport(ctx, owner, accepted.ID)
	if err != nil || !bytes.Equal(original, body) || metadata.Summary.OriginalReceived != 43219 || metadata.Summary.ReversedReceived != 43219 || metadata.Summary.UsableReceived != 0 {
		t.Fatal("restored original bytes/amounts", metadata, err)
	}
	replay, err := recovered.CreateFinanceExport(ctx, owner, in)
	beforeJSON, _ := json.Marshal(accepted)
	afterJSON, _ := json.Marshal(replay)
	if err != nil || !bytes.Equal(beforeJSON, afterJSON) {
		t.Fatal("restored accepted replay", replay, err)
	}
	for _, q := range []struct{ query, want string }{{"SELECT request_hash FROM record_operations WHERE actor_id='demo-user-owner' AND operation_key='export-recovery-snapshot-12345'", operation}, {"SELECT after_json FROM audit_events WHERE action='FINANCE_EXPORTED'", audit}, {"SELECT snapshot_json FROM receipts WHERE entry_id='" + id + "'", receipt}} {
		var got string
		if err = recovered.DB.QueryRow(q.query).Scan(&got); err != nil || got != q.want {
			t.Fatal("recovery changed retained source", q.query, got, err)
		}
	}
	var count int
	if err = recovered.DB.QueryRow("SELECT COUNT(*) FROM audit_events WHERE action='FINANCE_EXPORTED'").Scan(&count); err != nil || count != 1 {
		t.Fatal("replay duplicated recovered audit", count, err)
	}
}
