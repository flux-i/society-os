package database

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrRecoveryRequired = errors.New("current recovery review is required")

func IsRecoveryRequired(err error) bool {
	return errors.Is(err, ErrRecoveryRequired) || (err != nil && strings.Contains(err.Error(), "society_recovery_review_required"))
}

// RecoveryOrigin is an offline journal identity. It never establishes authority
// from checkpoint roles, supplies a provider acknowledgement or releases a host.
type RecoveryOrigin struct {
	ID                string `json:"id"`
	FormatVersion     int    `json:"format_version"`
	SourceSHA256      string `json:"source_sha256"`
	SourceBytes       int64  `json:"source_bytes"`
	SourceSchema      int    `json:"source_schema"`
	SourceApplication string `json:"source_application"`
	SourceCreatedAt   int64  `json:"source_created_at"`
	RestoredAt        int64  `json:"restored_at"`
}

func recoveryRequired(ctx context.Context, q queryer) (bool, error) {
	var required bool
	err := q.QueryRowContext(ctx, "SELECT required FROM recovery_quarantine").Scan(&required)
	// Historical migration fixtures run current readers before this migration.
	// A missing view in the current schema remains an error and fails closed.
	if err != nil && strings.Contains(err.Error(), "no such table: recovery_quarantine") {
		var schema int
		if older := q.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&schema); older == nil && schema < 29 {
			return false, nil
		}
	}
	return required, err
}

func requireNoRecovery(ctx context.Context, q queryer) error {
	required, err := recoveryRequired(ctx, q)
	if err != nil {
		return err
	}
	if required {
		return ErrRecoveryRequired
	}
	return nil
}

func (s *Store) RecoveryRequired(ctx context.Context) (bool, error) {
	return recoveryRequired(ctx, s.DB)
}

// Only the verified fresh restore destination calls this offline entry point.
// One transaction retains its origin and every unresolved external identity.
func (s *Store) QuarantineRecovery(ctx context.Context, origin RecoveryOrigin, now time.Time) (RecoveryOrigin, error) {
	if origin.ID != "" || origin.FormatVersion != 1 || !custodyDigest.MatchString(origin.SourceSHA256) ||
		origin.SourceBytes <= 0 || origin.SourceBytes > 512*1024*1024 || origin.SourceSchema != SchemaVersion ||
		!validText(origin.SourceApplication, 1, 80) || origin.SourceCreatedAt <= 0 || origin.RestoredAt != 0 || now.Unix() <= 0 {
		return RecoveryOrigin{}, ErrInvalid
	}
	origin.ID, origin.RestoredAt = randomToken(), now.Unix()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return RecoveryOrigin{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO recovery_epochs(id,format_version,source_sha256,source_bytes,source_schema,source_application,source_created_at,restored_at)
		VALUES(?,?,?,?,?,?,?,?)`, origin.ID, origin.FormatVersion, origin.SourceSHA256, origin.SourceBytes, origin.SourceSchema, origin.SourceApplication, origin.SourceCreatedAt, origin.RestoredAt); err != nil {
		return RecoveryOrigin{}, err
	}
	for _, query := range []string{
		`INSERT INTO recovery_external_jobs SELECT ?,'OFFSITE',id,state,'WITHHELD' FROM offsite_backup_jobs WHERE state!='VERIFIED'`,
		`INSERT INTO recovery_external_jobs SELECT ?,'MESSAGE',id,state,'WITHHELD' FROM message_deliveries WHERE state IN('QUEUED','CLAIMED','FAILED','UNKNOWN')`,
	} {
		if _, err = tx.ExecContext(ctx, query, origin.ID); err != nil {
			return RecoveryOrigin{}, err
		}
	}
	return origin, tx.Commit()
}
