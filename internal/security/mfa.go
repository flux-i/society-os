package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Box struct{ aead cipher.AEAD }

func NewBox(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, errors.New("MFA encryption key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead}, nil
}
func (b *Box) Seal(user string, secret []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return b.aead.Seal(nonce, nonce, secret, []byte("society-totp-v1:"+user))
}
func (b *Box) Open(user string, encrypted []byte) ([]byte, error) {
	if b == nil || len(encrypted) < b.aead.NonceSize()+b.aead.Overhead() {
		return nil, errors.New("MFA secret is unavailable")
	}
	n := b.aead.NonceSize()
	secret, err := b.aead.Open(nil, encrypted[:n], encrypted[n:], []byte("society-totp-v1:"+user))
	if err != nil {
		return nil, errors.New("MFA encryption key does not match stored secrets")
	}
	return secret, nil
}

func LoadKey(path string, create bool) (*Box, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
			return nil, err
		}
		parent, err := os.Lstat(filepath.Dir(abs))
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("MFA key is missing; restore the separately held key")
		}
		if err != nil {
			return nil, err
		}
		if !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
			return nil, errors.New("MFA key directory must be private (mode 0700)")
		}
		f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			key := make([]byte, 32)
			if _, err = rand.Read(key); err == nil {
				_, err = f.WriteString(base64.RawStdEncoding.EncodeToString(key) + "\n")
			}
			if err == nil {
				err = f.Sync()
			}
			closeErr := f.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			directory, err := os.Open(filepath.Dir(abs))
			if err != nil {
				return nil, err
			}
			syncErr := directory.Sync()
			closeErr = directory.Close()
			if syncErr != nil {
				return nil, syncErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
		} else if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	parent, err := os.Lstat(filepath.Dir(abs))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("MFA key is missing; restore the separately held key")
	}
	if err != nil {
		return nil, err
	}
	if !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("MFA key directory must be private (mode 0700)")
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("MFA key is missing; restore the separately held key")
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 128 {
		return nil, errors.New("MFA key must be a regular private file (mode 0600)")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, errors.New("invalid MFA encryption key file")
	}
	return NewBox(key)
}

func NewSecret() []byte {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		panic(err)
	}
	return secret
}
func SecretText(secret []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

// RFC 6238, HMAC-SHA1, 30-second steps, six digits for authenticator interoperability.
func Code(secret []byte, step int64, digits int) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	modulus := uint32(1000000)
	if digits == 8 {
		modulus = 100000000
	}
	return fmt.Sprintf("%0*d", digits, value%modulus)
}
func Match(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	step := now.Unix() / 30
	for _, candidate := range []int64{step, step - 1, step + 1} {
		if candidate > lastStep && subtle.ConstantTimeCompare([]byte(Code(secret, candidate, 6)), []byte(code)) == 1 {
			return candidate, true
		}
	}
	return 0, false
}
