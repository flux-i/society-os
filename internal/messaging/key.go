package messaging

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func DefaultKeyPath(db string) string {
	return filepath.Join(filepath.Dir(db), "keys", "messages.key")
}

// Callback custody is separate from the database and the MFA encryption key.
// Never silently replace a missing key while provider history exists.
func LoadKey(path string, create bool) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parent := filepath.Dir(abs)
	if create {
		if err = os.MkdirAll(parent, 0700); err != nil {
			return nil, err
		}
	}
	dir, err := os.Lstat(parent)
	if err != nil {
		return nil, errors.New("the separately held message signing key is unavailable")
	}
	if !dir.IsDir() || dir.Mode().Perm()&0077 != 0 {
		return nil, errors.New("message key directory must be private (mode 0700)")
	}
	if create {
		f, fail := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if fail == nil {
			key := make([]byte, 32)
			if _, fail = rand.Read(key); fail == nil {
				_, fail = f.WriteString(base64.RawStdEncoding.EncodeToString(key) + "\n")
			}
			if fail == nil {
				fail = f.Sync()
			}
			closed := f.Close()
			if fail != nil {
				return nil, fail
			}
			if closed != nil {
				return nil, closed
			}
			d, fail := os.Open(parent)
			if fail != nil {
				return nil, fail
			}
			fail = d.Sync()
			closed = d.Close()
			if fail != nil {
				return nil, fail
			}
			if closed != nil {
				return nil, closed
			}
		} else if !errors.Is(fail, os.ErrExist) {
			return nil, fail
		}
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, errors.New("restore the separately held message signing key")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 128 {
		return nil, errors.New("message key must be a regular private file (mode 0600)")
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("message key changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, 129))
	if err != nil {
		return nil, err
	}
	if len(data) > 128 {
		return nil, errors.New("invalid message signing key")
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("invalid message signing key")
	}
	return key, nil
}
