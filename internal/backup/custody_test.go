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
	"society.local/portal/internal/database"
	"society.local/portal/internal/testfixtures"
)

func exhaustedCustodyCandidate(t *testing.T) (*OffsiteWorker, *testfixtures.S3, []age.Identity, string) {
	t.Helper()
	w, peer := configuredOffsiteWorker(t, "DENIED")
	recipients, identities := custodyPair(t)
	w.Recipients, w.Directory, w.Version = recipients, DefaultOffsitePath(w.Store.Path), "0.31.0-dev"
	ctx := context.Background()
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := w.Store.DB.QueryRow("SELECT id FROM offsite_backup_jobs").Scan(&id); err != nil {
		t.Fatal(err)
	}
	for attempt := 2; attempt <= 5; attempt++ {
		if _, err := w.Store.DB.Exec("UPDATE offsite_backup_jobs SET available_at=0 WHERE id=?", id); err != nil {
			t.Fatal(err)
		}
		if err := w.Process(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if puts, _ := peer.Counts(); puts != 5 || len(peer.Objects()) != 0 {
		t.Fatal("Expected exactly five independently rejected signed uploads and no object", puts)
	}
	return w, peer, []age.Identity{identities[0], identities[1]}, id
}

func TestOffsiteCustodyRejectsAuthenticatedArchiveForAnotherJobBindingOrCapture(t *testing.T) {
	for _, mismatch := range []string{"job", "configuration", "capture"} {
		t.Run(mismatch, func(t *testing.T) {
			s, root := setup(t)
			ctx := context.Background()
			now := time.Now().Add(-time.Hour).Truncate(time.Second)
			binding := strings.Repeat("b", 64)
			if err := s.BindOffsitePolicy(ctx, binding, "FIXTURE", now); err != nil {
				t.Fatal(err)
			}
			j, err := s.ClaimOffsite(ctx, binding, "recovery/society", now)
			if err != nil {
				t.Fatal(err)
			}
			bundle := filepath.Join(root, "snapshot")
			if _, err := Snapshot(ctx, s, bundle, "0.31.0-dev"); err != nil {
				t.Fatal(err)
			}
			recipients, keys := custodyPair(t)
			info := ArchiveInfo{FormatVersion: 1, JobID: j.ID, ConfigHash: j.ConfigHash, CapturedAt: j.CapturedAt}
			switch mismatch {
			case "job":
				info.JobID = strings.Repeat("x", 43)
			case "configuration":
				info.ConfigHash = strings.Repeat("e", 64)
			case "capture":
				info.CapturedAt--
			}
			source := filepath.Join(root, "mismatched.age")
			hash, size, err := EncryptArchive(ctx, bundle, source, info, recipients)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SealOffsite(ctx, j, hash, size, now); err != nil {
				t.Fatal(err)
			}
			if err := s.FinishOffsite(ctx, j, "FAILED", "UPLOAD_DENIED", "", now); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB.Exec("UPDATE offsite_backup_jobs SET attempts=5 WHERE id=?", j.ID); err != nil {
				t.Fatal(err)
			}
			original, err := s.OffsiteJobForCustody(ctx, j.ID)
			if err != nil {
				t.Fatal(err)
			}
			originalBytes, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(t.TempDir(), "custody")
			if _, err := ArchiveOffsiteCandidate(ctx, s, j.ID, source, destination, "Independent private archive must match its original authenticated identity", []age.Identity{keys[0]}, false); err == nil {
				t.Fatal("Fully authenticated archive for a different original was disposed")
			}
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatal("Rejected copy remained at the completed destination", err)
			}
			after, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(originalBytes, after) {
				t.Fatal("Mismatch rejection changed the source ciphertext", err)
			}
			current, err := s.OffsiteJobForCustody(ctx, j.ID)
			if err != nil || current != original {
				t.Fatal("Mismatch rejection changed the retained job", err)
			}
			if _, err := s.OffsiteCustodyFor(ctx, j.ID); err == nil {
				t.Fatal("Mismatch rejection retained a disposition")
			}
		})
	}
}

func TestOffsiteCustodyResumeRejectsChangedIdentityAndUnsafeMembersPreservingTheSavedCopy(t *testing.T) {
	w, peer, identities, id := exhaustedCustodyCandidate(t)
	ctx := context.Background()
	source := filepath.Join(w.Directory, id, "snapshot.tar.gz.age")
	root := t.TempDir()
	destination := filepath.Join(root, "retained-original")
	note := "Independent custody verification remains tied to this exact retained original"
	saved, err := ArchiveOffsiteCandidate(ctx, w.Store, id, source, destination, note, identities[:1], false)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(destination, "custody.json"))
	if err != nil {
		t.Fatal(err)
	}
	cipherBytes, err := os.ReadFile(filepath.Join(destination, "snapshot.tar.gz.age"))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"archive-identity", "original-identity", "note", "snapshot-digest", "unknown-field", "trailing-data", "extra-plaintext", "public-manifest", "linked-cipher"} {
		t.Run(scenario, func(t *testing.T) {
			copy := filepath.Join(root, scenario)
			if err := os.Mkdir(copy, 0700); err != nil {
				t.Fatal(err)
			}
			metadataPath, cipherPath := filepath.Join(copy, "custody.json"), filepath.Join(copy, "snapshot.tar.gz.age")
			metadata := append([]byte(nil), manifestBytes...)
			var decoded custodyManifest
			if err := json.Unmarshal(metadata, &decoded); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "archive-identity":
				decoded.ArchiveID = strings.Repeat("X", 43)
			case "original-identity":
				decoded.Original.JobID = strings.Repeat("Y", 43)
			case "note":
				decoded.Note = "A changed note cannot replace the originally verified reason"
			case "snapshot-digest":
				decoded.SnapshotSHA256 = strings.Repeat("e", 64)
			}
			if scenario == "archive-identity" || scenario == "original-identity" || scenario == "note" || scenario == "snapshot-digest" {
				metadata, err = json.Marshal(decoded)
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unknown-field" {
				metadata = append(metadata[:len(metadata)-1], []byte(`,"unexpected":true}`)...)
			}
			if scenario == "trailing-data" {
				metadata = append(metadata, []byte(` {"another":"manifest"}`)...)
			}
			if err := os.WriteFile(metadataPath, metadata, 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "public-manifest" {
				if err := os.Chmod(metadataPath, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "linked-cipher" {
				if err := os.Symlink(filepath.Join(destination, "snapshot.tar.gz.age"), cipherPath); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(cipherPath, cipherBytes, 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "extra-plaintext" {
				if err := os.WriteFile(filepath.Join(copy, "unexpected.txt"), []byte("Unsupported archive member"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ArchiveOffsiteCandidate(ctx, w.Store, id, "", copy, note, identities[1:], true); err == nil {
				t.Fatal("Unsafe or altered retained archive was resumed")
			}
			if _, err := os.Lstat(copy); err != nil {
				t.Fatal("Rejected resume deleted the completed evidence", err)
			}
			current, err := w.Store.OffsiteCustodyFor(ctx, id)
			if err != nil || current != saved {
				t.Fatal("Rejected resume changed the saved disposition", err)
			}
		})
	}
	retained, err := os.ReadFile(filepath.Join(destination, "custody.json"))
	if err != nil || !bytes.Equal(retained, manifestBytes) {
		t.Fatal("Rejected resumes changed the original custody manifest", err)
	}
	retained, err = os.ReadFile(filepath.Join(destination, "snapshot.tar.gz.age"))
	if err != nil || !bytes.Equal(retained, cipherBytes) {
		t.Fatal("Rejected resumes changed the original ciphertext", err)
	}
	if puts, _ := peer.Counts(); puts != 5 {
		t.Fatal("Custody resume resent an exhausted upload", puts)
	}
}

func TestOffsiteCustodyArchiveKeepsExactBytesAndOriginalThroughCleanupAndResume(t *testing.T) {
	w, peer, identities, id := exhaustedCustodyCandidate(t)
	ctx := context.Background()
	source := filepath.Join(w.Directory, id, "snapshot.tar.gz.age")
	originalBytes, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	original, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var events int
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "independent-custody")
	note := "Independent fictional custodian retained and decrypted the exact exhausted original"
	saved, err := ArchiveOffsiteCandidate(ctx, w.Store, id, source, destination, note, identities[:1], false)
	if err != nil {
		t.Fatal(err)
	}
	copyBytes, err := os.ReadFile(filepath.Join(destination, "snapshot.tar.gz.age"))
	if err != nil || !bytes.Equal(copyBytes, originalBytes) {
		t.Fatal("Independent archive changed the original ciphertext", err)
	}
	files, err := os.ReadDir(destination)
	if err != nil || len(files) != 2 {
		t.Fatal("Archive retained plaintext or omitted its custody manifest", files, err)
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("Custody archive member was not private", file.Name(), err)
		}
	}
	metadata, manifestHash, err := readCustodyManifest(destination)
	if err != nil || metadata.Original.JobID != id || metadata.Original.CapturedAt != original.CapturedAt || manifestHash != saved.ManifestSHA256 {
		t.Fatal("Custody identity did not preserve the independently verified capture", err)
	}
	unknown := filepath.Join(w.Directory, strings.Repeat("z", 43))
	if err := os.Mkdir(unknown, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(unknown, "preserve-unrecognized-copy")
	if err := os.WriteFile(marker, []byte("Independent investigation remains required"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := w.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(source)); !os.IsNotExist(err) {
		t.Fatal("Committed custody did not release only the original active staging directory", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("Custody cleanup deleted unknown evidence", err)
	}
	again, err := ArchiveOffsiteCandidate(ctx, w.Store, id, "", destination, note, identities[1:], true)
	if err != nil || saved != again {
		t.Fatal("Second custodian resume did not return the exact saved disposition after cleanup", saved, again, err)
	}
	current, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil || current != original || current.AcknowledgedAt != 0 || current.ObjectVersion != "" {
		t.Fatal("Archival manufactured a usable point or changed the original", current, err)
	}
	var count, afterEvents int
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_custody_dispositions").Scan(&count); err != nil || count != 1 {
		t.Fatal("Custody resume duplicated its disposition", count, err)
	}
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_backup_events").Scan(&afterEvents); err != nil || afterEvents != events {
		t.Fatal("Custody rewrote original upload history", afterEvents, events, err)
	}
	if puts, _ := peer.Counts(); puts != 5 {
		t.Fatal("Custody cleanup or resume resent an exhausted upload", puts)
	}
	// The new immutable disposition must survive actual matching-release recovery
	// as populated data, not merely exist as an empty table in older fixtures.
	bundle := filepath.Join(t.TempDir(), "post-disposition-snapshot")
	if _, err := Snapshot(ctx, w.Store, bundle, "0.32.0-dev"); err != nil {
		t.Fatal(err)
	}
	want := persistentSnapshotRows(t, filepath.Join(bundle, "society.db"))
	if len(want) != 117 || len(want["offsite_custody_dispositions"]) != 1 {
		t.Fatal("Snapshot omitted populated custody evidence", len(want))
	}
	path := filepath.Join(t.TempDir(), "recovered", "society.db")
	if _, err := Restore(ctx, bundle, path); err != nil {
		t.Fatal(err)
	}
	assertQuarantinedOriginals(t, bundle, path)
	restored, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	retained, err := restored.OffsiteCustodyFor(ctx, id)
	if err != nil || retained != saved {
		t.Fatal("Recovered custody disposition changed", err)
	}
	recoveredOriginal, err := restored.OffsiteJobForCustody(ctx, id)
	if err != nil || recoveredOriginal != original {
		t.Fatal("Recovered exhausted job or acknowledgement changed", err)
	}
	var allowed int
	if err := restored.DB.QueryRow("SELECT allowed FROM offsite_financial_protection").Scan(&allowed); err != nil || allowed != 0 {
		t.Fatal("Recovered custody evidence released financial effects", err)
	}
	restarted := *w
	restarted.Store, restarted.Directory = restored, DefaultOffsitePath(path)
	retainedStage := filepath.Join(restarted.Directory, id)
	if err := os.MkdirAll(retainedStage, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(retainedStage, "snapshot.tar.gz.age"), originalBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Process(ctx); err != nil {
		t.Fatal(err)
	}
	if retained, err := os.ReadFile(filepath.Join(retainedStage, "snapshot.tar.gz.age")); err != nil || !bytes.Equal(retained, originalBytes) {
		t.Fatal("Quarantined restart changed or cleaned the retained staged original", err)
	}
	if puts, _ := peer.Counts(); puts != 5 {
		t.Fatal("Recovered custody replayed an exhausted original", puts)
	}
	again, err = ArchiveOffsiteCandidate(ctx, restored, id, "", destination, note, identities[1:], true)
	if err != nil || again != saved {
		t.Fatal("Recovered same-archive retry created another disposition", err)
	}
}

func TestOffsiteCustodyCompletedCopySurvivesJournalFailureAndResumesOnce(t *testing.T) {
	w, peer, identities, id := exhaustedCustodyCandidate(t)
	ctx := context.Background()
	source := filepath.Join(w.Directory, id, "snapshot.tar.gz.age")
	destination := filepath.Join(t.TempDir(), "retained-complete-copy")
	note := "Independent fictional copy is synced and verified before its journal acknowledgement"
	if _, err := w.Store.DB.Exec("CREATE TRIGGER fail_test_custody BEFORE INSERT ON offsite_custody_dispositions BEGIN SELECT RAISE(ABORT,'independent journal failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err := ArchiveOffsiteCandidate(ctx, w.Store, id, source, destination, note, identities[:1], false); err == nil {
		t.Fatal("Injected journal failure was acknowledged")
	}
	metadata, manifestHash, err := readCustodyManifest(destination)
	if err != nil {
		t.Fatal("Failed journal acknowledgement deleted the completed private archive", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("Failed acknowledgement deleted the active original", err)
	}
	var count int
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_custody_dispositions").Scan(&count); err != nil || count != 0 {
		t.Fatal("Failed acknowledgement released a staging slot", count, err)
	}
	if _, err := ArchiveOffsiteCandidate(ctx, w.Store, id, source, destination, note, identities[:1], false); err == nil {
		t.Fatal("A completed archive was overwritten without deliberate resume")
	}
	if _, err := w.Store.DB.Exec("DROP TRIGGER fail_test_custody"); err != nil {
		t.Fatal(err)
	}
	saved, err := ArchiveOffsiteCandidate(ctx, w.Store, id, "", destination, note, identities[1:], true)
	if err != nil || saved.ArchiveID != metadata.ArchiveID || saved.ManifestSHA256 != manifestHash {
		t.Fatal("Deliberate resume changed the completed original identity", saved, err)
	}
	again, err := ArchiveOffsiteCandidate(ctx, w.Store, id, "", destination, note, identities[:1], true)
	if err != nil || again != saved {
		t.Fatal("Saved custody acknowledgement was not retryable", err)
	}
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_custody_dispositions").Scan(&count); err != nil || count != 1 {
		t.Fatal("Deliberate resume produced more than one disposition", count, err)
	}
	if puts, _ := peer.Counts(); puts != 5 {
		t.Fatal("Local custody recovery sent another upload", puts)
	}
}

func TestOffsiteCustodyRejectsWrongIdentityChangedBytesAndUnsafeDestinationsWithoutDisposition(t *testing.T) {
	w, peer, identities, id := exhaustedCustodyCandidate(t)
	ctx := context.Background()
	source := filepath.Join(w.Directory, id, "snapshot.tar.gz.age")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	before, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	input := filepath.Join(root, "input")
	if err := os.Mkdir(input, 0700); err != nil {
		t.Fatal(err)
	}
	changed := append([]byte(nil), original...)
	changed[len(changed)/2] ^= 1
	for name, body := range map[string][]byte{"changed.age": changed, "truncated.age": original[:len(original)/2]} {
		if err := os.WriteFile(filepath.Join(input, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(input, "linked.age")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(input, "readable.age")
	if err := os.WriteFile(public, original, 0644); err != nil {
		t.Fatal(err)
	}
	wrong, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	note := "Independent fictional custody requires valid private originals and a separate destination"
	cases := []struct {
		name, source, destination string
		keys                      []age.Identity
	}{
		{"wrong-identity", source, filepath.Join(root, "wrong-key-output"), []age.Identity{wrong}},
		{"changed-cipher", filepath.Join(input, "changed.age"), filepath.Join(root, "changed-output"), identities[:1]},
		{"truncated-cipher", filepath.Join(input, "truncated.age"), filepath.Join(root, "truncated-output"), identities[:1]},
		{"linked-source", link, filepath.Join(root, "linked-output"), identities[:1]},
		{"public-source", public, filepath.Join(root, "public-output"), identities[:1]},
		{"active-staging", source, filepath.Join(w.Directory, "unsafe-output"), identities[:1]},
		{"source-directory", source, filepath.Join(filepath.Dir(source), "unsafe-output"), identities[:1]},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ArchiveOffsiteCandidate(ctx, w.Store, id, c.source, c.destination, note, c.keys, false); err == nil {
				t.Fatal("Invalid custody evidence or unsafe destination was accepted")
			}
			if _, err := os.Stat(c.destination); !os.IsNotExist(err) {
				t.Fatal("Rejected copy retained an incomplete destination", err)
			}
		})
	}
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(working, w.Store.Path)
	if err != nil {
		t.Fatal(err)
	}
	abs := w.Store.Path
	w.Store.Path = relative
	if _, err := ArchiveOffsiteCandidate(ctx, w.Store, id, source, filepath.Join(w.Directory, "relative-path-output"), note, identities[:1], false); err == nil {
		t.Fatal("Relative database path bypassed the active staging boundary")
	}
	w.Store.Path = abs
	current, err := w.Store.OffsiteJobForCustody(ctx, id)
	if err != nil || current != before {
		t.Fatal("Rejected custody changed the retained original", err)
	}
	var count int
	if err := w.Store.DB.QueryRow("SELECT COUNT(*) FROM offsite_custody_dispositions").Scan(&count); err != nil || count != 0 {
		t.Fatal("Rejected custody released a staging slot", count, err)
	}
	if body, err := os.ReadFile(source); err != nil || !bytes.Equal(body, original) {
		t.Fatal("Rejected custody changed original ciphertext", err)
	}
	if puts, _ := peer.Counts(); puts != 5 {
		t.Fatal("Rejected custody sent another upload", puts)
	}
}
