package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestMoveChecklistOwnHistoryEndedMembershipAndExpiredStaffNeverExpandHouseholdAccess(t *testing.T) {
	s, a, b, o := moveFixture(t)
	ctx := context.Background()
	tenant := reviewLogin(t, s, "tenant@demo.society")
	input := moveInput()
	id, e := s.SubmitMoveChecklist(ctx, o, input)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.MoveChecklistFor(ctx, tenant, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("tenant read another person's case", e)
	}
	if _, e = s.MoveChecklistOptionsFor(ctx, o, "", "demo-flat-A-103", 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("own picker disclosed another household", e)
	}
	forged := moveInput()
	forged.FlatID = "demo-flat-A-103"
	if _, e = s.SubmitMoveChecklist(ctx, o, forged); !errors.Is(e, ErrForbidden) {
		t.Fatal("cross-home submission", e)
	}
	for _, flat := range []string{"demo-flat-A-101", "demo-flat-A-102"} {
		detail, e := s.Flat(ctx, flat)
		if e != nil {
			t.Fatal(e)
		}
		for _, m := range detail.Members {
			if m.ID == "demo-owner-A-101" && m.Active {
				if e = s.EndMembership(ctx, a, flat, m.MembershipID, EndMembership{RegistryChange: RegistryChange{Version: detail.Version, Reason: "Fictional owner has moved; preserve their submitted outcome only"}, EndDate: today()}); e != nil {
					t.Fatal(e)
				}
				break
			}
		}
	}
	p, e := s.CheckSession(ctx, o)
	if e != nil || p.CanReadRecords || !p.CanReadChecklists {
		t.Fatal("limited personal history flag", p, e)
	}
	retained := moveDetail(t, s, o, id)
	if retained.Source.Current || retained.Source.PersonName != "" || retained.Source.HomeLabel != "" || retained.CanOpenHome || retained.CanOpenContact || retained.CanRevise || !retained.CanCancel {
		t.Fatal("ended personal scope expanded", retained)
	}
	if _, e = s.SubmitMoveChecklist(ctx, o, input); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended scope replayed original submission", e)
	}
	cancel := moveAction(retained, "CANCELLED")
	moveAct(t, s, o, id, cancel)
	if _, e = s.ActOnMoveChecklist(ctx, o, id, cancel); e != nil {
		t.Fatal("unchanged own withdrawal retry", e)
	}
	if got := moveDetail(t, s, o, id); got.Version != 2 || got.Phase != "CANCELLED" || got.Pending {
		t.Fatal(got)
	}
	staffInput := moveInput()
	staffInput.FlatID = "demo-flat-A-103"
	staffInput.ResidentID = "demo-tenant-A-103"
	staffID, e := s.SubmitMoveChecklist(ctx, b, staffInput)
	if e != nil {
		t.Fatal(e)
	}
	x := moveDetail(t, s, b, staffID)
	checked := moveAction(x, "CHECK")
	checked.CheckKind = "IDENTITY"
	checked.CheckState = "CHECKED"
	checked.Reference = "Independent supplied staff identity reference"
	checked.SourceKey = x.Source.Key
	moveAct(t, s, b, staffID, checked)
	now := time.Now().Unix()
	accessExec(t, s, "UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-committee' AND role='ADMINISTRATOR' AND revoked_at IS NULL", now-172800, now-1)
	p, e = s.CheckSession(ctx, b)
	if e != nil || !p.CanReadAllRecords || p.CanManageRegistry || p.CanReadChecklists {
		t.Fatal("committee ledger inherited checklist staff", p, e)
	}
	if _, e = s.MoveChecklistFor(ctx, b, staffID, 1); !errors.Is(e, ErrForbidden) {
		t.Fatal("former staff author retained another person's case", e)
	}
	if _, e = s.ActOnMoveChecklist(ctx, b, staffID, checked); !errors.Is(e, ErrForbidden) {
		t.Fatal("expired staff replay", e)
	}
	fresh := moveDetail(t, s, a, staffID)
	next := moveAction(fresh, "CHECK")
	next.CheckKind = "REGISTRY"
	next.CheckState = "CHECKED"
	next.Reference = "Independent current registry source"
	next.SourceKey = fresh.Source.Key
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=? WHERE token_hash=?", now-3600, TokenHash(a))
	if _, e = s.ActOnMoveChecklist(ctx, a, staffID, next); !errors.Is(e, ErrReauthRequired) {
		t.Fatal("stale privileged verification", e)
	}
	if moveDetail(t, s, a, staffID).Version != 2 {
		t.Fatal("denied checks changed state")
	}
}
