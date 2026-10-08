package database

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMoveChecklistDatabaseRetainsIdentityOriginalsExactEventsAndDifferentReviewer(t *testing.T) {
	s, a, _, o := moveFixture(t)
	ctx := context.Background()
	id, e := s.SubmitMoveChecklist(ctx, o, moveInput())
	if e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"DELETE FROM move_checklist_resources WHERE id=?", "DELETE FROM move_checklist_versions WHERE resource_id=?", "DELETE FROM move_checklist_events WHERE resource_id=?", "UPDATE move_checklist_versions SET reason='An altered original source' WHERE resource_id=?", "UPDATE move_checklist_events SET reason='An altered decision reason' WHERE resource_id=?", "UPDATE move_checklist_resources SET flat_id='demo-flat-A-103' WHERE id=?", "UPDATE move_checklist_resources SET version=version+1,latest_version=version+1,pending_version=version+1 WHERE id=?"} {
		if _, e = s.DB.Exec(query, id); e == nil {
			t.Fatal("raw mutation bypassed retained history", query)
		}
	}
	principal, e := s.CheckSession(ctx, a)
	if e != nil {
		t.Fatal(e)
	}
	x := moveDetail(t, s, a, id).Snapshot
	x.Version = 2
	x.Action = "CHECK"
	x.ActorID = principal.ID
	x.OccurredAt = time.Now().Unix()
	x.Reason = "Independent raw malformed evidence must be rejected."
	x.Checks = x.Checks[:4]
	raw, e := json.Marshal(x)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO move_checklist_versions VALUES(?,?,?,?,?,?,?)", id, 2, "CHECK", string(raw), x.ActorID, x.OccurredAt, x.Reason); e == nil {
		t.Fatal("missing check accepted")
	}
	for _, kind := range moveCheckKinds {
		moveCheck(t, s, a, id, kind)
	}
	moveReady(t, s, a, id)
	ready := moveDetail(t, s, a, id)
	x = ready.Snapshot
	x.Version = 8
	x.Action = "APPROVED"
	x.Phase = "COMPLETED"
	x.ActorID = principal.ID
	x.OccurredAt = time.Now().Unix()
	x.Reason = "Independent raw self approval must remain forbidden."
	raw, e = json.Marshal(x)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.DB.Exec("INSERT INTO move_checklist_versions VALUES(?,?,?,?,?,?,?)", id, 8, "APPROVED", string(raw), x.ActorID, x.OccurredAt, x.Reason); e == nil || !strings.Contains(e.Error(), "different reviewer") {
		t.Fatal("raw checking-submitter approval guard did not reject this actor", e)
	}
	if got := moveDetail(t, s, a, id); got.Version != 7 || got.EventTotal != 7 || got.Approved != nil {
		t.Fatal("rejected raw mutations changed heads", got)
	}
	if _, e = s.DB.Exec("INSERT INTO move_checklist_resources VALUES('raw-no-events','demo-flat-A-101','demo-owner-A-101','CONTACT_REVIEW','demo-user-owner',1,1,1,1,NULL,'CHECKING')"); e == nil {
		t.Fatal("resource committed without originals and events")
	}
}
