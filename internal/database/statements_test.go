package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func statementInput(data []byte) StatementInput {
	hash := sha256.Sum256(data)
	return StatementInput{OperationKey: randomToken(), Title: "Fictional externally prepared income statement", Kind: "INCOME", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30", PreparedBy: "Fictional accountant", Source: "Supplied fictional accounting worksheet SEPT-2026", Filename: "income.csv", Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), Confirmed: true, Reason: "Checked the supplied fictional original and period"}
}
func statementDetail(t *testing.T, s *Store, token, id string) StatementDetail {
	t.Helper()
	x, err := s.StatementFor(context.Background(), token, id, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func statementDecision(x StatementFile, action string) StatementAction {
	return StatementAction{OperationKey: randomToken(), Version: x.Version, Action: action, Confirmed: true, Reason: "Independently checked this supplied version and decision"}
}
func checkedStatement(t *testing.T, s *Store, token string, in StatementInput, data []byte) string {
	t.Helper()
	ctx := context.Background()
	id, err := s.ReserveStatement(ctx, token, in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteStatement(ctx, token, id, data); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimStatementCheck(ctx, time.Now())
	if err != nil || job.ID != id {
		t.Fatal(job, err)
	}
	if err = s.FinishStatementCheck(ctx, job, "text/csv; charset=utf-8", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	return id
}
func approvedStatement(t *testing.T, s *Store, a, b string, in StatementInput, data []byte) string {
	t.Helper()
	id := checkedStatement(t, s, a, in, data)
	if _, err := s.DecideStatement(context.Background(), b, id, statementDecision(statementDetail(t, s, b, id).StatementFile, "APPROVED")); err != nil {
		t.Fatal(err)
	}
	return id
}
func publicationInput(t *testing.T, s *Store, token, id, kind string) StatementPublicationInput {
	t.Helper()
	x := statementDetail(t, s, token, id)
	in := StatementPublicationInput{OperationKey: randomToken(), FileID: id, FileVersion: x.Version, Target: MessageTarget{Kind: kind}, Confirmed: true, Reason: "Intentionally sharing this frozen version with the chosen audience"}
	preview, err := s.PreviewStatementPublication(context.Background(), token, in)
	if err != nil {
		t.Fatal(err)
	}
	in.PreviewHash = preview.PreviewHash
	return in
}
func publishStatement(t *testing.T, s *Store, a, b, id, kind string) string {
	t.Helper()
	pub, err := s.ProposeStatementPublication(context.Background(), a, publicationInput(t, s, a, id, kind))
	if err != nil {
		t.Fatal(err)
	}
	in := StatementAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "Separately approved the exact original and audience"}
	if _, err = s.DecideStatementPublication(context.Background(), b, pub, in); err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestStatementSeparateReviewPublicationTenantScopeAndOriginalAmountsRemainUntouched(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	b := maintenanceReviewer(t, s, a)
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	entry := post(t, s, a, received("432.19"))
	original, err := s.EntryFor(ctx, a, entry)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("Description,Amount\nFictional external figure,50000.00\n")
	in := statementInput(data)
	id := checkedStatement(t, s, a, in, data)
	if _, err = s.DecideStatement(ctx, a, id, statementDecision(statementDetail(t, s, a, id).StatementFile, "APPROVED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("self review", err)
	}
	for _, token := range []string{owner, tenant} {
		if _, err = s.StatementFor(ctx, token, id, 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("private pending original", err)
		}
	}
	if _, err = s.DecideStatement(ctx, b, id, statementDecision(statementDetail(t, s, b, id).StatementFile, "APPROVED")); err != nil {
		t.Fatal(err)
	}
	if _, err = s.StatementFor(ctx, tenant, id, 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("internal approval published automatically", err)
	}
	pubIn := publicationInput(t, s, a, id, "TENANTS")
	preview, err := s.PreviewStatementPublication(ctx, a, pubIn)
	if err != nil || preview.TargetPeople != 35 {
		t.Fatal("independent tenant audience", preview, err)
	}
	pub, err := s.ProposeStatementPublication(ctx, a, pubIn)
	if err != nil {
		t.Fatal(err)
	}
	decision := StatementAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "Separately approved the frozen tenant audience"}
	if _, err = s.DecideStatementPublication(ctx, a, pub, decision); !errors.Is(err, ErrForbidden) {
		t.Fatal("self publication", err)
	}
	if _, err = s.DecideStatementPublication(ctx, b, pub, decision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideStatementPublication(ctx, b, pub, decision); err != nil {
		t.Fatal("stable accepted retry", err)
	}
	visible, err := s.StatementFor(ctx, tenant, id, 1, 1, 1)
	if err != nil || visible.Staff || len(visible.Events) != 0 || len(visible.Versions) != 0 || len(visible.Publications) != 0 || visible.UploaderID != "" || visible.ReviewerID != "" || visible.CanReview || visible.CanPublish {
		t.Fatal("public metadata leaks private review", visible, err)
	}
	if _, err = s.StatementFor(ctx, owner, id, 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("tenant publication widened to owner", err)
	}
	_, download, err := s.DownloadStatement(ctx, tenant, id)
	if err != nil || !bytes.Equal(download, data) {
		t.Fatal("unchanged published original", err)
	}
	if _, err = s.EntryFor(ctx, tenant, entry); !errors.Is(err, ErrForbidden) {
		t.Fatal("statement gave tenant home finance access", err)
	}
	after, err := s.EntryFor(ctx, a, entry)
	if err != nil || after.AmountPaise != 43219 || after.ReceiptID != original.ReceiptID {
		t.Fatal("uploaded figures altered received money", after, err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_accesses", 1)
	for _, q := range []string{"DELETE FROM statement_files", "UPDATE statement_objects SET original_bytes=X'01'", "DELETE FROM statement_publication_events", "DELETE FROM statement_accesses"} {
		if _, err = s.DB.Exec(q); err == nil {
			t.Fatal("history/original changed", q)
		}
	}
	accessExec(t, s, `UPDATE flat_memberships SET end_date=date('now') WHERE resident_id='demo-tenant-A-103' AND end_date IS NULL`)
	if _, _, err = s.DownloadStatement(ctx, tenant, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended membership retained publication", err)
	}
	changed := decision
	changed.Reason = "Changed reason under the same operation"
	if _, err = s.DecideStatementPublication(ctx, b, pub, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed operation accepted", err)
	}
	blob, _ := json.Marshal(visible)
	if strings.Contains(string(blob), "Independently checked") {
		t.Fatal("private reason exposed")
	}
}

func TestStatementReplacementRetainsPublishedOriginalUntilSeparatePublicationAndRevocation(t *testing.T) {
	s, a := recordFixture(t)
	b := maintenanceReviewer(t, s, a)
	tenant := reviewLogin(t, s, "tenant@demo.society")
	ctx := context.Background()
	oldData := []byte("Description,Amount\nOriginal external statement,432.19\n")
	old := approvedStatement(t, s, a, b, statementInput(oldData), oldData)
	oldPub := publishStatement(t, s, a, b, old, "TENANTS")
	head := statementDetail(t, s, a, old)
	nextData := []byte("Description,Amount\nCorrected external statement,531.20\n")
	in := statementInput(nextData)
	in.Replaces = old
	in.Version = head.Version
	in.Title = "Fictional replacement with retained prior publication"
	next := approvedStatement(t, s, a, b, in, nextData)
	if x := statementDetail(t, s, a, next); x.Revision != 2 || !x.Current || x.PublicationID != "" {
		t.Fatal("replacement falsely claimed old publication", x)
	}
	if x := statementDetail(t, s, a, old); x.Current || x.PublicationID != oldPub {
		t.Fatal("earlier published original lost", x)
	}
	page, err := s.StatementsFor(ctx, tenant, "", "", "", 1)
	if err != nil || page.Total != 1 || page.Items[0].ID != old {
		t.Fatal("resident switched before publication", page, err)
	}
	if _, err = s.StatementFor(ctx, tenant, next, 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("unpublished replacement leaked", err)
	}
	summary, err := s.OverviewFor(ctx, a, "statements")
	if err != nil || summary.Counts["published"] != 1 {
		t.Fatal("publication overview", summary, err)
	}
	nextPub := publishStatement(t, s, a, b, next, "TENANTS")
	if _, _, err = s.DownloadStatement(ctx, tenant, old); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("superseded original still exposed", err)
	}
	_, data, err := s.DownloadStatement(ctx, tenant, next)
	if err != nil || !bytes.Equal(data, nextData) {
		t.Fatal("replacement bytes", err)
	}
	detail := statementDetail(t, s, a, next)
	var current StatementPublication
	for _, p := range detail.Publications {
		if p.ID == oldPub && (p.State != "SUPERSEDED" || len(p.Events) != 3 || p.File.ID != old) {
			t.Fatal("old publication history", p)
		}
		if p.ID == nextPub {
			current = p
		}
	}
	revoke := StatementAction{OperationKey: randomToken(), Version: current.Version, Action: "REVOKED", Confirmed: true, Reason: "Deliberately removed the current resident publication"}
	if _, err = s.DecideStatementPublication(ctx, a, nextPub, revoke); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{old, next} {
		if _, _, err = s.DownloadStatement(ctx, tenant, id); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("revoked portal download", id, err)
		}
	}
	for _, pair := range []struct {
		id   string
		data []byte
	}{{old, oldData}, {next, nextData}} {
		_, data, err = s.DownloadStatement(ctx, a, pair.id)
		if err != nil || !bytes.Equal(data, pair.data) {
			t.Fatal("treasury original lost", pair.id, err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_files", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_objects", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
}

func TestStatementChangedAudienceWithSameCountNeedsFreshProposalAndCurrentTreasury(t *testing.T) {
	s, a := recordFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	data := []byte("Label,Value\nFictional,432.19\n")
	id := approvedStatement(t, s, a, b, statementInput(data), data)
	in := publicationInput(t, s, a, id, "TENANTS")
	pub, err := s.ProposeStatementPublication(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, `UPDATE flat_memberships SET end_date=date('now') WHERE resident_id='demo-tenant-A-103' AND end_date IS NULL`)
	accessExec(t, s, `INSERT INTO residents VALUES('statement-new-tenant','Fictional new tenant')`)
	accessExec(t, s, `INSERT INTO flat_memberships VALUES('statement-new-member','demo-flat-A-103','statement-new-tenant','TENANT','2025-01-01',NULL,0,0)`)
	fresh, err := s.PreviewStatementPublication(ctx, a, in)
	if err != nil || fresh.TargetPeople != 35 || fresh.PreviewHash == in.PreviewHash {
		t.Fatal("same count hid changed people", fresh, err)
	}
	x := statementDetail(t, s, b, id).Publications[0]
	if x.CanApprove || x.Problem != "AUDIENCE_CHANGED" || !x.CanDecline {
		t.Fatal("changed proposal capabilities", x)
	}
	decision := StatementAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Confirmed: true, Reason: "Separately reviewed this audience before approval"}
	if _, err = s.DecideStatementPublication(ctx, b, pub, decision); !errors.Is(err, ErrConflict) {
		t.Fatal("stale audience approved", err)
	}
	if _, err = s.DecideStatementPublication(ctx, a, pub, StatementAction{OperationKey: randomToken(), Version: 1, Action: "WITHDRAWN", Confirmed: true, Reason: "Withdraw changed audience for a new exact proposal"}); err != nil {
		t.Fatal(err)
	}
	in.OperationKey = randomToken()
	in.PreviewHash = fresh.PreviewHash
	pub, err = s.ProposeStatementPublication(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, `UPDATE role_grants SET revoked_at=unixepoch() WHERE user_id='demo-user-admin' AND role='TREASURER' AND revoked_at IS NULL`)
	if _, err = s.DecideStatementPublication(ctx, b, pub, decision); !errors.Is(err, ErrForbidden) {
		t.Fatal("ended proposer finance authority", err)
	}
	if x = statementDetail(t, s, b, id).Publications[0]; x.CanApprove || x.Problem != "PROPOSER_AUTHORITY_ENDED" {
		t.Fatal("ended proposer capability", x)
	}
	if _, err = s.ReserveStatement(ctx, a, statementInput(data)); !errors.Is(err, ErrForbidden) {
		t.Fatal("registry administration implied finance", err)
	}
}

func TestStatementFreshAuthorityPrecedesAcceptedRetriesAndScopeChoosersAreBounded(t *testing.T) {
	s, a := recordFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	data := []byte("Label,Value\nFictional,432.19\n")
	in := statementInput(data)
	id := checkedStatement(t, s, a, in, data)
	decision := statementDecision(statementDetail(t, s, b, id).StatementFile, "APPROVED")
	if _, err := s.DecideStatement(ctx, b, id, decision); err != nil {
		t.Fatal(err)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=0 WHERE token_hash=?", TokenHash(b))
	if _, err := s.DecideStatement(ctx, b, id, decision); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale accepted retry", err)
	}
	accessExec(t, s, "UPDATE sessions SET reauthenticated_at=unixepoch() WHERE token_hash=?", TokenHash(b))
	accessExec(t, s, `UPDATE role_grants SET revoked_at=unixepoch() WHERE user_id='demo-user-committee' AND role='TREASURER' AND revoked_at IS NULL`)
	if _, err := s.DecideStatement(ctx, b, id, decision); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked accepted retry", err)
	}
	if _, err := s.StatementFor(ctx, b, id, 1, 1, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("committee sees private finance files", err)
	}
	for i := 0; i < 12; i++ {
		candidate := statementInput(data)
		candidate.Title = "PRIVATE statement original needing another review"
		if _, err := s.ReserveStatement(ctx, a, candidate); err != nil {
			t.Fatal(err)
		}
	}
	page, err := s.StatementsFor(ctx, a, "", "", "", 1)
	if err != nil || page.Total != 13 || len(page.Items) != 12 || page.PageSize != 12 {
		t.Fatal("bounded register", page, err)
	}
	page, err = s.StatementsFor(ctx, a, "", "", "", 2)
	if err != nil || len(page.Items) != 1 {
		t.Fatal("second page", page, err)
	}
	for _, kind := range []string{"HOMES", "PEOPLE"} {
		choices, err := s.StatementTargetsFor(ctx, a, kind, "", 1)
		if err != nil || len(choices.Items) != 12 || choices.Total < 100 {
			t.Fatal("bounded current audience choices", choices, err)
		}
		if _, err = s.StatementTargetsFor(ctx, b, kind, "", 1); !errors.Is(err, ErrForbidden) {
			t.Fatal("implicit audience chooser", err)
		}
	}
	for _, query := range []struct {
		kind, state string
		page        int
	}{{"BAD", "", 1}, {"", "BAD", 1}, {"", "", 0}, {"", "", 10001}} {
		if _, err = s.StatementsFor(ctx, a, "", query.kind, query.state, query.page); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid statement bounds", err)
		}
	}
}

func TestStatementValidationLeasesRetriesExpirationAndSharedStorageAllowance(t *testing.T) {
	s, a := recordFixture(t)
	ctx := context.Background()
	data := []byte("Label,Value\nFictional,432.19\n")
	in := statementInput(data)
	id, err := s.ReserveStatement(ctx, a, in)
	if err != nil {
		t.Fatal(err)
	}
	attention, err := s.OverviewFor(ctx, a, "statements")
	if err != nil || attention.Counts["originals_checking"] != 0 || attention.Counts["upload_attention"] != 1 {
		t.Fatal("an unuploaded reservation is not being checked", attention, err)
	}
	if err = s.CompleteStatement(ctx, a, id, []byte("Wrong bytes")); !errors.Is(err, ErrInvalid) {
		t.Fatal("changed original accepted", err)
	}
	if err = s.CompleteStatement(ctx, a, id, data); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteStatement(ctx, a, id, data); err != nil {
		t.Fatal("lost upload response retry", err)
	}
	now := time.Now()
	first, err := s.ClaimStatementCheck(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ClaimStatementCheck(ctx, now.Add(31*time.Second))
	if err != nil || second.LeaseToken == first.LeaseToken {
		t.Fatal("lease renewal", second, err)
	}
	if err = s.FinishStatementCheck(ctx, first, "text/csv; charset=utf-8", "", now.Add(32*time.Second)); err != nil {
		t.Fatal(err)
	}
	if x := statementDetail(t, s, a, id); x.Validation != "VALIDATING" || x.Version != second.Version {
		t.Fatal("stale checker overwrote lease", x)
	}
	third, err := s.ClaimStatementCheck(ctx, now.Add(62*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimStatementCheck(ctx, now.Add(93*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("fourth automatic attempt", err)
	}
	if err = s.FinishStatementCheck(ctx, third, "text/csv; charset=utf-8", "", now.Add(94*time.Second)); err != nil {
		t.Fatal(err)
	}
	x := statementDetail(t, s, a, id)
	if x.Validation != "REJECTED" || x.ValidationCode != "CHECKS_UNAVAILABLE" || !x.CanRetry {
		t.Fatal("bounded checking result", x)
	}
	if _, err = s.DecideStatement(ctx, a, id, statementDecision(x.StatementFile, "RETRY_VALIDATION")); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimStatementCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishStatementCheck(ctx, job, "text/csv; charset=utf-8", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	x = statementDetail(t, s, a, id)
	if x.Validation != "AVAILABLE" || x.CanRetry {
		t.Fatal("explicit checker retry", x)
	}
	expired, err := s.ReserveStatement(ctx, a, statementInput(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimStatementCheck(ctx, time.Now().Add(25*time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired reservation work", err)
	}
	if x = statementDetail(t, s, a, expired); x.Validation != "ABANDONED" || x.CanUpload {
		t.Fatal("expired reservation", x)
	}
	// Reserve exact remaining metadata allowance; no giant byte object is needed.
	own, _, err := documentStorageUsage(ctx, s.DB, "demo-user-admin")
	if err != nil {
		t.Fatal(err)
	}
	remaining := LocalUserDocumentQuota - own
	for remaining > 0 {
		item := statementInput(data)
		item.Filename = "allowance.xlsx"
		item.Size = 10 * 1024 * 1024
		if remaining < item.Size {
			item.Size = remaining
		}
		if _, err = s.ReserveStatement(ctx, a, item); err != nil {
			t.Fatal(err)
		}
		remaining -= item.Size
	}
	if _, err = s.ReserveStatement(ctx, a, statementInput(data)); !errors.Is(err, ErrInvalid) {
		t.Fatal("statement storage allowance", err)
	}
	doc, _ := documentFixtureInput()
	doc.Category = "ACCOUNTING_EXPORT"
	doc.Visibility = "ACCOUNTING_ONLY"
	if _, err = s.ReserveDocument(ctx, a, doc); !errors.Is(err, ErrInvalid) {
		t.Fatal("statement reservations bypassed shared library allowance", err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_objects", 1)
}

func TestStatementConcurrentReplacementReservationsProduceOneOriginalAndStableWinnerRetry(t *testing.T) {
	s, a := recordFixture(t)
	b := maintenanceReviewer(t, s, a)
	ctx := context.Background()
	data := []byte("Label,Value\nOriginal,432.19\n")
	id := approvedStatement(t, s, a, b, statementInput(data), data)
	head := statementDetail(t, s, a, id)
	inputs := []StatementInput{statementInput(data), statementInput(data)}
	for i := range inputs {
		inputs[i].Replaces = id
		inputs[i].Version = head.Version
	}
	type result struct {
		id    string
		err   error
		index int
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for i := range inputs {
		go func(index int) {
			<-start
			x, e := s.ReserveStatement(ctx, a, inputs[index])
			results <- result{x, e, index}
		}(i)
	}
	close(start)
	successes, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err == nil {
			successes++
			same, e := s.ReserveStatement(ctx, a, inputs[r.index])
			if e != nil || same != r.id {
				t.Fatal("winner stable retry", same, e)
			}
		} else if errors.Is(r.err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal("unexpected concurrent reservation", r.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal(successes, conflicts)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_files", 2)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM statement_objects", 1)
}
