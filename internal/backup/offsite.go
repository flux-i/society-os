package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"

	"society.local/portal/internal/database"
	"society.local/portal/internal/objectstorage"
)

type OffsiteConfig struct {
	FormatVersion int                  `json:"format_version"`
	Storage       objectstorage.Config `json:"storage"`
	Recipients    []string             `json:"public_recipients"`
}

func LoadOffsiteConfig(path string, fixtureAllowed bool) (OffsiteConfig, *objectstorage.Client, []age.Recipient, string, error) {
	var config OffsiteConfig
	if err := objectstorage.DecodePrivate(path, &config); err != nil {
		return config, nil, nil, "", err
	}
	if config.FormatVersion != 1 || len(config.Recipients) < 2 || len(config.Recipients) > 4 {
		return config, nil, nil, "", errors.New("off-site configuration requires a supported format and two distinct public custody recipients")
	}
	client, err := objectstorage.New(config.Storage, fixtureAllowed)
	if err != nil {
		return config, nil, nil, "", err
	}
	sort.Strings(config.Recipients)
	recipients := make([]age.Recipient, 0, len(config.Recipients))
	seen := map[string]bool{}
	for _, public := range config.Recipients {
		key, err := age.ParseX25519Recipient(public)
		if err != nil || seen[public] {
			return config, nil, nil, "", errors.New("custody material must contain distinct public X25519 recipients only")
		}
		seen[public] = true
		recipients = append(recipients, key)
	}
	// No access credential or private identity is retained in SQLite or returned.
	bound, _ := json.Marshal([]any{config.FormatVersion, config.Storage.Mode, config.Storage.Region, config.Storage.Bucket, config.Storage.Prefix, config.Storage.Endpoint, config.Recipients})
	return config, client, recipients, database.InputDigest(bound), nil
}

func LoadCustodyIdentities(path string) ([]age.Identity, error) {
	body, err := objectstorage.ReadPrivate(path, 16*1024)
	if err != nil {
		return nil, err
	}
	var identities []age.Identity
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		identity, err := age.ParseX25519Identity(line)
		if err != nil || len(identities) >= 4 {
			return nil, errors.New("custody file must contain bounded native private identities only")
		}
		identities = append(identities, identity)
	}
	if len(identities) == 0 {
		return nil, errors.New("a private custody identity is required")
	}
	return identities, nil
}

type OffsiteWorker struct {
	Store      *database.Store
	Client     *objectstorage.Client
	Recipients []age.Recipient
	ConfigHash string
	Prefix     string
	Directory  string
	Version    string
}

func DefaultOffsitePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "offsite-staging")
}

var offsiteJobID = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
var errOffsiteStagingFull = errors.New("staging has reached its retained-file limit")

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	stat, err := os.Lstat(path)
	if err != nil || !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
		return errors.New("off-site staging requires a private regular directory")
	}
	return nil
}

func (w *OffsiteWorker) jobDirectory(id string) (string, error) {
	if !offsiteJobID.MatchString(id) {
		return "", errors.New("off-site staging identity is invalid")
	}
	return filepath.Join(w.Directory, id), nil
}

