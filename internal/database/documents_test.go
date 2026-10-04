package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"testing"
	"time"
)

func documentFixtureInput() (DocumentInput, []byte) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{40, 65, 50, 255})
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	data := out.Bytes()
	sum := sha256.Sum256(data)
	return DocumentInput{OperationKey: randomToken(), Title: "Fictional resident record", Filename: "निवासी-record.png", Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:]), Category: "RESIDENT_DOCUMENT", Visibility: "RESIDENT_SPECIFIC"}, data
}
func mustLibrary(t *testing.T, s *Store, token, id string) LibraryDocument {
	t.Helper()
	x, err := s.DocumentFor(context.Background(), token, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func checkedLibrary(t *testing.T, s *Store, token string, in DocumentInput, data []byte) string {
	t.Helper()
	ctx := context.Background()
	id, err := s.ReserveDocument(ctx, token, in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, token, id, data); err != nil {
		t.Fatal(err)
	}
	job, err := s.ClaimDocumentCheck(ctx, time.Now())
	if err != nil || job.ID != id {
		t.Fatal("wrong validation work", job.ID, id, err)
	}
	if err = s.FinishDocumentCheck(ctx, job, "image/png", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	return id
}
func libraryAction(x LibraryDocument, action string) DocumentAction {
	return DocumentAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "A separately checked fictional document decision", Confirmed: true}
}
func approveLibrary(t *testing.T, s *Store, reviewer, id string) {
	t.Helper()
	x := mustLibrary(t, s, reviewer, id)
	if _, err := s.DecideDocument(context.Background(), reviewer, id, libraryAction(x, "APPROVED")); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentVersionPaginationCountsOnlyPermittedImmutableVersions(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	in, data := documentFixtureInput()
	in.Category, in.Visibility = "CIRCULAR", "ALL_AUTHORIZED_RESIDENTS"
	current := ""
	for revision := 1; revision <= 14; revision++ {
		if current != "" {
			in.Replaces = current
			in.Version = mustLibrary(t, s, owner, current).Version
		}
		in.OperationKey = randomToken()
		current = checkedLibrary(t, s, owner, in, data)
		approveLibrary(t, s, admin, current)
	}
	in.OperationKey, in.Replaces = randomToken(), current
	in.Version = mustLibrary(t, s, owner, current).Version
	pending := checkedLibrary(t, s, owner, in, data)
	first, err := s.DocumentFor(ctx, tenant, current, 1)
	if err != nil || first.HistoryTotal != 14 || len(first.Versions) != 12 || first.Revision != 14 {
		t.Fatal("first permitted history page", first, err)
	}
	second, err := s.DocumentFor(ctx, tenant, current, 99)
	if err != nil || second.HistoryPage != 2 || second.HistoryTotal != 14 || len(second.Versions) != 2 {
		t.Fatal("clamped permitted history", second, err)
	}
	seen := map[string]bool{}
	for _, version := range append(first.Versions, second.Versions...) {
		if seen[version.ID] || version.ID == pending || version.State != "APPROVED" {
			t.Fatal("duplicate or hidden version", version)
		}
		seen[version.ID] = true
	}
	private := mustLibrary(t, s, owner, current)
	if private.HistoryTotal != 15 || len(seen) != 14 || len(first.Events) != 0 || len(private.Events) == 0 {
		t.Fatal("private review count/trail exposed or originals skipped")
	}
	if _, err = s.DocumentFor(ctx, tenant, pending, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("pending replacement exposed", err)
	}
	if _, err = s.DB.Exec(`UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-tenant-A-103'`, today()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DocumentFor(ctx, tenant, current, 2); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended audience retained history pagination", err)
	}
}

func TestDocumentsPendingPersonalHomeAndSocietyScopesFollowCurrentEntitlement(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	in, data := documentFixtureInput()
	id := checkedLibrary(t, s, owner, in, data)
	if _, err := s.DocumentFor(ctx, tenant, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("private filename leaked", err)
	}
	if page, err := s.DocumentsFor(ctx, tenant, "resident record", "", "", 1); err != nil || page.Total != 0 {
		t.Fatal("private count/search", page, err)
	}
	if _, err := s.DecideDocument(ctx, owner, id, libraryAction(mustLibrary(t, s, owner, id), "APPROVED")); !errors.Is(err, ErrForbidden) {
		t.Fatal("uploader approved file", err)
	}
	approveLibrary(t, s, admin, id)
	if _, dataGot, err := s.DownloadDocument(ctx, owner, id); err != nil || !bytes.Equal(dataGot, data) {
		t.Fatal("approved personal download", err)
	}
	in.OperationKey = randomToken()
	in.Title = "Fictional home record"
	in.Visibility = "FLAT_SPECIFIC"
	in.FlatID = "demo-flat-A-101"
	homeID := checkedLibrary(t, s, owner, in, data)
	now := time.Now().Unix()
	if _, err := s.DB.Exec(`INSERT INTO users(id,resident_id,login,display_name,password_hash,created_at,verified_at,password_changed_at,is_demo) VALUES('joint-doc-user','demo-joint-owner','joint-doc@demo.society','Demo Joint Doc Owner',?,?,?,?,1)`, HashPassword(DemoPassword), now, now, now); err != nil {
		t.Fatal(err)
	}
	joint := reviewLogin(t, s, "joint-doc@demo.society")
	if _, err := s.DocumentFor(ctx, joint, homeID, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("pending file shared with co-owner", err)
	}
	approveLibrary(t, s, admin, homeID)
	if _, _, err := s.DownloadDocument(ctx, joint, homeID); err != nil {
		t.Fatal("approved home access", err)
	}
	if _, err := s.DocumentFor(ctx, joint, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("named-person document inherited by co-owner", err)
	}
	in.OperationKey = randomToken()
	in.Title = "Fictional society circular"
	in.Category = "CIRCULAR"
	in.Visibility = "ALL_AUTHORIZED_RESIDENTS"
	in.FlatID = ""
	shared := checkedLibrary(t, s, owner, in, data)
	if _, err := s.DocumentFor(ctx, tenant, shared, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("unapproved public file leaked", err)
	}
	approveLibrary(t, s, admin, shared)
	if _, _, err := s.DownloadDocument(ctx, tenant, shared); err != nil {
		t.Fatal("approved society document", err)
	}
	// Ending only this home must remove its document access even while another home stays active.
	if _, err := s.DB.Exec(`UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'`, today()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.DownloadDocument(ctx, owner, homeID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("former-home uploader retained scoped document access", err)
	}
	if _, err := s.DB.Exec(`UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'`, today()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.DownloadDocument(ctx, owner, id); err != nil {
		t.Fatal("named-person retained history", err)
	}
	// The author retains their own upload history, but their former co-owner loses current-home access.
	if _, err := s.DB.Exec(`UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-joint-owner'`, today()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DocumentFor(ctx, joint, homeID, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended co-owner still sees home file", err)
	}
	in, _ = documentFixtureInput()
	if _, err := s.ReserveDocument(ctx, owner, in); !errors.Is(err, ErrForbidden) {
		t.Fatal("ended author uploaded", err)
	}
}

func TestDocumentAccountingScopeIsSeparateFromRegistryAndUploadsDoNotPostMoney(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	committee := reviewLogin(t, s, "committee@demo.society")
	in, data := documentFixtureInput()
	in.Category = "INVOICE"
	in.Visibility = "ACCOUNTING_ONLY"
	if _, err := s.ReserveDocument(ctx, admin, in); !errors.Is(err, ErrForbidden) {
		t.Fatal("registry role obtained accounting", err)
	}
	id := checkedLibrary(t, s, committee, in, data)
	for _, token := range []string{admin, owner, tenant} {
		if _, err := s.DocumentFor(ctx, token, id, 1); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("restricted accounting filename leaked", err)
		}
	}
	if err := s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	approveLibrary(t, s, admin, id)
	in.OperationKey = randomToken()
	in.Category = "PAYMENT_EVIDENCE"
	in.Visibility = "FLAT_SPECIFIC"
	in.FlatID = "demo-flat-A-101"
	in.Title = "Fictional payment evidence"
	evidence := checkedLibrary(t, s, owner, in, data)
	approveLibrary(t, s, committee, evidence)
	if _, _, err := s.DownloadDocument(ctx, owner, evidence); err != nil {
		t.Fatal(err)
	}
	in.OperationKey = randomToken()
	in.FlatID = "demo-flat-A-103"
	if _, err := s.ReserveDocument(ctx, tenant, in); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("unentitled tenant supplied financial evidence", err)
	}
	for _, table := range []string{"entries", "receipts"} {
		var count int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("upload posted money", table, count, err)
		}
	}
}

func TestDocumentReplacementPreservesApprovedOriginalUntilSeparateReviewAndArchive(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	in, data := documentFixtureInput()
	in.Category = "CIRCULAR"
	in.Visibility = "ALL_AUTHORIZED_RESIDENTS"
	in.Title = "Fictional original circular"
	original := checkedLibrary(t, s, owner, in, data)
	approveLibrary(t, s, admin, original)
	old := mustLibrary(t, s, owner, original)
	in.OperationKey = randomToken()
	in.Replaces = original
	in.Version = old.Version
	in.Title = "Fictional revised circular"
	in.Filename = "revision.png"
	replacement := checkedLibrary(t, s, owner, in, data)
	page, err := s.DocumentsFor(ctx, tenant, "circular", "", "", 1)
	if err != nil || page.Total != 1 || page.Items[0].ID != original {
		t.Fatal("pending replacement displaced original", page, err)
	}
	public := mustLibrary(t, s, tenant, original)
	if public.HistoryTotal != 1 || len(public.Versions) != 1 {
		t.Fatal("pending version leaked", public)
	}
	approveLibrary(t, s, admin, replacement)
	page, err = s.DocumentsFor(ctx, tenant, "circular", "", "", 1)
	if err != nil || page.Total != 1 || page.Items[0].ID != replacement || page.Items[0].Revision != 2 {
		t.Fatal("replacement publication", page, err)
	}
	if _, got, err := s.DownloadDocument(ctx, tenant, original); err != nil || !bytes.Equal(got, data) {
		t.Fatal("original approved version lost", err)
	}
	for _, query := range []string{"UPDATE local_document_objects SET original_bytes=x'00' WHERE document_id=?", "DELETE FROM local_document_objects WHERE document_id=?", "UPDATE library_documents SET title='rewritten original' WHERE id=?", "DELETE FROM document_events WHERE document_id=?", "DELETE FROM library_documents WHERE id=?"} {
		if _, err := s.DB.Exec(query, original); err == nil {
			t.Fatal("original/history was mutable", query)
		}
	}
	action := libraryAction(mustLibrary(t, s, admin, replacement), "ARCHIVED")
	for i := 0; i < 2; i++ {
		if _, err := s.DecideDocument(ctx, admin, replacement, action); err != nil {
			t.Fatal("archive replay", err)
		}
	}
	for _, id := range []string{original, replacement} {
		if _, err := s.DocumentFor(ctx, tenant, id, 1); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("archive allowed fresh resident link", err)
		}
	}
	if _, got, err := s.DownloadDocument(ctx, admin, original); err != nil || !bytes.Equal(got, data) {
		t.Fatal("archive erased original", err)
	}
}

func TestDocumentChecksumRetriesStaleRaceAndCurrentPermissionsAreAtomic(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	committee := reviewLogin(t, s, "committee@demo.society")
	in, data := documentFixtureInput()
	id, err := s.ReserveDocument(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.ReserveDocument(ctx, owner, in)
	if err != nil || replay != id {
		t.Fatal("reservation duplicated", replay, err)
	}
	if err = s.CompleteDocument(ctx, owner, id, append(data, 1)); !errors.Is(err, ErrInvalid) {
		t.Fatal("wrong checksum accepted", err)
	}
	if _, _, err = s.DownloadDocument(ctx, owner, id); !errors.Is(err, ErrConflict) {
		t.Fatal("unvalidated file downloaded", err)
	}
	if err = s.CompleteDocument(ctx, owner, id, data); err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, owner, id, data); err != nil {
		t.Fatal("completion retry", err)
	}
	job, err := s.ClaimDocumentCheck(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDocumentCheck(ctx, job, "image/png", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	action := libraryAction(mustLibrary(t, s, admin, id), "APPROVED")
	var wg sync.WaitGroup
	errorsOut := make(chan error, 2)
	for _, token := range []string{admin, committee} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			a := action
			a.OperationKey = randomToken()
			_, e := s.DecideDocument(ctx, token, id, a)
			errorsOut <- e
		}(token)
	}
	wg.Wait()
	close(errorsOut)
	success, conflict := 0, 0
	for e := range errorsOut {
		if e == nil {
			success++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("stale decision race", success, conflict)
	}
	in.OperationKey = randomToken()
	in.Title = strings.Repeat("आ", 120)
	id2 := checkedLibrary(t, s, owner, in, data)
	x := mustLibrary(t, s, admin, id2)
	action = libraryAction(x, "APPROVED")
	for i := 0; i < 2; i++ {
		if _, err = s.DecideDocument(ctx, admin, id2, action); err != nil {
			t.Fatal("decision replay", err)
		}
	}
	if _, err = s.DB.Exec(`UPDATE role_grants SET revoked_at=? WHERE user_id='demo-user-admin'`, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DecideDocument(ctx, admin, id2, action); err == nil {
		t.Fatal("revoked reviewer replayed decision")
	}
}

func TestDocumentQuotasExpiredReservationsAndValidationLeasesAreBounded(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	in, data := documentFixtureInput()
	in.Size = MaxUploadBytes
	for i := 0; i < 5; i++ {
		in.OperationKey = randomToken()
		if _, err := s.ReserveDocument(ctx, owner, in); err != nil {
			t.Fatal(err)
		}
	}
	in.OperationKey = randomToken()
	if _, err := s.ReserveDocument(ctx, owner, in); !errors.Is(err, ErrInvalid) {
		t.Fatal("quota bypassed by pending uploads", err)
	}
	if _, err := s.ClaimDocumentCheck(ctx, time.Now().Add(25*time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expiry check", err)
	}
	in, data = documentFixtureInput()
	id, err := s.ReserveDocument(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CompleteDocument(ctx, owner, id, data); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first, err := s.ClaimDocumentCheck(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ClaimDocumentCheck(ctx, now.Add(29*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("live lease stolen", err)
	}
	second, err := s.ClaimDocumentCheck(ctx, now.Add(31*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.FinishDocumentCheck(ctx, first, "image/png", "", now); !errors.Is(err, ErrConflict) {
		t.Fatal("stale worker overwrote new lease", err)
	}
	if err = s.FinishDocumentCheck(ctx, second, "", "CHECKS_UNAVAILABLE", now); err != nil {
		t.Fatal(err)
	}
	x := mustLibrary(t, s, admin, id)
	if !x.CanRetry {
		t.Fatal("unavailable checks cannot retry", x)
	}
	if _, err = s.DecideDocument(ctx, admin, id, libraryAction(x, "RETRY_VALIDATION")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err = s.ClaimDocumentCheck(ctx, now.Add(time.Duration(i)*31*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.ClaimDocumentCheck(ctx, now.Add(94*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("exhausted lease retried", err)
	}
	x = mustLibrary(t, s, owner, id)
	if x.Validation != "REJECTED" || x.CanRetry {
		t.Fatal("crashed validation stayed stuck", x)
	}
}

func TestDocumentContractExpiryAcceptsFutureCalendarDatesWithoutChangingPastRecordRules(t *testing.T) {
	s, _ := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	in, _ := documentFixtureInput()
	in.Category = "CONTRACT"
	in.Expiry = "2027-10-04"
	if _, err := s.ReserveDocument(ctx, owner, in); err != nil {
		t.Fatal("future contract expiry rejected", err)
	}
	for _, date := range []string{"2027-02-30", "2027-2-01", "2101-01-01"} {
		in.OperationKey = randomToken()
		in.Expiry = date
		if _, err := s.ReserveDocument(ctx, owner, in); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid expiry accepted", date, err)
		}
	}
	if validDate("2027-10-04") {
		t.Fatal("past financial/registry date rule changed")
	}
}
