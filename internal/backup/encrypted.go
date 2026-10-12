package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"filippo.io/age"
)

const MaxEncryptedBytes int64 = 256 * 1024 * 1024
const MaxSnapshotBytes int64 = 512 * 1024 * 1024

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type ArchiveInfo struct {
	FormatVersion  int    `json:"format_version"`
	CapturedAt     int64  `json:"captured_at"`
	JobID          string `json:"job_id"`
	ConfigHash     string `json:"config_hash"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

type contextReader struct {
	ctx context.Context
	in  io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.in.Read(p)
}

type cappedWriter struct {
	out   io.Writer
	limit int64
	bytes int64
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.limit-w.bytes {
		return 0, errors.New("encrypted archive exceeds its physical size limit")
	}
	n, err := w.out.Write(p)
	w.bytes += int64(n)
	return n, err
}

// The live process receives public recipients only. There is no symmetric
// decryption secret or private custody identity on its encryption path.
func EncryptArchive(ctx context.Context, bundle, destination string, info ArchiveInfo, recipients []age.Recipient) (string, int64, error) {
	manifest, err := Verify(ctx, bundle)
	if err != nil {
		return "", 0, err
	}
	if manifest.Bytes <= 0 || manifest.Bytes > MaxSnapshotBytes || info.FormatVersion != 1 || info.CapturedAt <= 0 || info.JobID == "" || !sha256Pattern.MatchString(info.ConfigHash) || len(recipients) < 2 || len(recipients) > 4 {
		return "", 0, errors.New("archive identity, custody recipients or snapshot size is invalid")
	}
	info.ManifestSHA256, _, err = digest(filepath.Join(bundle, "manifest.json"))
	if err != nil {
		return "", 0, err
	}
	metadata, err := json.Marshal(info)
	if err != nil {
		return "", 0, err
	}
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", 0, err
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(destination)
		}
	}()
	bounded := &cappedWriter{out: f, limit: MaxEncryptedBytes}
	encrypted, err := age.Encrypt(bounded, recipients...)
	if err != nil {
		return "", 0, err
	}
	compressed := gzip.NewWriter(encrypted)
	archive := tar.NewWriter(compressed)
	if err = archive.WriteHeader(&tar.Header{Name: "archive.json", Mode: 0600, Size: int64(len(metadata)), Typeflag: tar.TypeReg}); err != nil {
		return "", 0, err
	}
	if _, err = archive.Write(metadata); err != nil {
		return "", 0, err
	}
	for _, name := range []string{"manifest.json", "society.db"} {
		path := filepath.Join(bundle, name)
		stat, err := os.Lstat(path)
		if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || (name == "manifest.json" && stat.Size() > 64*1024) || (name == "society.db" && stat.Size() != manifest.Bytes) {
			return "", 0, errors.New("snapshot input is not a bounded private regular file")
		}
		input, err := os.Open(path)
		if err != nil {
			return "", 0, err
		}
		opened, statErr := input.Stat()
		if statErr != nil || !os.SameFile(stat, opened) {
			input.Close()
			return "", 0, errors.New("snapshot input changed while opening")
		}
		err = archive.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: stat.Size(), Typeflag: tar.TypeReg})
		if err == nil {
			_, err = io.CopyN(archive, contextReader{ctx, input}, stat.Size())
		}
		closeErr := input.Close()
		if err != nil {
			return "", 0, err
		}
		if closeErr != nil {
			return "", 0, closeErr
		}
	}
	for _, close := range []func() error{archive.Close, compressed.Close, encrypted.Close, f.Sync, f.Close} {
		if err = close(); err != nil {
			return "", 0, err
		}
	}
	hash, size, err := digest(destination)
	if err != nil {
		return "", 0, err
	}
	complete = true
	return hash, size, nil
}

// Extract exactly three regular files, then consume the complete authenticated
// stream before trusting any of them. Failed extraction removes only its newly
// created private destination, never an existing directory or checkpoint.
func DecryptArchive(ctx context.Context, encryptedPath, expectedSHA256, destination string, identities []age.Identity) (ArchiveInfo, Manifest, error) {
	var info ArchiveInfo
	var manifest Manifest
	if !sha256Pattern.MatchString(expectedSHA256) || len(identities) < 1 || len(identities) > 4 {
		return info, manifest, errors.New("independent encrypted checksum and custody identities are required")
	}
	stat, err := os.Lstat(encryptedPath)
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() <= 0 || stat.Size() > MaxEncryptedBytes {
		return info, manifest, errors.New("encrypted input must be a bounded private regular file")
	}
	actualHash, actualBytes, err := digest(encryptedPath)
	if err != nil || actualHash != expectedSHA256 || actualBytes != stat.Size() {
		return info, manifest, errors.New("encrypted checksum or size differs from the independently retained acknowledgement")
	}
	f, err := os.Open(encryptedPath)
	if err != nil {
		return info, manifest, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(stat, opened) {
		return info, manifest, errors.New("encrypted input changed while opening")
	}
	consumedHash := sha256.New()
	decrypted, err := age.Decrypt(contextReader{ctx, io.TeeReader(f, consumedHash)}, identities...)
	if err != nil {
		return info, manifest, errors.New("custody identity does not decrypt this archive")
	}
	compressed, err := gzip.NewReader(decrypted)
	if err != nil {
		return info, manifest, errors.New("encrypted payload is not the supported compressed archive")
	}
	defer compressed.Close()
	bounded := &io.LimitedReader{R: compressed, N: MaxSnapshotBytes + 128*1024}
	archive := tar.NewReader(bounded)
	abs, err := filepath.Abs(destination)
	if err != nil {
		return info, manifest, err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return info, manifest, err
	}
	if err = os.Mkdir(abs, 0700); err != nil {
		return info, manifest, errors.New("decrypted bundle destination must be new")
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(abs)
		}
	}()
	seen := map[string]bool{}
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return info, manifest, errors.New("encrypted archive is truncated or malformed")
		}
		limit := int64(64 * 1024)
		if header.Name == "society.db" {
			limit = MaxSnapshotBytes
		}
		if (header.Name != "archive.json" && header.Name != "manifest.json" && header.Name != "society.db") || seen[header.Name] || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > limit || len(header.PAXRecords) != 0 || header.Linkname != "" {
			return info, manifest, errors.New("archive contains an unexpected path, duplicate, link or oversized member")
		}
		seen[header.Name] = true
		output, err := os.OpenFile(filepath.Join(abs, header.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return info, manifest, err
		}
		_, copyErr := io.CopyN(output, archive, header.Size)
		if copyErr == nil {
			copyErr = output.Sync()
		}
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			return info, manifest, errors.New("archive member could not be verified and retained")
		}
	}
	// tar EOF alone need not consume the age authentication tag or gzip checksum.
	// Reject appended content while reading all remaining authenticated bytes.
	trailing, err := io.ReadAll(io.LimitReader(bounded, 4097))
	if err != nil || len(trailing) > 4096 || bounded.N <= 0 || len(bytes.Trim(trailing, "\x00")) != 0 {
		return info, manifest, errors.New("archive authentication, compression limit or trailing content is invalid")
	}
	var one [1]byte
	if n, err := bounded.Read(one[:]); n != 0 || !errors.Is(err, io.EOF) {
		return info, manifest, errors.New("archive stream did not reach authenticated EOF")
	}
	if hex.EncodeToString(consumedHash.Sum(nil)) != expectedSHA256 {
		return info, manifest, errors.New("consumed encrypted bytes differ from the independently retained original")
	}
	if len(seen) != 3 {
		return info, manifest, errors.New("archive does not contain the exact required files")
	}
	metadata, err := os.ReadFile(filepath.Join(abs, "archive.json"))
	if err != nil {
		return info, manifest, err
	}
	decoder := json.NewDecoder(bytes.NewReader(metadata))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&info); err != nil {
		return info, manifest, errors.New("unsupported archive identity")
	}
	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) || info.FormatVersion != 1 || info.CapturedAt <= 0 || info.JobID == "" || len(info.JobID) > 100 || !sha256Pattern.MatchString(info.ConfigHash) || !sha256Pattern.MatchString(info.ManifestSHA256) {
		return info, manifest, errors.New("unsupported or incomplete archive identity")
	}
	manifestHash, _, err := digest(filepath.Join(abs, "manifest.json"))
	if err != nil || manifestHash != info.ManifestSHA256 {
		return info, manifest, errors.New("snapshot manifest differs from the encrypted archive identity")
	}
	manifest, err = Verify(ctx, abs)
	if err != nil {
		return info, manifest, fmt.Errorf("decrypted snapshot failed verification: %w", err)
	}
	complete = true
	return info, manifest, nil
}