// A durable acknowledgement can survive a crash before the final file cleanup.
// Remove only directories whose immutable saved job is already VERIFIED; unknown
// and unconfirmed originals remain available for independent investigation.
func (w *OffsiteWorker) cleanAcknowledgedStaging(ctx context.Context) error {
	if err := privateDirectory(w.Directory); err != nil {
		return err
	}
	items, err := os.ReadDir(w.Directory)
	if err != nil {
		return err
	}
	for _, item := range items {
		if !item.IsDir() || !offsiteJobID.MatchString(item.Name()) {
			continue
		}
		j, err := w.Store.OffsiteJobForCustody(ctx, item.Name())
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if j.State == "VERIFIED" {
			if err := os.RemoveAll(filepath.Join(w.Directory, item.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *OffsiteWorker) checkStaging() error {
	if err := privateDirectory(w.Directory); err != nil {
		return err
	}
	var bytes int64
	var files int
	err := filepath.WalkDir(w.Directory, func(_ string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if item.Type()&os.ModeSymlink != 0 {
			return errors.New("staging contains an unexpected link")
		}
		if item.IsDir() {
			return nil
		}
		info, err := item.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return errors.New("staging contains an unexpected file")
		}
		bytes += info.Size()
		files++
		if files > 64 || bytes > int64(database.OffsiteStagingSlots)*MaxEncryptedBytes+MaxSnapshotBytes {
			return errOffsiteStagingFull
		}
		return nil
	})
	return err
}

func (w *OffsiteWorker) prepare(ctx context.Context, j *database.OffsiteJob) (string, error) {
	directory, err := w.jobDirectory(j.ID)
	if err != nil {
		return "", err
	}
	if err = w.checkStaging(); err != nil {
		return "", err
	}
	path := filepath.Join(directory, "snapshot.tar.gz.age")
	if j.State == "UPLOADING" {
		if stat, err := os.Lstat(directory); err != nil || !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
			return "", errors.New("retained staging identity is unavailable")
		}
		if err := os.RemoveAll(filepath.Join(directory, "snapshot")); err != nil {
			return "", err
		}
		return path, nil // The storage boundary verifies the retained exact bytes.
	}
	// No external request can have occurred while this row remains CAPTURING.
	if stat, err := os.Lstat(directory); err == nil {
		if !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
			return "", errors.New("staging identity is not a private directory")
		}
		if err = os.RemoveAll(directory); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err = os.Mkdir(directory, 0700); err != nil {
		return "", err
	}
	bundle := filepath.Join(directory, "snapshot")
	if _, err = Snapshot(ctx, w.Store, bundle, w.Version); err != nil {
		os.RemoveAll(directory)
		return "", err
	}
	info := ArchiveInfo{FormatVersion: 1, CapturedAt: j.CapturedAt, JobID: j.ID, ConfigHash: w.ConfigHash}
	hash, bytes, err := EncryptArchive(ctx, bundle, path, info, w.Recipients)
	if err != nil {
		os.RemoveAll(directory)
		return "", err
	}
	if err = syncPath(directory); err != nil {
		return "", err
	}
	if err = w.Store.SealOffsite(ctx, *j, hash, bytes, time.Now()); err != nil {
		return "", err
	}
	j.SHA256, j.Bytes, j.State = hash, bytes, "SEALED"
	// Retain the encrypted original, rather than leaving a plaintext staging copy.
	if err = os.RemoveAll(bundle); err != nil {
		return "", err
	}
	if err = w.Store.BeginOffsiteUpload(ctx, *j, time.Now()); err != nil {
		return "", err
	}
	j.State = "UPLOADING"
	return path, nil
}

func (w *OffsiteWorker) Process(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := w.cleanAcknowledgedStaging(ctx); err != nil {
		return err
	}
	j, err := w.Store.ClaimOffsite(ctx, w.ConfigHash, w.Prefix, time.Now())
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	path, err := w.prepare(ctx, &j)
	state, code, version := "FAILED", "CAPTURE_FAILED", ""
	if errors.Is(err, errOffsiteStagingFull) {
		code = "STAGING_FULL"
	}
	if err == nil {
		ack, putErr := w.Client.Put(ctx, j.ObjectKey, path, j.SHA256, j.Bytes, time.Now())
		if putErr == nil {
			state, code, version = "VERIFIED", "", ack.Version
		} else {
			var upload *objectstorage.UploadError
			if errors.As(putErr, &upload) {
				code = upload.Code
				if upload.Uncertain {
					state = "UNCERTAIN"
				}
			} else {
				code = "UPLOAD_REJECTED"
			}
		}
	}
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if finishErr := w.Store.FinishOffsite(finishCtx, j, state, code, version, time.Now()); finishErr != nil {
		return finishErr
	}
	if state == "VERIFIED" {
		if directory, err := w.jobDirectory(j.ID); err == nil {
			os.RemoveAll(directory)
		}
	}
	return nil
}

func (w *OffsiteWorker) Run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.Process(ctx); err != nil && ctx.Err() == nil {
			logger.Warn("offsite_backup_unavailable", "stage", "bounded_attempt")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// This runs only in the independent offline custody command. A writer's PUT
// credential cannot substitute for exact-version retrieval and private decrypt.
func ReconcileOffsite(ctx context.Context, store *database.Store, reader *objectstorage.Client, id, version, destination, note string, identities []age.Identity) (ArchiveInfo, Manifest, error) {
	var info ArchiveInfo
	var manifest Manifest
	j, err := store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		return info, manifest, err
	}
	if j.State != "UNCERTAIN" {
		return info, manifest, database.ErrConflict
	}
	if err = os.Mkdir(destination, 0700); err != nil {
		return info, manifest, errors.New("custody reconciliation destination must be new")
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(destination)
		}
	}()
	path := filepath.Join(destination, "snapshot.tar.gz.age")
	if err = reader.GetVersion(ctx, j.ObjectKey, version, path, j.SHA256, j.Bytes, time.Now()); err != nil {
		return info, manifest, err
	}
	info, manifest, err = DecryptArchive(ctx, path, j.SHA256, filepath.Join(destination, "verified-bundle"), identities)
	if err != nil {
		return info, manifest, err
	}
	if info.JobID != j.ID || info.ConfigHash != j.ConfigHash || info.CapturedAt != j.CapturedAt {
		return info, manifest, errors.New("independently decrypted identity differs from its retained original")
	}
	if err = store.ReconcileOffsite(ctx, j.ID, j.SHA256, j.Bytes, version, note, time.Now()); err != nil {
		return info, manifest, err
	}
	complete = true
	return info, manifest, nil
}
