package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"filippo.io/age"
	"society.local/portal/internal/database"
	"society.local/portal/internal/security"
)

// The added journal is the only permitted row difference from a matching copy.
// Every other table (including private BLOBs and provenance) stays exact.
func assertQuarantinedOriginals(t *testing.T, bundle, destination string) {
	t.Helper()
	source := filepath.Join(bundle, "society.db")
	want, got := persistentSnapshotRows(t, source), persistentSnapshotRows(t, destination)
	if len(want) != 119 || len(got) != 119 {
		t.Fatal("Expected all118 business tables plus provenance", len(want), len(got))
	}
	if len(got["recovery_epochs"]) != len(want["recovery_epochs"])+1 {
		t.Fatal("Recovery did not append exactly one immutable epoch")
	}
	for _, old := range want["recovery_epochs"] {
		found := false
		for _, current := range got["recovery_epochs"] {
			found = found || old == current
		}
		if !found {
			t.Fatal("Recovery replaced a predecessor epoch")
		}
	}
	for _, old := range want["recovery_external_jobs"] {
		found := false
		for _, current := range got["recovery_external_jobs"] {
			found = found || old == current
		}
		if !found {
			t.Fatal("Recovery replaced a predecessor external disposition")
		}
	}
	delete(want, "recovery_epochs")
	delete(got, "recovery_epochs")
	delete(want, "recovery_external_jobs")
	delete(got, "recovery_external_jobs")
	if len(got["recovery_epoch_heads"]) != len(want["recovery_epoch_heads"])+1 {
		t.Fatal("Recovery did not append exactly one durable current epoch head")
	}
	for _, old := range want["recovery_epoch_heads"] {
		found := false
		for _, current := range got["recovery_epoch_heads"] {
			found = found || current == old
		}
		if !found {
			t.Fatal("Recovery replaced a predecessor authority boundary")
		}
	}
	delete(want, "recovery_epoch_heads")
	delete(got, "recovery_epoch_heads")
	if !reflect.DeepEqual(want, got) {
		t.Fatal("Recovery changed a confirmed original, private BLOB, source or provenance")
	}
	manifest, err := Verify(context.Background(), bundle)
	if err != nil {
		t.Fatal("Recovery altered its retained source", err)
	}
	store, err := database.Open(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var count int
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM recovery_epochs WHERE source_sha256=? AND source_bytes=? AND source_schema=? AND source_application=?", manifest.SHA256, manifest.Bytes, manifest.SchemaVersion, manifest.ApplicationVersion).Scan(&count); err != nil || count < 1 {
		t.Fatal("Recovery journal did not bind the actual verified source", err)
	}
	var expected, actual int
	if err := store.DB.QueryRow("SELECT (SELECT COUNT(*) FROM offsite_backup_jobs WHERE state!='VERIFIED')+(SELECT COUNT(*) FROM message_deliveries WHERE state IN('QUEUED','CLAIMED','FAILED','UNKNOWN'))").Scan(&expected); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRow("SELECT COUNT(*) FROM recovery_external_jobs WHERE epoch_id=(SELECT epoch_id FROM recovery_epoch_heads ORDER BY sequence DESC LIMIT 1)").Scan(&actual); err != nil || actual != expected {
		t.Fatal("Recovery failed to retain every unresolved external identity", expected, actual, err)
	}
	if required, err := store.RecoveryRequired(context.Background()); err != nil || !required {
		t.Fatal("Verified operational restore omitted quarantine", err)
	}
}

