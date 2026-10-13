package backup

import (
	"context"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// RestoreEncrypted verifies the whole independently retrieved archive, then
// restores its matching checkpoint into a fresh, quarantined private database.
// Scratch plaintext never survives this offline operation; source bytes remain.
func RestoreEncrypted(ctx context.Context, encrypted, expectedSHA256, destination string, identities []age.Identity) (ArchiveInfo, Manifest, error) {
	scratch, err := os.MkdirTemp("", "society-recovery-verify-")
	if err != nil {
		return ArchiveInfo{}, Manifest{}, err
	}
	defer os.RemoveAll(scratch)
	bundle := filepath.Join(scratch, "verified")
	info, manifest, err := DecryptArchive(ctx, encrypted, expectedSHA256, bundle, identities)
	if err != nil {
		return info, manifest, err
	}
	manifest, err = Restore(ctx, bundle, destination)
	return info, manifest, err
}
