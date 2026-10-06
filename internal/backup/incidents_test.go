package backup

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"testing"
	"time"
)

func TestIncidentRecoveryRetainsOriginalAndMetadataFreePreviewPrivateReviewNoticeResponseAndNoDebt(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if e := s.SeedDemoAccounts(ctx); e != nil {
		t.Fatal(e)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	a := upkeepRecoveryLogin(t, s, "admin@demo.society")
	b := upkeepRecoveryLogin(t, s, "committee@demo.society")
	tenant := upkeepRecoveryLogin(t, s, "tenant@demo.society")
	owner := upkeepRecoveryLogin(t, s, "owner@demo.society")
	ruleInput := database.RuleInput{OperationKey: "recovery-rule-proposal-12345", Title: "Supplied shared corridor policy", Text: "A fictional supplied policy preserves clear shared access.", PolicyReference: "Supplied fictional resolution R-01", EffectiveFrom: "2026-01-01", FinePermitted: true, Confirmed: true}
	rule, e := s.CreateRule(ctx, a, ruleInput)
	if e != nil {
		t.Fatal(e)
	}
	publish := database.RuleAction{OperationKey: "recovery-rule-publish-12345", Version: 1, Action: "PUBLISHED", Reason: "Separately checked the supplied fictional rule authority", Confirmed: true}
	if _, e = s.DecideRule(ctx, b, rule, publish); e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 16, 12)), nil); e != nil {
		t.Fatal(e)
	}
	raw := encoded.Bytes()
	private := []byte("Exif\x00\x00PRIVATE device and GPS metadata")
	segment := append([]byte{0xff, 0xe1, 0, byte(len(private) + 2)}, private...)
	original := append(append(append([]byte{}, raw[:2]...), segment...), raw[2:]...)
	pic, e := s.UploadIncidentPicture(ctx, tenant, "recovery-picture-12345", "scene.jpg", original)
	if e != nil {
		t.Fatal(e)
	}
	reportInput := database.IncidentInput{OperationKey: "recovery-incident-12345", RuleID: rule, FlatID: "demo-flat-A-101", IncidentDate: "2026-01-02", Comment: "PRIVATE fictional reporter observation and evidence detail.", PictureID: pic.ID, Confirmed: true}
	id, e := s.SaveIncident(ctx, tenant, "", reportInput)
	if e != nil {
		t.Fatal(e)
	}
	act := func(store *database.Store, token, action string, extra database.IncidentAction) {
		t.Helper()
		x, e := store.IncidentFor(ctx, token, id, 1)
		if e != nil {
			t.Fatal(e)
		}
		extra.OperationKey = "recovery-incident-" + action + "-12345"
		extra.Version = x.Version
		extra.Action = action
		extra.Reason = "PRIVATE independent review of supplied fictional evidence"
		extra.Confirmed = true
		if _, e = store.ActOnIncident(ctx, token, id, extra); e != nil {
			t.Fatal(action, e)
		}
	}
	act(s, a, "NOTE", database.IncidentAction{})
	act(s, b, "UNDER_REVIEW", database.IncidentAction{})
	act(s, b, "ISSUE_NOTICE", database.IncidentAction{Title: "Please supply your household account", Body: "A fictional corridor observation needs your household response.", ResponseBy: time.Now().AddDate(0, 0, 4).Format("2006-01-02")})
	detail, e := s.IncidentFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	responseInput := database.IncidentResponseInput{OperationKey: "recovery-subject-response-12345", Version: 1, Body: "PRIVATE household response retained only for this person and handlers.", Confirmed: true}
	response, e := s.RespondToIncident(ctx, owner, detail.NoticeID, responseInput)
	if e != nil {
		t.Fatal(e)
	}
	detail, e = s.IncidentFor(ctx, a, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	own, e := s.IncidentFor(ctx, tenant, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	notice, e := s.IncidentNoticeFor(ctx, owner, detail.NoticeID, 1)
	if e != nil {
		t.Fatal(e)
	}
	preview, e := s.IncidentPictureFor(ctx, owner, pic.ID, false)
	if e != nil || bytes.Contains(preview.Bytes, private) {
		t.Fatal("safe preview", e)
	}
	bundle := filepath.Join(root, "incident-snapshot")
	if _, e = Snapshot(ctx, s, bundle, "incident-test"); e != nil {
		t.Fatal(e)
	}
	act(s, b, "REMOVE_NOTICE", database.IncidentAction{})
	act(s, a, "SUBSTANTIATED", database.IncidentAction{})
	if _, e = s.DecideRule(ctx, a, rule, database.RuleAction{OperationKey: "recovery-later-retirement-12345", Version: 2, Action: "RETIRED", Reason: "A later supplied retirement must be outside the snapshot", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(root, "restored-incidents.db")
	if _, e = Restore(ctx, bundle, target); e != nil {
		t.Fatal(e)
	}
	restored, e := database.Open(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if e = restored.VerifyMFAKey(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = restored.CheckSession(ctx, a); !errors.Is(e, database.ErrUnauthenticated) {
		t.Fatal("old session revived", e)
	}
	current := upkeepRecoveryLogin(t, restored, "admin@demo.society")
	reporter := upkeepRecoveryLogin(t, restored, "tenant@demo.society")
	subject := upkeepRecoveryLogin(t, restored, "owner@demo.society")
	got, e := restored.IncidentFor(ctx, current, id, 1)
	if e != nil || !reflect.DeepEqual(got, detail) {
		t.Fatal("private snapshot restore", got, e)
	}
	got, e = restored.IncidentFor(ctx, reporter, id, 1)
	if e != nil || !reflect.DeepEqual(got, own) {
		t.Fatal("reporter snapshot restore", got, e)
	}
	gotNotice, e := restored.IncidentNoticeFor(ctx, subject, detail.NoticeID, 1)
	if e != nil || !reflect.DeepEqual(gotNotice, notice) {
		t.Fatal("notice/response restore", gotNotice, e)
	}
	gotPicture, e := restored.IncidentPictureFor(ctx, reporter, pic.ID, true)
	if e != nil || !bytes.Equal(gotPicture.Bytes, original) || gotPicture.SHA256 != pic.SHA256 {
		t.Fatal("original restore", e)
	}
	gotPreview, e := restored.IncidentPictureFor(ctx, subject, pic.ID, false)
	if e != nil || !bytes.Equal(gotPreview.Bytes, preview.Bytes) {
		t.Fatal("preview restore", e)
	}
	if _, e = restored.IncidentPictureFor(ctx, subject, pic.ID, true); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject original after restore", e)
	}
	if _, e = restored.IncidentFor(ctx, subject, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject private case after restore", e)
	}
	if again, e := restored.SaveIncident(ctx, reporter, "", reportInput); e != nil || again != id {
		t.Fatal("report replay", e)
	}
	if again, e := restored.RespondToIncident(ctx, subject, detail.NoticeID, responseInput); e != nil || again != response {
		t.Fatal("response replay", e)
	}
	for _, table := range []string{"entries", "receipts"} {
		var n int
		if e = restored.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); e != nil || n != 0 {
			t.Fatal("restored report creates debt", table, n, e)
		}
	}
}
