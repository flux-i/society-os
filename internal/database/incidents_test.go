package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func incidentRuleInput() RuleInput {
	return RuleInput{OperationKey: randomToken(), Title: "Keep shared access clear", Text: "Fictional policy: shared corridors must remain clear for access.", PolicyReference: "Fictional supplied committee resolution R-01", EffectiveFrom: "2026-01-01", FinePermitted: true, Confirmed: true}
}
func incidentFixture(t *testing.T) (*Store, string, string, string, string) {
	t.Helper()
	s, admin := adminFixture(t)
	owner := reviewLogin(t, s, "owner@demo.society")
	reviewer := maintenanceReviewer(t, s, admin)
	tenant := reviewLogin(t, s, "tenant@demo.society")
	return s, admin, reviewer, owner, tenant
}
func publishIncidentRule(t *testing.T, s *Store, a, b string) string {
	t.Helper()
	id, e := s.CreateRule(context.Background(), a, incidentRuleInput())
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.DecideRule(context.Background(), b, id, RuleAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Reason: "Separately checked the fictional policy text and authority", Confirmed: true})
	if e != nil {
		t.Fatal(e)
	}
	return id
}
func incidentInput(rule string) IncidentInput {
	return IncidentInput{OperationKey: randomToken(), RuleID: rule, FlatID: "demo-flat-A-102", IncidentDate: "2026-01-02", Comment: "A fictional obstruction was observed by the shared corridor.", Confirmed: true}
}
func incidentCase(t *testing.T, s *Store, token, id string) IncidentDetail {
	t.Helper()
	x, e := s.IncidentFor(context.Background(), token, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func incidentAction(t *testing.T, s *Store, token, id, action string) IncidentDetail {
	t.Helper()
	x := incidentCase(t, s, token, id)
	_, e := s.ActOnIncident(context.Background(), token, id, IncidentAction{OperationKey: randomToken(), Version: x.Version, Action: action, Reason: "Fictional evidence and response were deliberately reviewed", Confirmed: true})
	if e != nil {
		t.Fatal(action, e)
	}
	return incidentCase(t, s, token, id)
}
func incidentPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	m := image.NewRGBA(image.Rect(0, 0, 10, 8))
	m.Set(3, 4, image.White)
	if e := png.Encode(&b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}

func TestIncidentRulePublicationReplacementDatesAndNoFinancialEffects(t *testing.T) {
	s, a, b, o, _ := incidentFixture(t)
	ctx := context.Background()
	in := incidentRuleInput()
	id, e := s.CreateRule(ctx, a, in)
	if e != nil {
		t.Fatal(e)
	}
	action := RuleAction{OperationKey: randomToken(), Version: 1, Action: "PUBLISHED", Reason: "Checked supplied policy text with separate authority", Confirmed: true}
	if _, e = s.DecideRule(ctx, a, id, action); !errors.Is(e, ErrForbidden) {
		t.Fatal("self publication", e)
	}
	if _, e = s.SaveIncident(ctx, o, "", incidentInput(id)); e == nil {
		t.Fatal("unpublished rule accepted")
	}
	if _, e = s.DecideRule(ctx, b, id, action); e != nil {
		t.Fatal(e)
	}
	report, e := s.SaveIncident(ctx, o, "", incidentInput(id))
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, report, "SUBSTANTIATED")
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	replacement := incidentRuleInput()
	replacement.ReplacesID = id
	replacement.Text = "A supplied replacement policy preserves the prior version for old cases."
	next, e := s.CreateRule(ctx, b, replacement)
	if e != nil {
		t.Fatal(e)
	}
	action.OperationKey = randomToken()
	if _, e = s.DecideRule(ctx, a, next, action); e != nil {
		t.Fatal(e)
	}
	old, e := s.RuleFor(ctx, o, id, 1)
	if e != nil || old.State != "RETIRED" {
		t.Fatal(old, e)
	}
	if incidentCase(t, s, o, report).Rule.Text != in.Text {
		t.Fatal("old case text changed")
	}
	if _, e = s.SaveIncident(ctx, o, "", incidentInput(id)); e == nil {
		t.Fatal("retired rule accepted")
	}
	invalidInput := incidentInput(next)
	invalidInput.IncidentDate = "2025-12-31"
	if _, e = s.SaveIncident(ctx, o, "", invalidInput); e == nil {
		t.Fatal("before effective rule")
	}
	invalidInput.IncidentDate = "2099-01-01"
	if _, e = s.SaveIncident(ctx, o, "", invalidInput); e == nil {
		t.Fatal("future incident")
	}
	if _, e = s.DB.Exec("UPDATE society_rules SET rule_text='replacement' WHERE id=?", id); e == nil {
		t.Fatal("frozen rule changed")
	}
}

func TestIncidentScopesVersionsPrivatePictureNoticeAndResponses(t *testing.T) {
	s, a, b, o, tenant := incidentFixture(t)
	ctx := context.Background()
	rule := publishIncidentRule(t, s, a, b)
	bytes := incidentPNG(t)
	pic, e := s.UploadIncidentPicture(ctx, o, randomToken(), "scene.png", bytes)
	if e != nil {
		t.Fatal(e)
	}
	in := incidentInput(rule)
	in.PictureID = pic.ID
	id, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.IncidentFor(ctx, tenant, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("unrelated private case", e)
	}
	before := incidentCase(t, s, o, id)
	incidentAction(t, s, a, id, "NOTE")
	after := incidentCase(t, s, o, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("staff note changed reporter snapshot")
	}
	notice := IncidentAction{OperationKey: randomToken(), Version: incidentCase(t, s, a, id).Version, Action: "ISSUE_NOTICE", Reason: "Privately reviewed the case before requesting a response", Title: "Please clarify this observation", Body: "A fictional obstruction was observed. Please provide your account.", ResponseBy: today(), Confirmed: true}
	if _, e = s.ActOnIncident(ctx, b, id, notice); e != nil {
		t.Fatal(e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM entries", 0)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM receipts", 0)
	// Owner has both A-101 and A-102 in the synthetic fixture: notice views use current home scope, never finance permission.
	notices, e := s.IncidentNoticesFor(ctx, o, "", 1)
	if e != nil || notices.Total != 1 {
		t.Fatal(notices, e)
	}
	n, e := s.IncidentNoticeFor(ctx, o, notices.Items[0].ID, 1)
	if e != nil || n.Body != notice.Body {
		t.Fatal(n, e)
	}
	frozen := n
	incidentAction(t, s, a, id, "NOTE")
	n, e = s.IncidentNoticeFor(ctx, o, n.ID, 1)
	if e != nil || !reflect.DeepEqual(frozen, n) {
		t.Fatal("private change affected subject", e)
	}
	if _, e = s.IncidentPictureFor(ctx, tenant, pic.ID, false); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("unrelated picture", e)
	}
	answer := IncidentResponseInput{OperationKey: randomToken(), Version: n.Version, Body: "This is the responding resident's supplied fictional explanation.", Confirmed: true}
	response, e := s.RespondToIncident(ctx, o, n.ID, answer)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.RespondToIncident(ctx, o, n.ID, answer)
	if e != nil || again != response {
		t.Fatal("response retry", e)
	}
	incidentAction(t, s, b, id, "REMOVE_NOTICE")
	if _, e = s.IncidentNoticeFor(ctx, o, n.ID, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("removed notice visible", e)
	}
	if _, e = s.RespondToIncident(ctx, o, n.ID, answer); e == nil {
		t.Fatal("removed notice replay accepted")
	}
	handler := incidentCase(t, s, a, id)
	if len(handler.Notices) != 1 || handler.ResponseTotal != 1 {
		t.Fatal("retained notice/response", handler)
	}
}

func TestIncidentCurrentParticipationConcurrentDecisionAndRetryAuthority(t *testing.T) {
	s, a, b, o, _ := incidentFixture(t)
	ctx := context.Background()
	rule := publishIncidentRule(t, s, a, b)
	in := incidentInput(rule)
	id, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.SaveIncident(ctx, o, "", in)
	if e != nil || again != id {
		t.Fatal(e)
	}
	decision := IncidentAction{OperationKey: randomToken(), Version: 1, Action: "SUBSTANTIATED", Reason: "Supplied fictional evidence was independently checked", Confirmed: true}
	if _, e = s.ActOnIncident(ctx, o, id, decision); !errors.Is(e, ErrForbidden) {
		t.Fatal("reporter self decision", e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, token := range []string{a, b} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			copy := decision
			copy.OperationKey = randomToken()
			_, e := s.ActOnIncident(ctx, token, id, copy)
			errs <- e
		}(token)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	incidentAction(t, s, a, id, "REOPEN")
	decision.Version = incidentCase(t, s, b, id).Version
	if _, e = s.ActOnIncident(ctx, b, id, decision); e != nil {
		t.Fatal(e)
	}
	accessExec(t, s, "UPDATE role_grants SET revoked_at=1 WHERE user_id='demo-user-committee' AND role='COMMITTEE'")
	if _, e = s.ActOnIncident(ctx, b, id, decision); !errors.Is(e, ErrForbidden) {
		t.Fatal("revoked reviewer replay", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'", today())
	if _, e = s.SaveIncident(ctx, o, "", in); !errors.Is(e, ErrForbidden) {
		t.Fatal("ended reporter retry", e)
	}
}

func TestIncidentPictureOriginalChecksumMetadataRemovalLimitsAndSharedQuota(t *testing.T) {
	s, a, _, o, _ := incidentFixture(t)
	ctx := context.Background()
	var encoded bytes.Buffer
	if e := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1800, 900)), nil); e != nil {
		t.Fatal(e)
	}
	raw := encoded.Bytes()
	private := []byte("Exif\x00\x00PRIVATE GPS device uploader metadata")
	segment := append([]byte{0xff, 0xe1, byte((len(private) + 2) >> 8), byte(len(private) + 2)}, private...)
	original := append(append(append([]byte{}, raw[:2]...), segment...), raw[2:]...)
	key := randomToken()
	picture, e := s.UploadIncidentPicture(ctx, o, key, "camera.jpg", original)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.UploadIncidentPicture(ctx, o, key, "camera.jpg", original)
	if e != nil || again.ID != picture.ID {
		t.Fatal("picture retry", e)
	}
	if _, e = s.UploadIncidentPicture(ctx, o, key, "other.jpg", original); !errors.Is(e, ErrConflict) {
		t.Fatal("picture retry payload", e)
	}
	download, e := s.IncidentPictureFor(ctx, o, picture.ID, true)
	if e != nil || !bytes.Equal(download.Bytes, original) {
		t.Fatal("original", e)
	}
	preview, e := s.IncidentPictureFor(ctx, o, picture.ID, false)
	if e != nil || bytes.Contains(preview.Bytes, private) {
		t.Fatal("metadata in preview", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='INCIDENT_PICTURE_ORIGINAL_READ'", 1)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='INCIDENT_PICTURE_PREVIEW_READ'", 1)
	var accessActor, accessReason, accessJSON string
	if e = s.DB.QueryRow("SELECT actor_user_id,reason,after_json FROM audit_events WHERE action='INCIDENT_PICTURE_ORIGINAL_READ'").Scan(&accessActor, &accessReason, &accessJSON); e != nil {
		t.Fatal("original access audit", e)
	}
	if accessActor != "demo-user-owner" || !strings.Contains(accessJSON, picture.ID) || !strings.Contains(accessJSON, picture.SHA256) || strings.Contains(accessJSON+accessReason, "PRIVATE") || strings.Contains(accessJSON+accessReason, picture.Filename) {
		t.Fatal("access audit scope or private metadata", accessActor, accessReason, accessJSON)
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(preview.Bytes))
	if e != nil || cfg.Width != 1280 || cfg.Height != 640 {
		t.Fatal("preview dimensions", cfg, e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM incident_pictures", 1)
	for _, bad := range [][]byte{[]byte("<svg><script>alert(1)</script></svg>"), original[:25], make([]byte, MaxIncidentPictureBytes+1)} {
		if _, e = s.UploadIncidentPicture(ctx, o, randomToken(), "bad.jpg", bad); e == nil {
			t.Fatal("invalid picture accepted")
		}
	}
	if _, e = s.UploadIncidentPicture(ctx, o, randomToken(), "../scene.png", incidentPNG(t)); e == nil {
		t.Fatal("path filename")
	}
	if _, e = s.UploadIncidentPicture(ctx, o, randomToken(), "wrong.jpg", incidentPNG(t)); e == nil {
		t.Fatal("type mismatch")
	}
	if _, e = s.DB.Exec("UPDATE incident_pictures SET filename='changed.jpg' WHERE id=?", picture.ID); e == nil {
		t.Fatal("mutable evidence")
	}
	if _, e = s.DB.Exec("DELETE FROM incident_pictures WHERE id=?", picture.ID); e == nil {
		t.Fatal("deleted evidence")
	}
	own, total, e := documentStorageUsage(ctx, s.DB, "demo-user-owner")
	if e != nil || own != int64(len(original)+len(preview.Bytes)) || total != own {
		t.Fatal("picture quota", own, total, e)
	}
	if _, e = s.ReserveDocument(ctx, o, DocumentInput{OperationKey: randomToken(), Title: "A fictional private large reservation", Filename: "reserve.png", Size: MaxUploadBytes, SHA256: strings.Repeat("a", 64), Category: "PAYMENT_EVIDENCE", Visibility: "FLAT_SPECIFIC", FlatID: "demo-flat-A-101"}); e != nil {
		t.Fatal(e)
	}
	_, total, e = documentStorageUsage(ctx, s.DB, "demo-user-owner")
	if e != nil || total != own+MaxUploadBytes {
		t.Fatal("shared library quota", total, e)
	}
	reservation := DocumentInput{Title: "Private quota reservation", Filename: "quota.png", Size: MaxUploadBytes, SHA256: strings.Repeat("a", 64), Category: "PAYMENT_EVIDENCE", Visibility: "FLAT_SPECIFIC", FlatID: "demo-flat-A-101"}
	for i := 0; i < 3; i++ {
		reservation.OperationKey = randomToken()
		if _, e = s.ReserveDocument(ctx, o, reservation); e != nil {
			t.Fatal(e)
		}
	}
	reservation.OperationKey = randomToken()
	if _, e = s.ReserveDocument(ctx, o, reservation); e == nil {
		t.Fatal("picture bytes did not count toward library quota")
	}
	reservation.OperationKey = randomToken()
	reservation.Size = LocalUserDocumentQuota - own - 4*MaxUploadBytes - 1
	if _, e = s.ReserveDocument(ctx, o, reservation); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UploadIncidentPicture(ctx, o, randomToken(), "extra.png", incidentPNG(t)); e == nil {
		t.Fatal("pending library bytes did not count toward picture quota")
	}
	for _, dimensions := range [][2]uint32{{4097, 1}, {3000, 3000}} {
		large := append([]byte(nil), incidentPNG(t)...)
		binary.BigEndian.PutUint32(large[16:20], dimensions[0])
		binary.BigEndian.PutUint32(large[20:24], dimensions[1])
		binary.BigEndian.PutUint32(large[29:33], crc32.ChecksumIEEE(large[12:29]))
		if _, e = s.UploadIncidentPicture(ctx, a, randomToken(), "large.png", large); e == nil {
			t.Fatal("picture pixel bound", dimensions)
		}
	}
}

func TestIncidentSubjectCannotReadReporterOriginalOrOtherResidentsResponsesAndEndedHomeRevokes(t *testing.T) {
	s, a, b, o, tenant := incidentFixture(t)
	ctx := context.Background()
	rule := publishIncidentRule(t, s, a, b)
	pic, e := s.UploadIncidentPicture(ctx, tenant, randomToken(), "private-scene.png", incidentPNG(t))
	if e != nil {
		t.Fatal(e)
	}
	in := incidentInput(rule)
	in.FlatID = "demo-flat-A-101"
	in.PictureID = pic.ID
	in.Comment = "PRIVATE REPORTER detail should never appear in a subject notice."
	id, e := s.SaveIncident(ctx, tenant, "", in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.IncidentFor(ctx, o, id, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject private case", e)
	}
	x := incidentCase(t, s, b, id)
	action := IncidentAction{OperationKey: randomToken(), Version: x.Version, Action: "ISSUE_NOTICE", Reason: "PRIVATE officer evidence and identity must remain private", Title: "Please provide your account", Body: "A fictional corridor observation needs your response.", ResponseBy: today(), Confirmed: true}
	if _, e = s.ActOnIncident(ctx, b, id, action); e != nil {
		t.Fatal(e)
	}
	list, e := s.IncidentNoticesFor(ctx, o, "", 1)
	if e != nil || list.Total != 1 {
		t.Fatal(list, e)
	}
	n, e := s.IncidentNoticeFor(ctx, o, list.Items[0].ID, 1)
	if e != nil {
		t.Fatal(e)
	}
	serialized, _ := json.Marshal(n)
	for _, private := range []string{"PRIVATE", "reporter_id", "author_id", "notice_id", "Demo Tenant"} {
		if bytes.Contains(serialized, []byte(private)) {
			t.Fatal("subject leak", private, string(serialized))
		}
	}
	if _, e = s.IncidentPictureFor(ctx, o, pic.ID, true); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("subject original", e)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM audit_events WHERE action='INCIDENT_PICTURE_ORIGINAL_READ'", 0)
	if _, e = s.IncidentPictureFor(ctx, o, pic.ID, false); e != nil {
		t.Fatal("subject safe preview", e)
	}
	answer := IncidentResponseInput{OperationKey: randomToken(), Version: n.Version, Body: "PRIVATE SUBJECT response visible to this person and handlers only.", Confirmed: true}
	if _, e = s.RespondToIncident(ctx, o, n.ID, answer); e != nil {
		t.Fatal(e)
	}
	if _, e = s.IncidentNoticeFor(ctx, tenant, n.ID, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("reporter subject response", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET can_view_finances=0 WHERE resident_id='demo-owner-A-101'")
	if _, e = s.IncidentNoticeFor(ctx, o, n.ID, 1); e != nil {
		t.Fatal("notice depends on finance", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101' AND flat_id='demo-flat-A-101'", today())
	if _, e = s.IncidentNoticeFor(ctx, o, n.ID, 1); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended tagged home", e)
	}
	if _, e = s.IncidentPictureFor(ctx, o, pic.ID, false); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended preview", e)
	}
	if _, e = s.RespondToIncident(ctx, o, n.ID, answer); e == nil {
		t.Fatal("ended response replay", e)
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-tenant-A-103'", today())
	if _, e = s.IncidentPictureFor(ctx, tenant, pic.ID, true); !errors.Is(e, sql.ErrNoRows) {
		t.Fatal("ended reporter original", e)
	}
	if !reflect.DeepEqual(incidentCase(t, s, tenant, id).Comment, in.Comment) {
		t.Fatal("lost personal textual history")
	}
}

func TestIncidentCorrectionNoticeFreezeDuplicateWithdrawalAndImmutableHistory(t *testing.T) {
	s, a, b, o, _ := incidentFixture(t)
	ctx := context.Background()
	rule := publishIncidentRule(t, s, a, b)
	in := incidentInput(rule)
	id, e := s.SaveIncident(ctx, o, "", in)
	if e != nil {
		t.Fatal(e)
	}
	x := incidentCase(t, s, a, id)
	notice := IncidentAction{OperationKey: randomToken(), Version: x.Version, Action: "ISSUE_NOTICE", Reason: "Frozen response notice issued against the submitted observation", Title: "Please clarify this case", Body: "Supplied fictional notice asking for a household response.", ResponseBy: today(), Confirmed: true}
	if _, e = s.ActOnIncident(ctx, a, id, notice); e != nil {
		t.Fatal(e)
	}
	before := incidentCase(t, s, o, id)
	in.OperationKey = randomToken()
	in.Version = before.Version
	in.FlatID = "demo-flat-A-101"
	if _, e = s.SaveIncident(ctx, o, id, in); e == nil {
		t.Fatal("active notice tag silently changed")
	}
	incidentAction(t, s, b, id, "REMOVE_NOTICE")
	if _, e = s.SaveIncident(ctx, o, id, in); e != nil {
		t.Fatal(e)
	}
	changed := incidentCase(t, s, a, id)
	if changed.FlatID != "demo-flat-A-101" || changed.NoticeTotal != 1 {
		t.Fatal(changed)
	}
	if _, e = s.DB.Exec("UPDATE incident_events SET reason='changed' WHERE incident_id=?", id); e == nil {
		t.Fatal("mutable retained submission")
	}
	another := in
	another.Version = 0
	another.OperationKey = randomToken()
	dup, e := s.SaveIncident(ctx, o, "", another)
	if e != nil {
		t.Fatal(e)
	}
	action := IncidentAction{OperationKey: randomToken(), Version: 1, Action: "DUPLICATE", DuplicateOf: id, Reason: "Identified this as a duplicate of the same supplied observation", Confirmed: true}
	if _, e = s.ActOnIncident(ctx, b, dup, action); e != nil {
		t.Fatal(e)
	}
	if incidentCase(t, s, a, dup).DuplicateOf != id {
		t.Fatal("duplicate link lost")
	}
	reopened := incidentAction(t, s, a, dup, "REOPEN")
	if reopened.DuplicateOf != "" || reopened.State != "UNDER_REVIEW" {
		t.Fatal(reopened)
	}
	own := incidentCase(t, s, o, id)
	withdraw := IncidentAction{OperationKey: randomToken(), Version: own.Version, Action: "WITHDRAWN", Reason: "Reporter deliberately withdraws this unresolved fictional case", Confirmed: true}
	if _, e = s.ActOnIncident(ctx, o, id, withdraw); e != nil {
		t.Fatal(e)
	}
	if incidentCase(t, s, o, id).State != "WITHDRAWN" {
		t.Fatal("withdrawal")
	}
}

func TestIncidentOverviewIndependentCurrentAttentionMetadataAndPrivateSnapshot(t *testing.T) {
	s, a, b, o, tenant := incidentFixture(t)
	ctx := context.Background()
	rule := publishIncidentRule(t, s, a, b)
	pending, e := s.CreateRule(ctx, b, incidentRuleInput())
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.SaveIncident(ctx, tenant, "", incidentInput(rule))
	if e != nil {
		t.Fatal(e)
	}
	overview := func(token string) Overview {
		t.Helper()
		x, e := s.OverviewFor(ctx, token, "incidents")
		if e != nil {
			t.Fatal(e)
		}
		if len(x.Items) > 4 {
			t.Fatal("unbounded attention")
		}
		blob, _ := json.Marshal(x)
		if bytes.Contains(blob, []byte("PRIVATE")) {
			t.Fatal("private overview", string(blob))
		}
		return x
	}
	op := overview(a)
	if op.Counts["needs_review"] != 1 || op.Counts["pending_rules"] != 1 || op.Counts["responses_needed"] != 0 {
		t.Fatal(op)
	}
	if len(op.Items) != 2 || op.Items[0].ID != pending {
		t.Fatal("exact metadata links", op.Items)
	}
	incidentAction(t, s, a, id, "NEEDS_INFO")
	own := overview(tenant)
	if own.Counts["information_needed"] != 1 || len(own.Items) != 1 || own.Items[0].ID != id || own.Items[0].Kind != "INCIDENT_CLARIFICATION" {
		t.Fatal(own)
	}
	op = overview(a)
	if len(op.Items) != 2 || op.Items[0].ID != id || op.Items[0].Kind != "INCIDENT" {
		t.Fatal("staff cannot clarify another person's report", op.Items)
	}
	mine, e := s.SaveIncident(ctx, a, "", incidentInput(rule))
	if e != nil {
		t.Fatal(e)
	}
	incidentAction(t, s, b, mine, "NEEDS_INFO")
	gotOwn := false
	for _, item := range overview(a).Items {
		if item.ID == mine {
			gotOwn = item.Kind == "INCIDENT_CLARIFICATION"
		}
	}
	if !gotOwn {
		t.Fatal("an operational reporter still needs their own clarification action")
	}
	x := incidentCase(t, s, b, id)
	notice := IncidentAction{OperationKey: randomToken(), Version: x.Version, Action: "ISSUE_NOTICE", Title: "A supplied household response request", Body: "The supplied fictional observation requests your account.", ResponseBy: today(), Reason: "PRIVATE review prior to deliberately sharing a response notice", Confirmed: true}
	if _, e = s.ActOnIncident(ctx, b, id, notice); e != nil {
		t.Fatal(e)
	}
	subject := overview(o)
	if subject.Counts["responses_needed"] != 1 || subject.Counts["needs_review"] != 0 || subject.Counts["pending_rules"] != 0 || len(subject.Items) != 1 || subject.Items[0].Kind != "INCIDENT_NOTICE" {
		t.Fatal(subject)
	}
	incidentAction(t, s, a, id, "NOTE")
	later := overview(o)
	if !reflect.DeepEqual(subject.Items, later.Items) || !reflect.DeepEqual(subject.Counts, later.Counts) {
		t.Fatal("private note changed subject overview")
	}
	n := incidentCase(t, s, a, id).NoticeID
	if _, e = s.RespondToIncident(ctx, o, n, IncidentResponseInput{OperationKey: randomToken(), Version: 1, Body: "The current household supplies its fictional account for review.", Confirmed: true}); e != nil {
		t.Fatal(e)
	}
	if overview(o).Counts["responses_needed"] != 0 {
		t.Fatal("responded notice remains attention")
	}
	accessExec(t, s, "UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-tenant-A-103'", today())
	if overview(tenant).Counts["information_needed"] != 0 {
		t.Fatal("ended participation attention")
	}
}
