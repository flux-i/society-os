package backup

import (
	"context"
	"encoding/base32"
	"os"
	"path/filepath"
	"reflect"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
	"strings"
	"testing"
	"time"
)

func TestNonDemoWorkspaceRestorePreservesSetupImportSourceIdentityAndBoundKeysWithoutBearerSessions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s, err := database.Open(ctx, filepath.Join(root, "source", "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	signing := strings.Repeat("b", 64)
	setup := database.WorkspaceSetup{FormatVersion: 1, SocietyKey: "rehearsal-society", SocietyName: "Sample restore rehearsal", Mode: "LOCAL_WORKSPACE", BootstrapID: "restore-setup", AdministratorName: "Sample Custodian", AdministratorEmail: "registry@example.test", IdentityVerified: true, VerificationNote: "Verified fictional local custodian for restore coverage", TermDays: 90}
	password := "Private-fictional-workspace-password!"
	if _, err = s.BootstrapWorkspace(ctx, setup, password, s.MFA.Fingerprint(), signing); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.Login(ctx, setup.AdministratorEmail, password, database.HashPassword("dummy"))
	if err != nil {
		t.Fatal(err)
	}
	factor, err := s.SetupMFA(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(factor.Secret)
	if _, err = s.ConfirmMFA(ctx, token, security.Code(secret, time.Now().Unix()/30, 6)); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../../testdata/registry-import-118.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewRegistryImport(ctx, token, string(body))
	if err != nil {
		t.Fatal(err)
	}
	input := database.ApplyRegistryImport{InputText: string(body), Digest: p.Digest, BaseDigest: p.BaseDigest, EffectiveDate: p.EffectiveDate, OperationKey: "restore-register", Confirmed: true, Note: "Verified fictional source rows for retained restore rehearsal"}
	original, err := s.ApplyRegistryImport(ctx, token, input)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := Snapshot(ctx, s, filepath.Join(root, "bundle"), "0.28.0-dev")
	if err != nil || manifest.DataKind != "local-workspace" || manifest.Counts.Flats != 118 {
		t.Fatal("workspace snapshot", manifest, err)
	}
	path := filepath.Join(root, "restored", "workspace.db")
	if _, err = Restore(ctx, filepath.Join(root, "bundle"), path); err != nil {
		t.Fatal(err)
	}
	restored, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restored.MFA = s.MFA
	if err = restored.VerifyWorkspaceKeys(ctx, s.MFA.Fingerprint(), signing); err != nil {
		t.Fatal(err)
	}
	if err = restored.VerifyWorkspaceKeys(ctx, s.MFA.Fingerprint(), strings.Repeat("c", 64)); err == nil {
		t.Fatal("replacement restore key accepted")
	}
	if err = restored.VerifyMFAKey(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sessions", "account_tokens", "mfa_pending", "mfa_recovery_codes"} {
		var n int
		if err = restored.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatal("restore revived bearer material", table, n, err)
		}
	}
	for _, table := range []string{"workspace_setup", "registry_imports", "registry_import_entities"} {
		var before, after string
		query := "SELECT json_group_array(json_array("
		cols, err := s.DB.Query("PRAGMA table_info(" + table + ")")
		if err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for cols.Next() {
			var cid, nn, pk int
			var name, typ string
			var d any
			if err = cols.Scan(&cid, &name, &typ, &nn, &d, &pk); err != nil {
				t.Fatal(err)
			}
			names = append(names, name)
		}
		cols.Close()
		query += strings.Join(names, ",") + ")) FROM (SELECT * FROM " + table + " ORDER BY rowid)"
		if err = s.DB.QueryRow(query).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err = restored.DB.QueryRow(query).Scan(&after); err != nil || before != after {
			t.Fatal("workspace/import provenance changed", table, err)
		}
	}
	// Process restart followed by current real MFA and a deliberate replay.
	newToken, identity, err := restored.Login(ctx, setup.AdministratorEmail, password, database.HashPassword("dummy"))
	if err != nil || !identity.MFAPending || identity.IsDemo {
		t.Fatal("restored identity bypass", err)
	}
	if _, err = restored.VerifyMFA(ctx, newToken, security.Code(secret, time.Now().Unix()/30+1, 6), false); err != nil {
		t.Fatal(err)
	}
	again, err := restored.ApplyRegistryImport(ctx, newToken, input)
	if err != nil || !reflect.DeepEqual(original, again) {
		t.Fatal("post-restore retry changed import", again, err)
	}
}
