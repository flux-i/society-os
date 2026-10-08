package database

import (
	"context"
	"testing"
)

func TestBudgetUnconfirmedReportsAndConfirmedPurposeAllocationsCountOneOriginalReceipt(t *testing.T) {
	s, a, b := budgetFixture(t)
	ctx := context.Background()
	budget := budgetPropose(t, s, a, "", budgetProposal())
	budgetApprove(t, s, b, budget)
	fund := createPublishedFund(t, s, a, b, fundProposal())
	owner := reviewLogin(t, s, "owner@demo.society")
	claim := createFundClaim(t, s, owner, fundClaim(fund.ID, "432.19", "BUDGET-CLAIM-43219"))
	if x := budgetComparison(t, s, a, budget); x.CollectionsPaise != 0 || x.ReceiptCount != 0 {
		t.Fatal("unverified claim became received cash", x)
	}
	confirmation := confirmClaim("BUDGET-ONE-PAYMENT", "200.00")
	for i := 0; i < 2; i++ {
		if _, e := s.DecideFundReport(ctx, b, claim, confirmation); e != nil {
			t.Fatal(e)
		}
	}
	verified := claimDetails(t, s, b, claim)
	if verified.EntryID == "" || verified.ReceiptID == "" || verified.AllocationID == "" {
		t.Fatal("actual original receipt/purpose allocation missing", verified)
	}
	createFundClaim(t, s, owner, fundClaim(fund.ID, "10.00", "BUDGET-STILL-PENDING-CLAIM"))
	x := budgetComparison(t, s, a, budget)
	if x.OriginalCollectionsPaise != 43219 || x.CollectionsPaise != 43219 || x.ReceiptCount != 1 || x.RecordedExpensesPaise != 0 {
		t.Fatal("report, its receipt or attribution was counted again", x)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
}
