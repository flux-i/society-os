package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"society.local/portal/internal/objectstorage"
	"society.local/portal/internal/security"
	"society.local/portal/internal/testfixtures"
)

func configuredOffsiteWorker(t *testing.T, mode string) (*OffsiteWorker, *testfixtures.S3) {
	t.Helper()
	s, root := setup(t)
	fixture := testfixtures.NewS3(t)
	fixture.SetMode(mode)
	recipients, _ := custodyPair(t)
	const hash = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := s.BindOffsitePolicy(context.Background(), hash, "FIXTURE", time.Now()); err != nil {
		t.Fatal(err)
	}
	client, err := objectstorage.New(fixture.Config("writer"), true)
	if err != nil {
		t.Fatal(err)
	}
	return &OffsiteWorker{Store: s, Client: client, Recipients: recipients, ConfigHash: hash, Prefix: fixture.Config("writer").Prefix, Directory: filepath.Join(root, "stage"), Version: "0.30.0-dev"}, fixture
}

func TestOffsiteRestartCleansOnlyAcknowledgedStaging(t *testing.T) {
	w, fixture := configuredOffsiteWorker(t, "")
	ctx := context.Background()
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := w.Store.DB.QueryRow("SELECT id FROM offsite_backup_jobs WHERE state='VERIFIED'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	original, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	acknowledged := filepath.Join(w.Directory, id)
	if err := os.Mkdir(acknowledged, 0700); err != nil {
		t.Fatal(err)
	}
	// Simulate a crash after durable acknowledgement and before file cleanup.
	if err := os.WriteFile(filepath.Join(acknowledged, "snapshot.tar.gz.age"), fixture.Objects()[0].Bytes, 0600); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(w.Directory, strings.Repeat("x", 43))
	if err := os.Mkdir(unknown, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(unknown, "unclaimed-copy")
	if err := os.WriteFile(marker, []byte("Preserve independent retained bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted := *w
	if err := restarted.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(acknowledged); !os.IsNotExist(err) {
		t.Fatal("Restart retained an already acknowledged staging copy", err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "Preserve independent retained bytes" {
		t.Fatal("Restart deleted an unrecognized retained copy", err)
	}
	current, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil || current != original {
		t.Fatal("Cleanup changed immutable acknowledgement", err)
	}
	if puts, _ := fixture.Counts(); puts != 1 {
		t.Fatal("Cleanup resent an acknowledged copy", puts)
	}
}

func TestOffsiteRejectedUploadKeepsExactOriginalAcrossFiveBoundedAttempts(t *testing.T) {
	w, fixture := configuredOffsiteWorker(t, "DENIED")
	ctx := context.Background()
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := w.Store.DB.QueryRow("SELECT id FROM offsite_backup_jobs").Scan(&id); err != nil {
		t.Fatal(err)
	}
	original, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.Directory, id, "snapshot.tar.gz.age")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 2; attempt <= 5; attempt++ {
		if _, err := w.Store.DB.Exec("UPDATE offsite_backup_jobs SET available_at=0 WHERE id=?", id); err != nil {
			t.Fatal(err)
		}
		if err := w.Process(ctx); err != nil {
			t.Fatal(err)
		}
		current, err := w.Store.OffsiteJobForCustody(ctx, id)
		body, readErr := os.ReadFile(path)
		if err != nil || readErr != nil || current.Attempts != attempt || current.State != "FAILED" || current.ErrorCode != "UPLOAD_DENIED" || current.ObjectKey != original.ObjectKey || current.SHA256 != original.SHA256 || current.Bytes != original.Bytes || current.CapturedAt != original.CapturedAt || !bytes.Equal(before, body) {
			t.Fatal("Rejected-request retry changed the original or exceeded its budget", attempt, err, readErr)
		}
	}
	if _, err := w.Store.DB.Exec("UPDATE offsite_backup_jobs SET available_at=0 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if puts, _ := fixture.Counts(); puts != 5 {
		t.Fatal("Exhausted attempt was sent again", puts)
	}
	var jobs int
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_jobs").Scan(&jobs); err != nil || jobs != 1 {
		t.Fatal("Immediate retry invented another snapshot", jobs, err)
	}
}

func TestOffsitePhysicalStagingLimitIsVisibleAndResolvesWithoutLosingTheJob(t *testing.T) {
	w, fixture := configuredOffsiteWorker(t, "")
	ctx := context.Background()
	if err := os.MkdirAll(w.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	for i := range 65 {
		path := filepath.Join(w.Directory, "retained-"+string(rune('a'+i)))
		if err := os.WriteFile(path, []byte("retained"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	var id, state, code string
	if err := w.Store.DB.QueryRow("SELECT id,state,error_code FROM offsite_backup_jobs").Scan(&id, &state, &code); err != nil || state != "FAILED" || code != "STAGING_FULL" {
		t.Fatal("Physical staging exhaustion was hidden", state, code, err)
	}
	if puts, _ := fixture.Counts(); puts != 0 {
		t.Fatal("Exhausted staging sent external bytes", puts)
	}
	if err := w.Store.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	w.Store.MFA, _ = security.NewBox(make([]byte, 32))
	token := upkeepRecoveryLogin(t, w.Store, "admin@demo.society")
	status, err := w.Store.OffsiteStatusFor(ctx, token, "", 1)
	if err != nil || !status.StagingFull || !status.FinancePaused {
		t.Fatal("Operator status concealed the capacity failure", err)
	}
	for i := range 65 {
		if err := os.Remove(filepath.Join(w.Directory, "retained-"+string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.Store.DB.Exec("UPDATE offsite_backup_jobs SET available_at=0 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	current, err := w.Store.OffsiteJobForCustody(ctx, id)
	status, statusErr := w.Store.OffsiteStatusFor(ctx, token, "", 1)
	if err != nil || statusErr != nil || current.State != "VERIFIED" || current.Attempts != 2 || status.StagingFull || status.FinancePaused {
		t.Fatal("Resolved capacity did not retain and complete its bounded retry", err, statusErr)
	}
	if puts, _ := fixture.Counts(); puts != 1 {
		t.Fatal("Capacity retry duplicated external handoff", puts)
	}
}

func TestOffsiteLostReplyNeverResendsAndRequiresIndependentVersionDecrypt(t *testing.T) {
	s, root := setup(t)
	fixture := testfixtures.NewS3(t)
	fixture.SetMode("LOST_REPLY")
	_, identities := custodyPair(t)
	config := OffsiteConfig{FormatVersion: 1, Storage: fixture.Config("writer"), Recipients: []string{identities[0].Recipient().String(), identities[1].Recipient().String()}}
	body, _ := json.Marshal(config)
	path := filepath.Join(root, "private-config.json")
	os.WriteFile(path, body, 0600)
	_, client, recipients, hash, err := LoadOffsiteConfig(path, true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.BindOffsitePolicy(ctx, hash, "FIXTURE", time.Now()); err != nil {
		t.Fatal(err)
	}
	w := &OffsiteWorker{Store: s, Client: client, Recipients: recipients, ConfigHash: hash, Prefix: config.Storage.Prefix, Directory: filepath.Join(root, "staging"), Version: "0.30.0-dev"}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	objects := fixture.Objects()
	if len(objects) != 1 {
		t.Fatal("lost reply did not retain one original", len(objects))
	}
	var id string
	if err := s.DB.QueryRow("SELECT id FROM offsite_backup_jobs WHERE state='UNCERTAIN'").Scan(&id); err != nil {
		t.Fatal("lost reply became a usable point", err)
	}
	var allowed int
	if err := s.DB.QueryRow("SELECT allowed FROM offsite_financial_protection").Scan(&allowed); err != nil || allowed != 0 {
		t.Fatal("unknown upload unpaused finances", allowed, err)
	}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if puts, _ := fixture.Counts(); puts != 1 {
		t.Fatal("unknown handoff replayed automatically", puts)
	}
	fixture.SetMode("")
	reader, _ := objectstorage.New(fixture.Config("reader"), true)
	wrong, _ := age.GenerateX25519Identity()
	if _, _, err := ReconcileOffsite(ctx, s, reader, id, objects[0].Version, filepath.Join(root, "wrong-custody"), "Independent fictional custody check", []age.Identity{wrong}); err == nil {
		t.Fatal("wrong custody identity confirmed unknown original")
	}
	j, _ := s.OffsiteJobForCustody(ctx, id)
	if j.State != "UNCERTAIN" {
		t.Fatal("failed independent check changed the saved result")
	}
	info, manifest, err := ReconcileOffsite(ctx, s, reader, id, objects[0].Version, filepath.Join(root, "verified-custody"), "Independent fictional custodian decrypted and verified the exact original", []age.Identity{identities[1]})
	if err != nil || info.JobID != id || info.CapturedAt != j.CapturedAt || manifest.SchemaVersion != 29 {
		t.Fatal("independent reconciliation", info, err)
	}
	current, _ := s.OffsiteJobForCustody(ctx, id)
	if current.State != "VERIFIED" || current.SHA256 != j.SHA256 || current.CapturedAt != j.CapturedAt || current.ObjectVersion != objects[0].Version {
		t.Fatal("reconciliation altered the original identity")
	}
	if err := s.DB.QueryRow("SELECT allowed FROM offsite_financial_protection").Scan(&allowed); err != nil || allowed != 1 {
		t.Fatal("fresh independently verified point did not restore protection", allowed, err)
	}
}

func TestOffsiteVerifiedCopyCleansPlainStagingAndPinsDestinationAcrossCredentialRotation(t *testing.T) {
	s, root := setup(t)
	fixture := testfixtures.NewS3(t)
	_, identities := custodyPair(t)
	config := OffsiteConfig{FormatVersion: 1, Storage: fixture.Config("writer"), Recipients: []string{identities[0].Recipient().String(), identities[1].Recipient().String()}}
	path := filepath.Join(root, "private.json")
	body, _ := json.Marshal(config)
	os.WriteFile(path, body, 0600)
	_, client, recipients, hash, err := LoadOffsiteConfig(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindOffsitePolicy(context.Background(), hash, "FIXTURE", time.Now()); err != nil {
		t.Fatal(err)
	}
	w := &OffsiteWorker{Store: s, Client: client, Recipients: recipients, ConfigHash: hash, Prefix: config.Storage.Prefix, Directory: filepath.Join(root, "stage"), Version: "0.30.0-dev"}
	if err := w.Process(context.Background()); err != nil {
		t.Fatal(err)
	}
	var verified int
	s.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_jobs WHERE state='VERIFIED'").Scan(&verified)
	if verified != 1 {
		t.Fatal("exact signed upload not confirmed")
	}
	files, _ := os.ReadDir(w.Directory)
	if len(files) != 0 {
		t.Fatal("verified plaintext/encrypted staging retained", len(files))
	}
	config.Storage.SecretKey = "rotated-unused-fixture-signing-credential"
	config.Recipients[0], config.Recipients[1] = config.Recipients[1], config.Recipients[0]
	body, _ = json.Marshal(config)
	os.WriteFile(path, body, 0600)
	_, _, _, rotated, err := LoadOffsiteConfig(path, true)
	if err != nil || rotated != hash {
		t.Fatal("credential rotation or recipient order changed the retained destination", err)
	}
	config.Storage.Prefix += "/different"
	body, _ = json.Marshal(config)
	os.WriteFile(path, body, 0600)
	_, _, _, changed, err := LoadOffsiteConfig(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindOffsitePolicy(context.Background(), changed, "FIXTURE", time.Now()); err == nil {
		t.Fatal("changed destination silently reset required protection")
	}
}
