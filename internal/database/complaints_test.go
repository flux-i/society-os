package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func complaintInput() ComplaintInput {
	return ComplaintInput{OperationKey: randomToken(), FlatID: "demo-flat-A-101", Category: "PLUMBING", Subject: "Fictional leaking kitchen pipe", Description: "A slow fictional leak needs a visit and an update from the handler.", Priority: "NORMAL"}
}
func complaintAction(version int, action, message string) ComplaintAction {
	return ComplaintAction{OperationKey: randomToken(), Version: version, Action: action, Message: message, Visibility: "RESIDENT_VISIBLE"}
}
func mustComplaint(t *testing.T, s *Store, token, id string) ComplaintDetail {
	t.Helper()
	c, err := s.ComplaintFor(context.Background(), token, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func mustComplaintUpdate(t *testing.T, s *Store, token, id string, in ComplaintAction) {
	t.Helper()
	if _, err := s.UpdateComplaint(context.Background(), token, id, in); err != nil {
		t.Fatal(err)
	}
}

func TestComplaintOwnershipDoesNotFollowSharedFlatAndEndedMembersRetainReadOnlyHistory(t *testing.T) {
	s, _ := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	tenant := reviewLogin(t, s, "tenant@demo.society")
	in := complaintInput()
	id, err := s.CreateComplaint(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	_, err = s.DB.Exec(`INSERT INTO users(id,resident_id,login,display_name,password_hash,created_at,verified_at,password_changed_at,is_demo) VALUES('joint-case-user','demo-joint-owner','joint-case@demo.society','Demo Joint Case Owner',?,?,?,?,1)`, HashPassword(DemoPassword), now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	joint := reviewLogin(t, s, "joint-case@demo.society")
	for _, other := range []string{joint, tenant} {
		if _, err = s.ComplaintFor(ctx, other, id, 1); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("another author saw personal case", err)
		}
		list, e := s.ComplaintsFor(ctx, other, "leaking", "", 1)
		if e != nil || list.Total != 0 {
			t.Fatal("search exposed another case", list, e)
		}
		if _, err = s.UpdateComplaint(ctx, other, id, complaintAction(1, "COMMENT", "Another person tried to add a message")); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal("another author wrote to case", err)
		}
	}
	in.OperationKey = randomToken()
	in.FlatID = "demo-flat-B-101"
	if _, err = s.CreateComplaint(ctx, owner, in); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign home report accepted", err)
	}
	if _, err = s.DB.Exec("UPDATE flat_memberships SET end_date=? WHERE resident_id='demo-owner-A-101'", today()); err != nil {
		t.Fatal(err)
	}
	c := mustComplaint(t, s, owner, id)
	if c.CanParticipate || c.Number == "" {
		t.Fatal("ended member history policy", c)
	}
	if _, err = s.UpdateComplaint(ctx, owner, id, complaintAction(1, "COMMENT", "An ended member tried to reopen the conversation")); !errors.Is(err, ErrForbidden) {
		t.Fatal("ended member could write", err)
	}
	in = complaintInput()
	if _, err = s.CreateComplaint(ctx, owner, in); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("ended member submitted another report", err)
	}
}

