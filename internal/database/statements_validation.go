package database

import (
	"context"
	"database/sql"
	"time"
)

type StatementCheckJob struct {
	ID         string
	Filename   string
	SHA256     string
	Bytes      []byte
	LeaseToken string
	Version    int
}

func (s *Store) ClaimStatementCheck(ctx context.Context, now time.Time) (StatementCheckJob, error) {
	var job StatementCheckJob
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return job, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE app_metadata SET value=value WHERE key='fixture_version'`); err != nil {
		return job, err
	}
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+` WHERE x.state='PENDING' AND ((x.validation='PENDING' AND (x.uploaded_at>0 OR x.expires_at<=?)) OR (x.validation='VALIDATING' AND x.lease_until<=?)) ORDER BY x.created_at,x.id LIMIT 1`, now.Unix(), now.Unix()))
	if err != nil {
		return job, err
	}
	var attempts int
	if err = tx.QueryRowContext(ctx, `SELECT attempts FROM statement_files WHERE id=?`, x.ID).Scan(&attempts); err != nil {
		return job, err
	}
	action, reason := "", ""
	if x.UploadedAt == 0 {
		action, reason = "ABANDONED", "The original upload reservation expired"
		_, err = tx.ExecContext(ctx, `UPDATE statement_files SET validation='ABANDONED',version=version+1 WHERE id=?`, x.ID)
	} else if attempts >= 3 {
		action, reason = "REJECTED", "Content-check attempts are unavailable; explicit retry required"
		_, err = tx.ExecContext(ctx, `UPDATE statement_files SET validation='REJECTED',validation_code='CHECKS_UNAVAILABLE',lease_token='',lease_until=0,version=version+1 WHERE id=?`, x.ID)
	}
	if action != "" {
		if err != nil {
			return job, err
		}
		x.Version++
		if err = statementFileEvent(ctx, tx, "", x, action, reason, now.Unix()); err != nil {
			return job, err
		}
		if err = tx.Commit(); err != nil {
			return job, err
		}
		return job, sql.ErrNoRows
	}
	job = StatementCheckJob{ID: x.ID, Filename: x.Filename, SHA256: x.SHA256, Version: x.Version + 1, LeaseToken: randomToken()}
	if err = tx.QueryRowContext(ctx, `SELECT original_bytes FROM statement_objects WHERE file_id=?`, x.ID).Scan(&job.Bytes); err != nil {
		return job, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE statement_files SET validation='VALIDATING',attempts=attempts+1,lease_token=?,lease_until=?,version=version+1 WHERE id=?`, job.LeaseToken, now.Add(30*time.Second).Unix(), x.ID)
	if err != nil {
		return job, err
	}
	x.Version = job.Version
	if err = statementFileEvent(ctx, tx, "", x, "VALIDATING", "Original content checks claimed", now.Unix()); err != nil {
		return job, err
	}
	return job, tx.Commit()
}

func (s *Store) FinishStatementCheck(ctx context.Context, job StatementCheckJob, contentType, code string, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE app_metadata SET value=value WHERE key='fixture_version'`); err != nil {
		return err
	}
	x, err := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+` WHERE x.id=? AND x.validation='VALIDATING' AND x.lease_token=? AND x.lease_until>? AND x.version=? AND x.state='PENDING'`, job.ID, job.LeaseToken, now.Unix(), job.Version))
	if err == sql.ErrNoRows {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	status, action, available := "REJECTED", "REJECTED", int64(0)
	if code == "" {
		if contentType != "application/pdf" && contentType != "text/csv; charset=utf-8" && contentType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			return ErrInvalid
		}
		status, action, available = "AVAILABLE", "VALIDATED", now.Unix()
	} else {
		contentType = ""
	}
	_, err = tx.ExecContext(ctx, `UPDATE statement_files SET validation=?,validation_code=?,content_type=?,available_at=?,lease_token='',lease_until=0,version=version+1 WHERE id=?`, status, code, contentType, available, job.ID)
	if err != nil {
		return err
	}
	x.Version++
	if err = statementFileEvent(ctx, tx, "", x, action, "Original content check result: "+status, now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
