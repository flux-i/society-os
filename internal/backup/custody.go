package backup

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"society.local/portal/internal/database"
)

type custodyManifest struct {
	FormatVersion  int         `json:"format_version"`
	ArchiveID      string      `json:"archive_id"`
	CipherSHA256   string      `json:"ciphertext_sha256"`
	CipherBytes    int64       `json:"ciphertext_bytes"`
	Original       ArchiveInfo `json:"original_archive"`
	SnapshotSHA256 string      `json:"snapshot_sha256"`
	Note           string      `json:"note"`
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func custodyDestination(dbPath, source, destination string) (string, error) {
	abs, err := filepath.Abs(destination)
	if err != nil || destination == "" {
		return "", errors.New("a private custody destination is required")
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	databaseAbsolute, err := filepath.Abs(dbPath)
	if err != nil {
		return "", err
	}
	databaseParent, err := filepath.EvalSymlinks(filepath.Dir(databaseAbsolute))
	if err != nil {
		return "", err
	}
	staging := filepath.Join(databaseParent, "offsite-staging")
	if resolved, err := filepath.EvalSymlinks(staging); err == nil {
		staging = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if pathWithin(staging, abs) {
		return "", errors.New("custody must be retained outside active staging")
	}
	if source != "" {
		sourceAbsolute, err := filepath.Abs(source)
		if err != nil {
			return "", err
		}
		sourceParent, err := filepath.EvalSymlinks(filepath.Dir(sourceAbsolute))
		if err != nil {
			return "", err
		}
		if pathWithin(sourceParent, abs) {
			return "", errors.New("custody destination must be separate from the source directory")
		}
	}
	return abs, nil
}

func copyCustodyCipher(ctx context.Context, source, target, checksum string, bytes int64) error {
	stat, err := os.Lstat(source)
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() != bytes || bytes <= 0 || bytes > MaxEncryptedBytes {
		return errors.New("custody source must be the bounded private original ciphertext")
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(stat, opened) {
		return errors.New("custody source changed while opening")
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	copied, err := io.Copy(io.MultiWriter(output, hash), io.LimitReader(contextReader{ctx, input}, bytes+1))
	if err == nil && (copied != bytes || hex.EncodeToString(hash.Sum(nil)) != checksum) {
		err = errors.New("copied ciphertext differs from the retained original")
	}
	if err == nil {
		err = output.Sync()
	}
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func verifyCustodyCipher(ctx context.Context, path string, j database.OffsiteJob, identities []age.Identity) (ArchiveInfo, Manifest, error) {
	scratch, err := os.MkdirTemp("", "society-custody-verify-")
	if err != nil {
		return ArchiveInfo{}, Manifest{}, err
	}
	defer os.RemoveAll(scratch)
	info, manifest, err := DecryptArchive(ctx, path, j.SHA256, filepath.Join(scratch, "verified"), identities)
	if err != nil {
		return info, manifest, err
	}
	if info.JobID != j.ID || info.ConfigHash != j.ConfigHash || info.CapturedAt != j.CapturedAt {
		return info, manifest, errors.New("decrypted identity differs from the retained original")
	}
	return info, manifest, nil
}

func readCustodyManifest(directory string) (custodyManifest, string, error) {
	var out custodyManifest
	stat, err := os.Lstat(directory)
	if err != nil || !stat.IsDir() || stat.Mode().Perm()&0077 != 0 {
		return out, "", errors.New("resume requires an existing private custody directory")
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 2 {
		return out, "", errors.New("custody archive must contain only its ciphertext and manifest")
	}
	for _, file := range files {
		if file.Name() != "snapshot.tar.gz.age" && file.Name() != "custody.json" {
			return out, "", errors.New("custody archive contains an unexpected file")
		}
		info, err := file.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || file.Type()&os.ModeSymlink != 0 {
			return out, "", errors.New("custody archive contains an unsafe member")
		}
	}
	path := filepath.Join(directory, "custody.json")
	stat, err = os.Lstat(path)
	if err != nil || stat.Size() <= 0 || stat.Size() > 64*1024 {
		return out, "", errors.New("custody manifest exceeds its supported size")
	}
	input, err := os.Open(path)
	if err != nil {
		return out, "", err
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(stat, opened) {
		return out, "", errors.New("custody manifest changed while opening")
	}
	payload, err := io.ReadAll(io.LimitReader(input, 64*1024+1))
	if err != nil || len(payload) > 64*1024 {
		return out, "", errors.New("custody manifest exceeds its supported size")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&out); err != nil {
		return out, "", errors.New("custody manifest is invalid")
	}
	var extra any
	if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return out, "", errors.New("custody manifest has trailing data")
	}
	checksum := sha256.Sum256(payload)
	return out, hex.EncodeToString(checksum[:]), nil
}

// A completed copy is retained even if its database acknowledgement fails. An
// explicit resume decrypts that same private copy before returning/committing
// the one original disposition. Neither path changes an upload acknowledgement.
func ArchiveOffsiteCandidate(ctx context.Context, store *database.Store, id, source, destination, note string, identities []age.Identity, resume bool) (database.OffsiteCustodyDisposition, error) {
	var out database.OffsiteCustodyDisposition
	j, err := store.OffsiteJobForCustody(ctx, id)
	if err != nil {
		return out, err
	}
	if !database.OffsiteCustodyEligible(j) || len(identities) == 0 || len(note) < 10 || len(note) > 300 {
		return out, errors.New("offline custody requires an exhausted known-failure encrypted candidate, identity and verification reason")
	}
	directory, err := custodyDestination(store.Path, source, destination)
	if err != nil {
		return out, err
	}
	var metadata custodyManifest
	var manifestSHA string
	var verifiedInfo ArchiveInfo
	var verifiedManifest Manifest
	if resume {
		metadata, manifestSHA, err = readCustodyManifest(directory)
		if err != nil {
			return out, err
		}
		verifiedInfo, verifiedManifest, err = verifyCustodyCipher(ctx, filepath.Join(directory, "snapshot.tar.gz.age"), j, identities)
		if err != nil {
			return out, err
		}
	} else {
		if _, err := store.OffsiteCustodyFor(ctx, id); err == nil {
			return out, database.ErrConflict
		} else if !errors.Is(err, sql.ErrNoRows) {
			return out, err
		}
		if err = os.Mkdir(directory, 0700); err != nil {
			return out, errors.New("custody destination must be new; use deliberate resume for a completed archive")
		}
		complete := false
		defer func() {
			if !complete {
				os.RemoveAll(directory)
			}
		}()
		cipher := filepath.Join(directory, "snapshot.tar.gz.age")
		if err = copyCustodyCipher(ctx, source, cipher, j.SHA256, j.Bytes); err != nil {
			return out, err
		}
		info, manifest, err := verifyCustodyCipher(ctx, cipher, j, identities)
		if err != nil {
			return out, err
		}
		verifiedInfo, verifiedManifest = info, manifest
		var identifier [32]byte
		if _, err = rand.Read(identifier[:]); err != nil {
			return out, err
		}
		metadata = custodyManifest{FormatVersion: 1, ArchiveID: base64.RawURLEncoding.EncodeToString(identifier[:]), CipherSHA256: j.SHA256, CipherBytes: j.Bytes, Original: info, SnapshotSHA256: manifest.SHA256, Note: note}
		encoded, err := json.Marshal(metadata)
		if err != nil {
			return out, err
		}
		file, err := os.OpenFile(filepath.Join(directory, "custody.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return out, err
		}
		_, err = file.Write(encoded)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			return out, err
		}
		if closeErr != nil {
			return out, closeErr
		}
		if err = syncPath(directory); err != nil {
			return out, err
		}
		if err = syncPath(filepath.Dir(directory)); err != nil {
			return out, err
		}
		manifestSHA, _, err = digest(filepath.Join(directory, "custody.json"))
		if err != nil {
			return out, err
		}
		complete = true
	}
	if metadata.FormatVersion != 1 || !offsiteJobID.MatchString(metadata.ArchiveID) || metadata.CipherSHA256 != j.SHA256 || metadata.CipherBytes != j.Bytes || metadata.Note != note {
		return out, errors.New("custody manifest differs from the retained original or deliberate verification")
	}
	if verifiedInfo != metadata.Original || verifiedManifest.SHA256 != metadata.SnapshotSHA256 {
		return out, errors.New("independent custody verification differs from the archived identity")
	}
	return store.RecordOffsiteCustody(ctx, database.OffsiteCustodyInput{JobID: j.ID, ArchiveID: metadata.ArchiveID, ManifestSHA256: manifestSHA, CipherSHA256: j.SHA256, CipherBytes: j.Bytes, CapturedAt: j.CapturedAt, ConfigHash: j.ConfigHash, Note: note}, time.Now())
}
