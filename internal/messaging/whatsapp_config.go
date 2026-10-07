package messaging

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func LoadWhatsAppConfig(path string) (WhatsAppConfig, error) {
	var config WhatsAppConfig
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 32768 {
		return config, errors.New("WhatsApp configuration must be a separately held regular private file (mode 0600)")
	}
	dir, err := os.Lstat(filepath.Dir(path))
	if err != nil || !dir.IsDir() || dir.Mode().Perm()&0077 != 0 {
		return config, errors.New("WhatsApp configuration directory must be private (mode 0700)")
	}
	f, err := os.Open(path)
	if err != nil {
		return config, errors.New("WhatsApp configuration unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return config, errors.New("WhatsApp configuration changed while opening")
	}
	d := json.NewDecoder(io.LimitReader(f, 32769))
	d.DisallowUnknownFields()
	if d.Decode(&config) != nil || d.Decode(new(any)) != io.EOF {
		return WhatsAppConfig{}, errors.New("invalid private WhatsApp configuration")
	}
	return config, nil
}

func (e *Engine) WithWhatsAppFixture(client *WhatsAppClient) error {
	if client == nil || client.mode != "CLOUD_FIXTURE" {
		return errors.New("the local preview accepts only an explicit loopback WhatsApp fixture")
	}
	e.whatsapp = client
	return nil
}

func (e *Engine) VerifyWhatsAppKey(ctx context.Context) error {
	if e.whatsapp == nil {
		return nil
	}
	sum := sha256.Sum256([]byte(e.whatsapp.config.AppSecret))
	fingerprint := hex.EncodeToString(sum[:])
	tx, err := e.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stored string
	err = tx.QueryRowContext(ctx, "SELECT value FROM app_metadata WHERE key='whatsapp_app_secret_fingerprint'").Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		var history int
		if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM whatsapp_handoffs").Scan(&history); err != nil {
			return err
		}
		if history != 0 {
			return errors.New("restore the separately held WhatsApp application secret and its database fingerprint")
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO app_metadata(key,value) VALUES('whatsapp_app_secret_fingerprint',?)", fingerprint); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !hmac.Equal([]byte(stored), []byte(fingerprint)) {
		return errors.New("WhatsApp application secret does not match this database")
	}
	return tx.Commit()
}
