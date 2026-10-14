package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/security"
)

func legacyRecoveryReviewStore(t *testing.T, count int) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "actual-schema29.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err = s.DB.Exec("CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,checksum TEXT NOT NULL,applied_at TEXT NOT NULL) STRICT"); err != nil {
		t.Fatal(err)
	}
	for version := 1; version <= 29; version++ {
		body, e := migrationBody(version)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.DB.Exec(string(body)); e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(body)
		if _, e = s.DB.Exec("INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(sum[:]), "2026-10-12"); e != nil {
			t.Fatal(e)
		}
	}
	if err = s.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		id := strings.Repeat(string(rune('A'+i)), 43)
		if _, err = s.DB.Exec("INSERT INTO recovery_epochs VALUES(?,1,?,1585152,29,'0.32.0-dev',?,?)", id, strings.Repeat("a", 64), time.Now().Unix()-120, time.Now().Unix()-60); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestRecoveryReviewMigrationPreservesGenuineSchema29AndNeverGuessesAnAmbiguousEpoch(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count))+" legacy origins", func(t *testing.T) {
			s := legacyRecoveryReviewStore(t, count)
			ctx := context.Background()
			before := moneyRows(t, s)
			const provenanceQuery = `SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT version,checksum,applied_at FROM schema_migrations WHERE version<=29 ORDER BY version)`
			var oldProvenance string
			if err := s.DB.QueryRow(provenanceQuery).Scan(&oldProvenance); err != nil {
				t.Fatal(err)
			}
			definitions, err := s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name")
			if err != nil {
				t.Fatal(err)
			}
			old := map[string]string{}
			for definitions.Next() {
				var kind, name, body string
				if err = definitions.Scan(&kind, &name, &body); err != nil {
					t.Fatal(err)
				}
				old[kind+":"+name] = body
			}
			if err = definitions.Err(); err != nil {
				t.Fatal(err)
			}
			definitions.Close()
			if err = s.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			after := moneyRows(t, s)
			for table, rows := range before {
				if table == "schema_migrations" {
					continue
				}
				if after[table] != rows {
					t.Fatal("Upgrade altered a retained predecessor table or private BLOB", table)
				}
			}
			var retainedProvenance string
			if err = s.DB.QueryRow(provenanceQuery).Scan(&retainedProvenance); err != nil || retainedProvenance != oldProvenance {
				t.Fatal("Upgrade rewrote preceding migration provenance", err)
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM schema_migrations", SchemaVersion)
			if err = s.VerifySchema(ctx); err != nil {
				t.Fatal("New migration lacks exact current checksummed provenance", err)
			}
			for key, body := range old {
				parts := strings.SplitN(key, ":", 2)
				var current string
				if err = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", parts[0], parts[1]).Scan(&current); err != nil || current != body {
					t.Fatal("Upgrade changed an existing SQL object", key, err)
				}
			}
			maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_nominations", 0)
			maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_reviewers", 0)
			maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_proposals", 0)
			maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_decisions", 0)
			if count == 1 {
				origin, e := s.RecoveryOriginForCustody(ctx)
				if e != nil || origin.ID != strings.Repeat("A", 43) || origin.SourceSchema != 29 {
					t.Fatal("Single known legacy origin lost its exact binding", origin, e)
				}
				maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epoch_heads", 1)
			} else {
				maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epoch_heads", 0)
			}
			if count == 2 {
				if _, err = s.RecoveryOriginForCustody(ctx); !errors.Is(err, ErrRecoveryAmbiguous) {
					t.Fatal("Tied legacy timestamps were guessed as current authority", err)
				}
				origin := controlledRecoveryOrigin()
				origin.ID = strings.Repeat("B", 43)
				in, passwords := recoveryAppointment(t, s, origin)
				if _, err = s.NominateRecovery(ctx, in, passwords); !errors.Is(err, ErrRecoveryAmbiguous) {
					t.Fatal("Ambiguous old origin accepted authority", err)
				}
				if !reflect.DeepEqual(after, moneyRows(t, s)) {
					t.Fatal("Rejected ambiguous nomination altered originals")
				}
				// Explicit offline verified-restore journal entry establishes the
				// known boundary even when its wall clock precedes both predecessors.
				latest, e := s.QuarantineRecovery(ctx, controlledRecoveryOrigin(), time.Now().Add(-time.Hour))
				if e != nil {
					t.Fatal(e)
				}
				current, e := s.RecoveryOriginForCustody(ctx)
				if e != nil || current.ID != latest.ID {
					t.Fatal("New known boundary selected an older timestamp", e)
				}
				maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epochs", 3)
				maintenanceCount(t, s, "SELECT COUNT(*) FROM recovery_epoch_heads", 1)
			}
		})
	}
}

