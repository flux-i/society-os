package documents

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"society.local/portal/internal/database"
)

func RunStatementValidation(ctx context.Context, db *database.Store, logger *slog.Logger) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := db.ClaimStatementCheck(ctx, time.Now())
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("statement_check_claim_failed")
				}
				continue
			}
			sum := sha256.Sum256(job.Bytes)
			mime, code := "", "CHECKSUM_MISMATCH"
			if hex.EncodeToString(sum[:]) == job.SHA256 {
				mime, code = ValidateStatementOriginal(ctx, job.Filename, job.Bytes)
			}
			if ctx.Err() != nil {
				return
			}
			if err = db.FinishStatementCheck(ctx, job, mime, code, time.Now()); err != nil {
				logger.Warn("statement_check_completion_failed")
			}
		}
	}
}
