package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
	"society.local/portal/internal/database"
	"society.local/portal/internal/objectstorage"
)

func seriesCipher(body []byte, recipients []age.Recipient) ([]byte, error) {
	if len(body) > 8192 || len(recipients) < 2 || len(recipients) > 4 {
		return nil, errors.New("bounded manifest and separate public custody recipients required")
	}
	var out bytes.Buffer
	writer, err := age.Encrypt(&out, recipients...)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(body); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	if out.Len() > 65536 {
		return nil, errors.New("encrypted reservation exceeds its limit")
	}
	return out.Bytes(), nil
}

func uploadSeries(ctx context.Context, store *database.Store, client *objectstorage.Client, intent database.RecoverySeriesIntent, key string, retry *database.RecoverySeriesRetry) (database.RecoverySeriesIntent, error) {
	// The local copy is made before recording a handoff; a local I/O failure
	// cannot invent network uncertainty. SQLite retains the exact original BLOB.
	directory, err := os.MkdirTemp("", "society-series-put-")
	if err != nil {
		return intent, err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "reservation.age")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return intent, err
	}
	_, err = file.Write(intent.Ciphertext)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return intent, err
	}
	if closeErr != nil {
		return intent, closeErr
	}
	intent, started, err := store.StartRecoverySeriesHandoff(ctx, intent.OperationID, key, retry)
	if err != nil || !started {
		return intent, err
	}
	ack, putErr := client.Put(ctx, intent.ObjectKey, path, intent.CipherSHA256, intent.CipherBytes, time.Now())
	state, code, version := "VERIFIED", "STORAGE_CONFIRMED", ack.Version
	if putErr != nil {
		state, code, version = "UNKNOWN", "UPLOAD_UNCERTAIN", ""
		var upload *objectstorage.UploadError
		if errors.As(putErr, &upload) && !upload.Uncertain {
			state, code = "REJECTED", upload.Code
		}
	}
	// Preserve the real handoff outcome even during graceful command shutdown.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return store.FinishRecoverySeriesHandoff(finish, intent.OperationID, key, state, code, version)
}

func ReserveRecoverySeries(ctx context.Context, store *database.Store, configFile string, fixtureAllowed bool, in database.RecoverySeriesRequest) (database.RecoverySeriesIntent, error) {
	config, client, recipients, hash, err := LoadOffsiteConfig(configFile, fixtureAllowed)
	if err != nil {
		return database.RecoverySeriesIntent{}, err
	}
	intent, err := store.RetainRecoverySeries(ctx, in, hash, config.Storage.Prefix, func(body []byte) ([]byte, error) { return seriesCipher(body, recipients) })
	if err != nil {
		return intent, err
	}
	return uploadSeries(ctx, store, client, intent, in.AttemptKey, nil)
}

func RetryRecoverySeries(ctx context.Context, store *database.Store, configFile string, fixtureAllowed bool, in database.RecoverySeriesRetry) (database.RecoverySeriesIntent, error) {
	_, client, _, hash, err := LoadOffsiteConfig(configFile, fixtureAllowed)
	if err != nil {
		return database.RecoverySeriesIntent{}, err
	}
	intent, err := store.RecoverySeriesForCustody(ctx, in.OperationID)
	if err != nil {
		return intent, err
	}
	if intent.ConfigHash != hash {
		return intent, errors.New("series retry configuration differs from its retained binding")
	}
	return uploadSeries(ctx, store, client, intent, in.AttemptKey, &in)
}

// No PUT, listing, delete, overwrite or default retry is available here. A
// separate configured reader must retrieve the independently supplied version.
func ReconcileRecoverySeries(ctx context.Context, store *database.Store, configFile string, fixtureAllowed bool, id, version string, identities []age.Identity) (database.RecoverySeriesIntent, error) {
	_, client, _, hash, err := LoadOffsiteConfig(configFile, fixtureAllowed)
	if err != nil {
		return database.RecoverySeriesIntent{}, err
	}
	intent, err := store.RecoverySeriesForCustody(ctx, id)
	if err != nil {
		return intent, err
	}
	if intent.ConfigHash != hash || len(identities) == 0 || (intent.State != "UNKNOWN" && intent.State != "HANDOFF" && intent.State != "VERIFIED") {
		return intent, errors.New("exact unresolved original, matching reader binding and private custody identities required")
	}
	if intent.State == "VERIFIED" && intent.Version != version {
		return intent, errors.New("a different version cannot replace the verified original")
	}
	directory, err := os.MkdirTemp("", "society-series-get-")
	if err != nil {
		return intent, err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "reservation.age")
	if err = client.GetVersion(ctx, intent.ObjectKey, version, path, intent.CipherSHA256, intent.CipherBytes, time.Now()); err != nil {
		return intent, err
	}
	cipher, err := objectstorage.ReadPrivate(path, 65536)
	if err != nil {
		return intent, err
	}
	reader, err := age.Decrypt(bytes.NewReader(cipher), identities...)
	if err != nil {
		return intent, errors.New("supplied custody cannot decrypt the exact series original")
	}
	body, err := io.ReadAll(io.LimitReader(reader, 8193))
	if err != nil || len(body) > 8192 || !bytes.Equal(body, []byte(intent.ManifestJSON)) {
		return intent, errors.New("retrieved reservation manifest differs from its retained source, policy, operation or namespace")
	}
	return store.FinishRecoverySeriesHandoff(ctx, intent.OperationID, intent.AttemptKey, "VERIFIED", "CUSTODY_VERIFIED", version)
}