func TestComplaintTransitionsResidentClosureReopeningAndHistoryPreservation(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	id, err := s.CreateComplaint(ctx, owner, complaintInput())
	if err != nil {
		t.Fatal(err)
	}
	bad := complaintAction(1, "STATUS", "Trying to skip resolution before closing")
	bad.Status = "CLOSED"
	if _, err = s.UpdateComplaint(ctx, admin, id, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("unresolved case closed", err)
	}
	if c := mustComplaint(t, s, owner, id); c.Version != 1 || c.HistoryTotal != 1 {
		t.Fatal("rejected transition changed history", c)
	}
	for i, state := range []string{"ACKNOWLEDGED", "IN_PROGRESS", "WAITING", "IN_PROGRESS", "RESOLVED"} {
		in := complaintAction(i+1, "STATUS", "A clear fictional progress and waiting explanation")
		in.Status = state
		mustComplaintUpdate(t, s, admin, id, in)
	}
	close := complaintAction(6, "STATUS", "The resident checked and confirmed the repair")
	close.Status = "CLOSED"
	for i := 0; i < 2; i++ {
		mustComplaintUpdate(t, s, owner, id, close)
	}
	c := mustComplaint(t, s, owner, id)
	if c.Status != "CLOSED" || c.Version != 7 || c.HistoryTotal != 7 || c.ResolvedAt == 0 || c.ClosedAt == 0 {
		t.Fatal("closure/retry history", c)
	}
	if _, err = s.UpdateComplaint(ctx, owner, id, complaintAction(7, "COMMENT", "This message needs the case to reopen")); !errors.Is(err, ErrInvalid) {
		t.Fatal("closed conversation changed", err)
	}
	reopen := complaintAction(7, "STATUS", "The leak returned after the original repair")
	reopen.Status = "OPEN"
	mustComplaintUpdate(t, s, owner, id, reopen)
	c = mustComplaint(t, s, owner, id)
	if c.Status != "OPEN" || c.Version != 8 || c.ResolvedAt != 0 || c.ClosedAt != 0 {
		t.Fatal("reopen did not reset active lifecycle", c)
	}
	for _, query := range []string{"DELETE FROM complaints WHERE id=?", "UPDATE complaints SET subject='rewritten report' WHERE id=?", "UPDATE complaints SET status='CLOSED' WHERE id=?", "DELETE FROM complaint_updates WHERE complaint_id=?", "UPDATE complaint_updates SET message='rewritten evidence' WHERE complaint_id=?"} {
		if _, err = s.DB.Exec(query, id); err == nil {
			t.Fatal("preserved history mutated", query)
		}
	}
	var count int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM entries").Scan(&count); err != nil || count != 0 {
		t.Fatal("service request changed money", count, err)
	}
}

func TestComplaintStaffNotesAreHiddenBeforeHistoryPaginationAndSearch(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	id, err := s.CreateComplaint(ctx, owner, complaintInput())
	if err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 33; version++ {
		in := complaintAction(version, "COMMENT", fmt.Sprintf("STAFF_SECRET_OnlyHandler private message %d", version))
		in.Visibility = "STAFF_ONLY"
		mustComplaintUpdate(t, s, admin, id, in)
	}
	hidden := mustComplaint(t, s, owner, id)
	if hidden.Version != 1 || hidden.UpdatedAt != hidden.CreatedAt || len(hidden.Updates[0].ID) != 43 {
		t.Fatal("private-note activity leaked in public metadata", hidden)
	}
	public := complaintAction(34, "COMMENT", "The resident can see this actual visit update")
	mustComplaintUpdate(t, s, admin, id, public)
	c := mustComplaint(t, s, owner, id)
	bytes, _ := json.Marshal(c)
	if c.HistoryTotal != 2 || c.Version != 2 || len(c.Updates) != 2 || strings.Contains(string(bytes), "STAFF_SECRET") || len(c.Handlers) != 0 {
		t.Fatal("private history/count/handlers exposed", string(bytes))
	}
	last, err := s.ComplaintFor(ctx, owner, id, 999)
	if err != nil || last.HistoryPage != 1 || len(last.Updates) != 2 {
		t.Fatal("private notes distorted resident pagination", last, err)
	}
	staff := mustComplaint(t, s, admin, id)
	if staff.HistoryTotal != 35 || len(staff.Updates) != 30 || !strings.Contains(staff.Updates[0].Message, "STAFF_SECRET") {
		t.Fatal("staff history pagination", staff)
	}
	older, err := s.ComplaintFor(ctx, admin, id, 2)
	if err != nil || len(older.Updates) != 5 || older.Updates[0].Action != "CREATED" {
		t.Fatal("older history lost", older, err)
	}
	for _, token := range []string{owner, admin} {
		list, e := s.ComplaintsFor(ctx, token, "STAFF_SECRET", "", 1)
		if e != nil || list.Total != 0 {
			t.Fatal("note indexed into case search", list, e)
		}
	}
	bad := complaintAction(35, "COMMENT", "Resident tried to conceal a staff message")
	bad.Visibility = "STAFF_ONLY"
	if _, err = s.UpdateComplaint(ctx, owner, id, bad); !errors.Is(err, ErrForbidden) {
		t.Fatal("resident created staff note", err)
	}
	mustComplaintUpdate(t, s, owner, id, complaintAction(2, "COMMENT", "The author can respond using the public conversation version"))
	if c = mustComplaint(t, s, owner, id); c.Version != 3 || c.HistoryTotal != 3 {
		t.Fatal("private notes interfered with public participation", c)
	}
}