func TestEncryptedOperationalRestoreQuarantinesItsOwnCapturingJobWithEitherCustodianAndRetainsNestedEpoch(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if err := s.SeedDemoAccounts(ctx); err != nil {
		t.Fatal(err)
	}
	s.MFA, _ = security.NewBox(make([]byte, 32))
	oldToken := upkeepRecoveryLogin(t, s, "admin@demo.society")
	start := time.Now()
	binding := strings.Repeat("b", 64)
	if err := s.BindOffsitePolicy(ctx, binding, "FIXTURE", start); err != nil {
		t.Fatal(err)
	}
	j, err := s.ClaimOffsite(ctx, binding, "private-original-key", start)
	if err != nil {
		t.Fatal(err)
	}
	original, err := s.OffsiteJobForCustody(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "source")
	if _, err := Snapshot(ctx, s, bundle, "0.32.0-dev"); err != nil {
		t.Fatal(err)
	}
	recipients, identities := custodyPair(t)
	cipher := filepath.Join(root, "source.tar.gz.age")
	hash, _, err := EncryptArchive(ctx, bundle, cipher, ArchiveInfo{FormatVersion: 1, CapturedAt: j.CapturedAt, JobID: j.ID, ConfigHash: binding}, recipients)
	if err != nil {
		t.Fatal(err)
	}
	for index, identity := range identities {
		target := filepath.Join(root, "custodian-"+string(rune('a'+index)), "society.db")
		info, _, err := RestoreEncrypted(ctx, cipher, hash, target, []age.Identity{identity})
		if err != nil || info.JobID != j.ID {
			t.Fatal("Genuine custody identity could not restore", err)
		}
		assertQuarantinedOriginals(t, bundle, target)
		r, err := database.Open(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		r.MFA = s.MFA
		if _, err := r.CheckSession(ctx, oldToken); !errors.Is(err, database.ErrUnauthenticated) {
			t.Fatal("Old bearer survived", err)
		}
		fresh, p, err := r.Login(ctx, "admin@demo.society", database.DemoPassword, database.HashPassword("dummy"))
		if err != nil || !p.RecoveryRequired || p.CanManageAccounts || p.CanReadRegistry || len(p.Roles) != 0 {
			t.Fatal("Fresh copied admin regained authority", err)
		}
		if _, err := r.SummaryFor(ctx, fresh); !errors.Is(err, database.ErrRecoveryRequired) {
			t.Fatal("Private registry crossed quarantine", err)
		}
		if _, err := r.ClaimOffsite(ctx, binding, "private-original-key", start.Add(time.Hour)); !errors.Is(err, database.ErrRecoveryRequired) {
			t.Fatal("Old capturing identity retried", err)
		}
		current, err := r.OffsiteJobForCustody(ctx, j.ID)
		if err != nil || current != original {
			t.Fatal("Original capture changed during operational restart", err)
		}
		worker := &OffsiteWorker{Store: r, Directory: DefaultOffsitePath(target)}
		if err := worker.Process(ctx); err != nil {
			t.Fatal("Quarantine attempted external work without a client", err)
		}
		if _, err := os.Stat(worker.Directory); !os.IsNotExist(err) {
			t.Fatal("Quarantine created or cleaned staging", err)
		}
		if index == 0 {
			nested := filepath.Join(root, "quarantined-checkpoint")
			if _, err := Snapshot(ctx, r, nested, "0.32.0-dev"); err != nil {
				t.Fatal(err)
			}
			nestedTarget := filepath.Join(root, "second-recovery", "society.db")
			if _, err := Restore(ctx, nested, nestedTarget); err != nil {
				t.Fatal(err)
			}
			assertQuarantinedOriginals(t, nested, nestedTarget)
		}
		r.Close()
	}
	if current, _, err := digest(cipher); err != nil || current != hash {
		t.Fatal("Operational restore changed source ciphertext", err)
	}
}

func TestOperationalRestoreJournalFailureRemovesOnlyFreshDestinationAndConcurrentRestoreHasOneWinner(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	if _, err := s.DB.Exec("CREATE TRIGGER injected_recovery_failure BEFORE INSERT ON recovery_epochs BEGIN SELECT RAISE(ABORT,'injected recovery acknowledgement failure'); END"); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(root, "journal-failure-source")
	if _, err := Snapshot(ctx, s, bundle, "0.32.0-dev"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "failed", "society.db")
	if _, err := Restore(ctx, bundle, target); err == nil || !strings.Contains(err.Error(), "injected recovery acknowledgement failure") {
		t.Fatal("Injected journal failure was hidden", err)
	}
	for _, path := range []string{target, target + "-wal", target + "-shm"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Incomplete target or sidecar survived", path, err)
		}
	}
	if _, err := Verify(ctx, bundle); err != nil {
		t.Fatal("Journal failure altered source", err)
	}
	if _, err := s.DB.Exec("DROP TRIGGER injected_recovery_failure"); err != nil {
		t.Fatal(err)
	}
	valid := filepath.Join(root, "valid-source")
	if _, err := Snapshot(ctx, s, valid, "0.32.0-dev"); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "concurrent", "society.db")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := Restore(ctx, valid, destination); results <- err }()
	}
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatal("Concurrent fresh destination produced multiple/no accepted hosts", winners)
	}
	assertQuarantinedOriginals(t, valid, destination)
	before, _, _ := digest(destination)
	if _, err := Restore(ctx, valid, destination); err == nil {
		t.Fatal("Accepted recovery destination was overwritten")
	}
	if after, _, _ := digest(destination); after != before {
		t.Fatal("Rejected existing destination changed bytes")
	}
}

func TestOperationalRestoreRejectsMalformedSourceIdentityBeforeCreatingDestination(t *testing.T) {
	s, root := setup(t)
	ctx := context.Background()
	bundle := filepath.Join(root, "source")
	if _, err := Snapshot(ctx, s, bundle, ""); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "rejected", "society.db")
	if _, err := Restore(ctx, bundle, target); err == nil {
		t.Fatal("Missing application identity became an operational host")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("Malformed identity created a target", err)
	}
}
