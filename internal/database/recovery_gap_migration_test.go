package database

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"society.local/portal/internal/security"
)

func TestRecoveryGapMigrationPreservesPopulatedGenuineSchema31WithoutRelease(t *testing.T) {
	ctx := context.Background()
	s := legacyRecoveryReviewStore(t, 0)
	for version := 30; version <= 31; version++ {
		body, err := migrationBody(version)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		if _, err = s.DB.Exec("INSERT INTO schema_migrations VALUES(?,?,?)", version, hex.EncodeToString(sum[:]), "2026-10-14"); err != nil {
			t.Fatal(err)
		}
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM schema_migrations", 31)
	maintenanceCount(t, s, "SELECT COUNT(*) FROM sqlite_schema WHERE name='recovery_gap_preparations'", 0)
	var err error
	s.MFA, _ = security.NewBox(make([]byte, 32))
	if err = s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.SeedDemoTreasury(ctx); err != nil {
		t.Fatal(err)
	}
	admin, _ := loginTest(t, s, "admin@demo.society", DemoPassword)
	if _, err = s.SetupMFA(ctx, admin); err != nil {
		t.Fatal(err)
	}
	code, _, err := s.DemoVerificationCode(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmMFA(ctx, admin, code); err != nil {
		t.Fatal(err)
	}
	post(t, s, admin, supplied("OPENING_DEBIT", "1500.01"))
	paid := post(t, s, admin, received("400.00"))
	if _, err = s.ReverseEntry(ctx, admin, paid, EntryAction{OperationKey: randomToken(), Confirmed: true, Reason: "Preserve this original 40000-paise receipt through the real schema-31 upgrade."}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	origin := RecoveryOrigin{ID: strings.Repeat("E", 43), FormatVersion: 1, SourceSHA256: strings.Repeat("e", 64), SourceBytes: 1757184, SourceSchema: 31, SourceApplication: "0.34.0-dev", SourceCreatedAt: now - 60, RestoredAt: now}
	// Arrange the predecessor's retained origin through its unchanged SQL.
	// The current offline restore correctly refuses a mismatching source schema.
	if _, err = s.DB.Exec("INSERT INTO recovery_epochs VALUES(?,?,?,?,?,?,?,?)", origin.ID, origin.FormatVersion, origin.SourceSHA256, origin.SourceBytes, origin.SourceSchema, origin.SourceApplication, origin.SourceCreatedAt, origin.RestoredAt); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO recovery_epoch_heads VALUES(1,?)", origin.ID); err != nil {
		t.Fatal(err)
	}
	nomination, passwords := recoveryAppointment(t, s, origin)
	if _, err = s.NominateRecovery(ctx, nomination, passwords); err != nil {
		t.Fatal(err)
	}
	login := func(email, password string) (string, Principal) {
		token, _ := loginTest(t, s, email, password)
		setup, err := s.SetupMFA(ctx, token)
		if err != nil {
			t.Fatal(err)
		}
		secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(setup.Secret)
		if err != nil {
			t.Fatal(err)
		}
		verified, err := s.ConfirmMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6))
		if err != nil || !verified.User.CanReviewRecovery {
			t.Fatal("A real current factor was not enrolled on the genuine predecessor", err)
		}
		return token, verified.User
	}
	one, p1 := login(nomination.Reviewers[0].Email, recoveryPasswordOne)
	_, p2 := login(nomination.Reviewers[1].Email, recoveryPasswordTwo)
	input := recoveryAccessProposal(t, s, one)
	frozen, _ := json.Marshal(input)
	id := strings.Repeat("P", 43)
	// Arrange an actual schema-31 retained review using its exact unchanged SQL
	// contract. New store methods intentionally reference schema-32 tables.
	if _, err = s.DB.Exec("INSERT INTO recovery_proposals VALUES(?,?,?,?,?,?,'ACCESS',1,NULL,?,?,?)", id, input.OperationKey, origin.ID, nomination.OperationID, p1.ID, nomination.Reviewers[0].IdentityRef, InputDigest(frozen), string(frozen), now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO recovery_decisions VALUES(?,?,?,?,1,'APPROVED',?,?,?,?)", id, "schema31-original-separate-decision", p2.ID, nomination.Reviewers[1].IdentityRef, "A separate schema-31 reviewer checked the supplied current household evidence.", "I independently checked this exact frozen version.", strings.Repeat("f", 64), now); err != nil {
		t.Fatal(err)
	}
	// Populate the actual predecessor numbering SQL and retain a synthetic
	// byte-boundary intent. This is migration preservation, not encryption or
	// independent storage verification; those use the real browser CLI fixture.
	numbering := suppliedNumbering(t, s, one, "TRV-SCHEMA31")
	numberingBody, _ := json.Marshal(numbering)
	policyID := strings.Repeat("N", 43)
	if _, err = s.DB.Exec("INSERT INTO recovery_numbering_policies VALUES(1,?,?,?,?,?,?,1,NULL,?,?,?,?)", policyID, numbering.OperationKey, origin.ID, nomination.OperationID, p1.ID, nomination.Reviewers[0].IdentityRef, InputDigest(numberingBody), string(numberingBody), numbering.Policy.Prefix, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO recovery_numbering_decisions VALUES(?,?,?,?,1,'APPROVED',?,?,?,?)", policyID, "schema31-numbering-decision", p2.ID, nomination.Reviewers[1].IdentityRef, "Independent predecessor reviewer checked supplied numbering.", "I checked the exact predecessor policy.", strings.Repeat("c", 64), now); err != nil {
		t.Fatal(err)
	}
	request := RecoverySeriesRequest{FormatVersion: 1, OperationID: "schema31-retained-series", AttemptKey: "schema31-original-handoff", EpochID: origin.ID, SourceSHA256: origin.SourceSHA256, NominationID: nomination.OperationID, PolicyID: policyID, PolicyVersion: 2, Custodians: nomination.Custodians}
	seal := func(body []byte) ([]byte, error) {
		return append([]byte("migration-byte-boundary-not-encryption-proof:"), body...), nil
	}
	if _, err = s.RetainRecoverySeries(ctx, request, strings.Repeat("b", 64), "private/fictional-series", seal); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.StartRecoverySeriesHandoff(ctx, request.OperationID, request.AttemptKey, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.FinishRecoverySeriesHandoff(ctx, request.OperationID, request.AttemptKey, "UNKNOWN", "UPLOAD_UNCERTAIN", ""); err != nil {
		t.Fatal(err)
	}
	names := []string{}
	rows, err := s.DB.Query("SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT IN('sessions','account_tokens','mfa_pending','mfa_recovery_codes') ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(names) != 117 {
		t.Fatal("The populated predecessor is not the independent schema-31 inventory", len(names))
	}
	before := statementMessageRows(t, s, names)
	definitions := map[string]string{}
	rows, err = s.DB.Query("SELECT type,name,sql FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var kind, name, sql string
		if err = rows.Scan(&kind, &name, &sql); err != nil {
			t.Fatal(err)
		}
		definitions[kind+":"+name] = sql
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	var provenance string
	const preceding = "SELECT json_group_array(json_array(version,checksum,applied_at)) FROM (SELECT version,checksum,applied_at FROM schema_migrations WHERE version<=31 ORDER BY version)"
	if err = s.DB.QueryRow(preceding).Scan(&provenance); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	maintenanceCount(t, s, "SELECT COUNT(*) FROM schema_migrations", SchemaVersion)
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	after := statementMessageRows(t, s, names)
	for table, retained := range before {
		if table != "schema_migrations" && after[table] != retained {
			t.Fatal("Gap migration changed preceding rows or BLOBs", table)
		}
	}
	var current string
	if err = s.DB.QueryRow(preceding).Scan(&current); err != nil || current != provenance {
		t.Fatal("Gap migration rewrote preceding provenance", err)
	}
	for key, sql := range definitions {
		parts := strings.SplitN(key, ":", 2)
		if err = s.DB.QueryRow("SELECT sql FROM sqlite_schema WHERE type=? AND name=?", parts[0], parts[1]).Scan(&current); err != nil || current != sql {
			t.Fatal("Gap migration rewrote a preceding SQL object", key, err)
		}
	}
	for _, table := range []string{"recovery_gap_preparations", "recovery_gap_decisions"} {
		maintenanceCount(t, s, "SELECT COUNT(*) FROM "+table, 0)
	}
	var amount, balance int64
	if err = s.DB.QueryRow("SELECT amount_paise FROM entries WHERE id=?", paid).Scan(&amount); err != nil || amount != 40000 {
		t.Fatal("Schema upgrade changed the retained paid amount", amount, err)
	}
	if err = s.DB.QueryRow("SELECT SUM(CASE WHEN kind='RECEIVED' THEN -amount_paise ELSE amount_paise END) FROM entries WHERE state='POSTED' AND id NOT IN(SELECT entry_id FROM entry_reversals)").Scan(&balance); err != nil || balance != 150001 {
		t.Fatal("Schema upgrade changed the independent corrected balance", balance, err)
	}
	status, err := s.RecoveryGapStatusFor(ctx, one)
	if err != nil || status.Pending != 0 || status.Approved != 0 || status.Known != 0 || status.Unresolved != 0 || status.CanResume {
		t.Fatal("Earlier access and numbering approvals invented a financial gap or release", status, err)
	}
}