func TestComplaintAssignmentRequiresCurrentEligibleHandlerAndFreshPrivilege(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	id, err := s.CreateComplaint(ctx, owner, complaintInput())
	if err != nil {
		t.Fatal(err)
	}
	assign := complaintAction(1, "ASSIGN", "Committee member will coordinate the visit")
	assign.AssignedTo = "demo-user-committee"
	mustComplaintUpdate(t, s, admin, id, assign)
	c := mustComplaint(t, s, owner, id)
	if c.AssignedTo != assign.AssignedTo || c.AssignedName == "" {
		t.Fatal("assignment lost", c)
	}
	bad := complaintAction(2, "ASSIGN", "Resident is not an authorized handler")
	bad.AssignedTo = "demo-user-owner"
	if _, err = s.UpdateComplaint(ctx, admin, id, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("ineligible assignment", err)
	}
	if _, err = s.DB.Exec("UPDATE users SET status='DISABLED' WHERE id='demo-user-committee'"); err != nil {
		t.Fatal(err)
	}
	unassign := complaintAction(2, "ASSIGN", "Handler unavailable; leave this open for reassignment")
	mustComplaintUpdate(t, s, admin, id, unassign)
	bad = complaintAction(3, "ASSIGN", "Disabled committee member cannot take new work")
	bad.AssignedTo = "demo-user-committee"
	if _, err = s.UpdateComplaint(ctx, admin, id, bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("disabled handler assigned", err)
	}
	if _, err = s.DB.Exec("UPDATE sessions SET reauthenticated_at=? WHERE user_id='demo-user-admin'", time.Now().Add(-6*time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	priority := complaintAction(3, "PRIORITY", "Fresh verification is needed for privileged changes")
	priority.Priority = "HIGH"
	if _, err = s.UpdateComplaint(ctx, admin, id, priority); !errors.Is(err, ErrReauthRequired) {
		t.Fatal("stale privilege accepted", err)
	}
	if _, err = s.DB.Exec("UPDATE role_grants SET valid_from=?,valid_until=? WHERE user_id='demo-user-admin'", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Second).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ComplaintFor(ctx, admin, id, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired handler read another person's case", err)
	}
}

func TestComplaintCreateRetryConcurrencyAndBoundedUnicode(t *testing.T) {
	s, admin := adminFixture(t)
	ctx := context.Background()
	owner := reviewLogin(t, s, "owner@demo.society")
	in := complaintInput()
	in.Description = strings.Repeat("आ", 3500)
	id, err := s.CreateComplaint(ctx, owner, in)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		again, e := s.CreateComplaint(ctx, owner, in)
		if e != nil || again != id {
			t.Fatal("create retry changed identity", again, e)
		}
	}
	in.Subject = "Changed content under an existing retry identity"
	if _, err = s.CreateComplaint(ctx, owner, in); !errors.Is(err, ErrConflict) {
		t.Fatal("retry substitution accepted", err)
	}
	var counter int
	if err = s.DB.QueryRow("SELECT last_number FROM complaint_counters").Scan(&counter); err != nil || counter != 1 {
		t.Fatal("retry consumed another case number", counter, err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.UpdateComplaint(ctx, admin, id, complaintAction(1, "COMMENT", "Racing handler update must preserve one winning version"))
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	passed, conflict := 0, 0
	for e := range results {
		if e == nil {
			passed++
		} else if errors.Is(e, ErrConflict) {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if passed != 1 || conflict != 1 {
		t.Fatal("concurrent writes", passed, conflict)
	}
	if c := mustComplaint(t, s, owner, id); c.Version != 2 || c.HistoryTotal != 2 {
		t.Fatal("racing history count", c)
	}
	for _, bad := range []ComplaintInput{{OperationKey: randomToken(), FlatID: "demo-flat-A-101", Category: "PLUMBING|LIFT", Subject: in.Subject, Description: in.Description, Priority: "NORMAL"}, {OperationKey: randomToken(), FlatID: "demo-flat-A-101", Category: "PLUMBING", Subject: in.Subject, Description: strings.Repeat("आ", 4001), Priority: "NORMAL"}} {
		if _, err = s.CreateComplaint(ctx, owner, bad); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid category/body accepted", err)
		}
	}
}