func TestRecoveryReviewerExpiryAndAccountSuspensionHoldWithoutRevivingCheckpointRoles(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	one, _, in := currentRecoveryReviewers(t, s)
	p, err := principal(ctx, s.DB, TokenHash(one), time.Now())
	if err != nil || !p.CanReviewRecovery {
		t.Fatal(err)
	}
	// A supplied replacement has a short, explicit term. Advance only the
	// principal's controlled verification clock, keeping the actual bearer valid.
	origin, err := s.RecoveryOriginForCustody(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next, passwords := recoveryAppointment(t, s, origin)
	next.OperationID = "short-term-current-reviewers"
	next.ReplacesOperationID = in.OperationID
	next.ValidUntil = time.Now().Unix() + 60
	for i := range next.Reviewers {
		if err = s.DB.QueryRow("SELECT id FROM users WHERE login=?", next.Reviewers[i].Email).Scan(&next.Reviewers[i].AccountID); err != nil {
			t.Fatal(err)
		}
	}
	passwords.Passwords = nil
	if _, err = s.NominateRecovery(ctx, next, passwords); err != nil {
		t.Fatal(err)
	}
	fresh, _ := loginTest(t, s, next.Reviewers[0].Email, recoveryPasswordOne)
	current, err := principal(ctx, s.DB, TokenHash(fresh), time.Now())
	if err != nil || !current.RecoveryReviewer {
		t.Fatal(err)
	}
	expired, err := principal(ctx, s.DB, TokenHash(fresh), time.Now().Add(2*time.Minute))
	if err != nil || expired.RecoveryReviewer || expired.CanReviewRecovery || expired.ScopeKey == current.ScopeKey || len(expired.Roles) != 0 {
		t.Fatal("Expired term retained review or old private authority", expired, err)
	}
	if _, err = s.DB.Exec("UPDATE users SET suspended_at=unixepoch('now') WHERE id=?", current.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CheckSession(ctx, fresh); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("Suspended nominee retained their bearer", err)
	}
}

func TestRecoveryReplacementMakesEveryEarlierDecisionStaleAndKeepsHistoricalEvidence(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	one, two, nomination := currentRecoveryReviewers(t, s)
	in := recoveryAccessProposal(t, s, one)
	original, err := s.ProposeRecovery(ctx, one, in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CheckSession(ctx, two)
	if err != nil {
		t.Fatal(err)
	}
	decision := RecoveryDecisionInput{OperationKey: randomToken(), ScopeKey: second.ScopeKey, Version: 1, Decision: "APPROVED", Reason: "Independent supplied current person, home and term verified.", Attestation: "I checked this frozen version separately from the author.", Confirmed: true}
	approved, err := s.DecideRecovery(ctx, two, original.ID, decision)
	if err != nil || approved.State != "APPROVED" {
		t.Fatal(approved, err)
	}
	before := moneyRows(t, s)
	var factorBefore string
	if err = s.DB.QueryRow("SELECT hex(secret_ciphertext) FROM mfa_factors WHERE user_id=(SELECT id FROM users WHERE login=?)", nomination.Reviewers[0].Email).Scan(&factorBefore); err != nil {
		t.Fatal(err)
	}
	origin, err := s.RecoveryOriginForCustody(ctx)
	if err != nil {
		t.Fatal(err)
	}
	replacement, passwords := recoveryAppointment(t, s, origin)
	replacement.OperationID = "replacement-evidence-bound-reviewers"
	replacement.ReplacesOperationID = nomination.OperationID
	for i := range replacement.Reviewers {
		if err = s.DB.QueryRow("SELECT id FROM users WHERE login=?", replacement.Reviewers[i].Email).Scan(&replacement.Reviewers[i].AccountID); err != nil {
			t.Fatal(err)
		}
	}
	passwords.Passwords = nil
	if _, err = s.NominateRecovery(ctx, replacement, passwords); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CheckSession(ctx, one); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("Replacement kept predecessor bearer", err)
	}
	fresh, person := loginTest(t, s, replacement.Reviewers[0].Email, recoveryPasswordOne)
	var encrypted []byte
	if err = s.DB.QueryRow("SELECT secret_ciphertext FROM mfa_factors WHERE user_id=?", person.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	secret, err := s.MFA.Open(person.ID, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	// The next accepted real TOTP window avoids replaying the enrollment code.
	current, err := s.VerifyMFA(ctx, fresh, security.Code(secret, time.Now().Unix()/30+1, 6), false)
	if err != nil || !current.CanReviewRecovery {
		t.Fatal(current, err)
	}
	retained, err := s.RecoveryProposalFor(ctx, fresh, original.ID)
	if err != nil || retained.State != "STALE" || retained.Version != 2 || !reflect.DeepEqual(retained.Decision, approved.Decision) || !reflect.DeepEqual(retained.Input, approved.Input) {
		t.Fatal("Earlier approved preparation must be stale while its exact historical decision stays retained", retained, err)
	}
	summary, err := s.RecoverySummaryFor(ctx, fresh)
	if err != nil || summary.Approved != 0 || summary.Pending != 0 || summary.Stale != 1 || summary.CanResume {
		t.Fatal("Current counts revived earlier authority", summary, err)
	}
	for _, state := range []string{"STALE", "APPROVED", "DECLINED", "PENDING"} {
		list, e := s.RecoveryProposalsFor(ctx, fresh, "ACCESS", state, 1)
		want := 0
		if state == "STALE" {
			want = 1
		}
		if e != nil || list.Total != want || len(list.Items) != want {
			t.Fatal("Filtered list disagrees with current nomination boundary", state, list, e)
		}
	}
	history, err := s.RecoveryHistoryFor(ctx, fresh, original.ID, 1)
	if err != nil || len(history.Items) != 1 || history.Items[0].State != "STALE" || history.Items[0].Decision.Decision != "APPROVED" {
		t.Fatal("Historical decision lost its separate current classification", history, err)
	}
	correction := recoveryAccessProposal(t, s, fresh)
	correction.PredecessorID, correction.PredecessorVersion = retained.ID, retained.Version
	correction.Access.Relationship = "FAMILY"
	linked, err := s.ProposeRecovery(ctx, fresh, correction)
	if err != nil || linked.State != "PENDING" || linked.PredecessorID != original.ID || linked.Version != 3 {
		t.Fatal("Current linked correction did not retain the original", linked, err)
	}
	var factorAfter string
	if err = s.DB.QueryRow("SELECT hex(secret_ciphertext) FROM mfa_factors WHERE user_id=?", person.ID).Scan(&factorAfter); err != nil || factorAfter != factorBefore {
		t.Fatal("Replacement reset the existing personal factor", err)
	}
	for _, table := range []string{"recovery_decisions", "entries", "receipts"} {
		if moneyRows(t, s)[table] != before[table] {
			t.Fatal("Replacement/correction rewrote predecessor evidence or originals", table)
		}
	}
}
