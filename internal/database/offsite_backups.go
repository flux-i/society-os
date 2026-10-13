package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrBackupProtection = errors.New("a current verified recovery point is required")

func IsBackupProtectionError(err error) bool {
	return errors.Is(err, ErrBackupProtection) || (err != nil && strings.Contains(err.Error(), "society_backup_protection_required"))
}

const OffsiteInterval = 30 * time.Minute
const OffsiteLease = 5 * time.Minute
const OffsiteStagingSlots = 4

type OffsiteJob struct {
	ID, ObjectKey, ConfigHash, State, Lease, SHA256, ObjectVersion, ErrorCode string
	CapturedAt, AvailableAt, LeaseUntil, Bytes, AcknowledgedAt                int64
	Attempts                                                                  int
}

const offsiteJobSelect = `SELECT id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until,encrypted_sha256,encrypted_bytes,object_version,acknowledged_at,error_code FROM offsite_backup_jobs `

func scanOffsiteJob(row interface{ Scan(...any) error }) (OffsiteJob, error) {
	var j OffsiteJob
	err := row.Scan(&j.ID, &j.ObjectKey, &j.ConfigHash, &j.CapturedAt, &j.State, &j.Attempts, &j.AvailableAt, &j.Lease, &j.LeaseUntil, &j.SHA256, &j.Bytes, &j.ObjectVersion, &j.AcknowledgedAt, &j.ErrorCode)
	return j, err
}

func offsiteEvent(ctx context.Context, tx *sql.Tx, id, action, actor, note string, now int64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO offsite_backup_events(job_id,action,actor_kind,note,occurred_at) VALUES(?,?,?,?,?)", optionalID(id), action, actor, note, now)
	return err
}

// Called only by the explicitly configured local operator command, never an API.
// Credentials may rotate; the destination and public recipients remain bound.
func (s *Store) BindOffsitePolicy(ctx context.Context, configHash, mode string, now time.Time) error {
	if len(configHash) != 64 || (mode != "FIXTURE" && mode != "S3") || now.Unix() <= 0 {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if mode == "FIXTURE" {
		var fictional bool
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT value FROM app_metadata WHERE key='data_kind'),'')='synthetic' AND NOT EXISTS(SELECT 1 FROM workspace_setup WHERE mode='LOCAL_WORKSPACE')").Scan(&fictional); err != nil {
			return err
		}
		if !fictional {
			return fmt.Errorf("%w: non-fictional workspaces require independent S3 protection", ErrInvalid)
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=highest_clock_at WHERE id=1"); err != nil {
		return err
	}
	var hash, existingMode string
	err = tx.QueryRowContext(ctx, "SELECT config_hash,storage_mode FROM offsite_policy WHERE id=1").Scan(&hash, &existingMode)
	if err == nil {
		if hash != configHash || mode != existingMode {
			return fmt.Errorf("%w: off-site destination or public custody material differs from its retained binding", ErrConflict)
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO offsite_policy VALUES(1,?,?,?,?)", configHash, mode, now.Unix(), now.Unix()); err != nil {
		return err
	}
	if err = offsiteEvent(ctx, tx, "", "CONFIGURED", "SYSTEM", "Protection configured; a confirmed recovery copy is still required", now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) OffsiteConfigured(ctx context.Context) (bool, error) {
	var bound bool
	err := s.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM offsite_policy)").Scan(&bound)
	return bound, err
}

// One writer reservation covers schedule, competing claims and expired handoffs.
// An expired upload is uncertain; it is never silently replayed as a new upload.
func (s *Store) ClaimOffsite(ctx context.Context, configHash, prefix string, now time.Time) (OffsiteJob, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return OffsiteJob{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=highest_clock_at WHERE id=1"); err != nil {
		return OffsiteJob{}, err
	}
	if err = requireNoRecovery(ctx, tx); err != nil {
		return OffsiteJob{}, err
	}
	var current string
	var highest int64
	if err = tx.QueryRowContext(ctx, "SELECT config_hash,highest_clock_at FROM offsite_policy WHERE id=1").Scan(&current, &highest); err != nil {
		return OffsiteJob{}, err
	}
	if configHash != current {
		return OffsiteJob{}, ErrConflict
	}
	if now.Unix() < highest {
		return OffsiteJob{}, ErrBackupProtection
	}
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=? WHERE id=1", now.Unix()); err != nil {
		return OffsiteJob{}, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,state FROM offsite_backup_jobs WHERE (state IN('CAPTURING','UPLOADING') OR (state='SEALED' AND attempts>=5)) AND lease_until<=?", now.Unix())
	if err != nil {
		return OffsiteJob{}, err
	}
	type expired struct{ id, state string }
	var expiredJobs []expired
	for rows.Next() {
		var x expired
		if err = rows.Scan(&x.id, &x.state); err != nil {
			rows.Close()
			return OffsiteJob{}, err
		}
		expiredJobs = append(expiredJobs, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return OffsiteJob{}, err
	}
	for _, x := range expiredJobs {
		state := "FAILED"
		if x.state == "UPLOADING" {
			state = "UNCERTAIN"
		}
		if _, err = tx.ExecContext(ctx, "UPDATE offsite_backup_jobs SET state=?,lease_token='',lease_until=0,error_code='LEASE_EXPIRED',available_at=? WHERE id=?", state, now.Unix(), x.id); err != nil {
			return OffsiteJob{}, err
		}
		note := "The previous copy ended before preparation finished; a bounded retry is available"
		if state == "UNCERTAIN" {
			note = "The previous upload ended without a confirmed result; an independent custody check is needed"
		} else if x.state == "SEALED" {
			note = "Preparation ended after the maximum attempts; the encrypted original remains retained"
		}
		if err = offsiteEvent(ctx, tx, x.id, state, "SYSTEM", note, now.Unix()); err != nil {
			return OffsiteJob{}, err
		}
	}
	var active bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM offsite_backup_jobs WHERE state IN('CAPTURING','SEALED','UPLOADING') AND lease_until>?)", now.Unix()).Scan(&active); err != nil {
		return OffsiteJob{}, err
	}
	if active {
		return OffsiteJob{}, commitOffsiteIdle(tx)
	}
	j, err := scanOffsiteJob(tx.QueryRowContext(ctx, offsiteJobSelect+"WHERE (state='FAILED' AND attempts<5 AND available_at<=?) OR (state='SEALED' AND attempts<5 AND lease_until<=?) ORDER BY captured_at,id LIMIT 1", now.Unix(), now.Unix()))
	lease := randomToken()
	if err == nil {
		j.State = "CAPTURING"
		if j.SHA256 != "" {
			j.State = "UPLOADING"
		}
		j.Attempts++
		j.Lease, j.LeaseUntil = lease, now.Add(OffsiteLease).Unix()
		if _, err = tx.ExecContext(ctx, "UPDATE offsite_backup_jobs SET state=?,attempts=?,lease_token=?,lease_until=?,error_code='' WHERE id=?", j.State, j.Attempts, lease, j.LeaseUntil, j.ID); err != nil {
			return OffsiteJob{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		var latest int64
		var active, slots int
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(captured_at),0),COALESCE(SUM(state IN('CAPTURING','SEALED','UPLOADING') AND lease_until>?),0),COALESCE(SUM(state!='VERIFIED' AND encrypted_bytes>0 AND NOT EXISTS(SELECT 1 FROM offsite_custody_dispositions d WHERE d.job_id=offsite_backup_jobs.id)),0) FROM offsite_backup_jobs`, now.Unix()).Scan(&latest, &active, &slots); err != nil {
			return OffsiteJob{}, err
		}
		if active > 0 || (latest > 0 && now.Unix()-latest < int64(OffsiteInterval/time.Second)) {
			return OffsiteJob{}, commitOffsiteIdle(tx)
		}
		if slots >= OffsiteStagingSlots {
			return OffsiteJob{}, commitOffsiteIdle(tx)
		}
		id := randomToken()
		j = OffsiteJob{ID: id, ConfigHash: configHash, ObjectKey: prefix + "/" + now.UTC().Format("2006/01/02") + "/" + id + ".tar.gz.age", State: "CAPTURING", CapturedAt: now.Unix(), Attempts: 1, AvailableAt: now.Unix(), Lease: lease, LeaseUntil: now.Add(OffsiteLease).Unix()}
		if _, err = tx.ExecContext(ctx, `INSERT INTO offsite_backup_jobs(id,object_key,config_hash,captured_at,state,attempts,available_at,lease_token,lease_until) VALUES(?,?,?,?,'CAPTURING',1,?,?,?)`, id, j.ObjectKey, configHash, j.CapturedAt, j.AvailableAt, lease, j.LeaseUntil); err != nil {
			return OffsiteJob{}, err
		}
	} else {
		return OffsiteJob{}, err
	}
	if err = offsiteEvent(ctx, tx, j.ID, "CLAIMED", "SYSTEM", "The scheduled recovery copy started", now.Unix()); err != nil {
		return OffsiteJob{}, err
	}
	return j, tx.Commit()
}

func commitOffsiteIdle(tx *sql.Tx) error {
	if err := tx.Commit(); err != nil {
		return err
	}
	return sql.ErrNoRows
}

func (s *Store) SealOffsite(ctx context.Context, j OffsiteJob, hash string, bytes int64, now time.Time) error {
	if len(hash) != 64 || bytes <= 0 || bytes > 256*1024*1024 {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE offsite_backup_jobs SET state='SEALED',encrypted_sha256=?,encrypted_bytes=? WHERE id=? AND state='CAPTURING' AND lease_token=? AND lease_until>?`, hash, bytes, j.ID, j.Lease, now.Unix())
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrConflict
	}
	if err = offsiteEvent(ctx, tx, j.ID, "SEALED", "SYSTEM", "The snapshot passed its checks and was encrypted for custody", now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) BeginOffsiteUpload(ctx context.Context, j OffsiteJob, now time.Time) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE offsite_backup_jobs SET state='UPLOADING' WHERE id=? AND state='SEALED' AND lease_token=? AND lease_until>?`, j.ID, j.Lease, now.Unix())
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrConflict
	}
	return nil
}

// The storage adapter has already validated the signed checksum/version reply.
func (s *Store) FinishOffsite(ctx context.Context, j OffsiteJob, state, code, version string, now time.Time) error {
	if state != "VERIFIED" && state != "FAILED" && state != "UNCERTAIN" {
		return ErrInvalid
	}
	if state == "VERIFIED" && (version == "" || version == "null" || len(version) > 1024 || now.Unix() < j.CapturedAt) {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	acknowledged := int64(0)
	if state == "VERIFIED" {
		acknowledged = now.Unix()
		code = ""
	}
	result, err := tx.ExecContext(ctx, `UPDATE offsite_backup_jobs SET state=?,error_code=?,object_version=?,acknowledged_at=?,lease_token='',lease_until=0,available_at=? WHERE id=? AND state IN('CAPTURING','SEALED','UPLOADING') AND lease_token=? AND lease_until>?`, state, code, version, acknowledged, now.Add(time.Minute).Unix(), j.ID, j.Lease, now.Unix())
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrConflict
	}
	note := "The recovery copy could not be completed"
	if state == "VERIFIED" {
		note = "Storage confirmed the original copy's checksum and version"
	} else if state == "UNCERTAIN" {
		note = "The copy may have arrived; an independent custody check must confirm the original"
	}
	if err = offsiteEvent(ctx, tx, j.ID, state, "SYSTEM", note, now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) OffsiteJobForCustody(ctx context.Context, id string) (OffsiteJob, error) {
	return scanOffsiteJob(s.DB.QueryRowContext(ctx, offsiteJobSelect+"WHERE id=?", id))
}

// This offline boundary is called only after exact-version retrieval, full
// encrypted checksum comparison and independent custody decrypt/verification.
func (s *Store) ReconcileOffsite(ctx context.Context, id, hash string, bytes int64, version, note string, now time.Time) error {
	if !validText(note, 10, 300) || version == "" || version == "null" || len(version) > 1024 {
		return ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE offsite_policy SET highest_clock_at=highest_clock_at WHERE id=1"); err != nil {
		return err
	}
	j, err := scanOffsiteJob(tx.QueryRowContext(ctx, offsiteJobSelect+"WHERE id=?", id))
	if err != nil {
		return err
	}
	if j.SHA256 != hash || j.Bytes != bytes || now.Unix() < j.CapturedAt {
		return ErrConflict
	}
	if j.State == "VERIFIED" {
		if j.ObjectVersion != version {
			return ErrConflict
		}
		return tx.Commit()
	}
	if j.State != "UNCERTAIN" {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE offsite_backup_jobs SET state='VERIFIED',error_code='',object_version=?,acknowledged_at=?,lease_token='',lease_until=0 WHERE id=?`, version, now.Unix(), id); err != nil {
		return err
	}
	if err = offsiteEvent(ctx, tx, id, "RECONCILED", "OFFLINE_CUSTODIAN", note, now.Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

type OffsiteHistory struct {
	ID             string `json:"id"`
	State          string `json:"state"`
	CapturedAt     int64  `json:"captured_at"`
	AcknowledgedAt int64  `json:"acknowledged_at"`
	Attempts       int    `json:"attempts"`
	ErrorCode      string `json:"error_code"`
	ArchivedAt     int64  `json:"archived_at"`
}
type OffsiteStatus struct {
	Configured     bool             `json:"configured"`
	Mode           string           `json:"mode"`
	Phase          string           `json:"phase"`
	FinancePaused  bool             `json:"finance_paused"`
	AsOf           int64            `json:"as_of"`
	CaptureAt      int64            `json:"capture_at"`
	AcknowledgedAt int64            `json:"acknowledged_at"`
	AgeSeconds     *int64           `json:"age_seconds"`
	NextCaptureAt  int64            `json:"next_capture_at"`
	StagingFull    bool             `json:"staging_full"`
	ArchivedCount  int              `json:"archived_count"`
	Counts         map[string]int   `json:"counts"`
	Items          []OffsiteHistory `json:"items"`
	Page           int              `json:"page"`
	PageSize       int              `json:"page_size"`
	Total          int              `json:"total"`
	StatusKey      string           `json:"status_key"`
}

type OffsiteEvent struct {
	Action     string `json:"action"`
	ActorKind  string `json:"actor_kind"`
	Note       string `json:"note"`
	OccurredAt int64  `json:"occurred_at"`
}

type OffsiteDetail struct {
	OffsiteHistory
	CustodyNote string         `json:"custody_note,omitempty"`
	Events      []OffsiteEvent `json:"events"`
	Page        int            `json:"page"`
	PageSize    int            `json:"page_size"`
	EventTotal  int            `json:"event_total"`
}

func (s *Store) OffsiteDetailFor(ctx context.Context, token, id string, page int) (OffsiteDetail, error) {
	out := OffsiteDetail{Events: []OffsiteEvent{}, Page: page, PageSize: 12}
	if !operationPattern.MatchString(id) || page < 1 || page > 10000 {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return out, err
	}
	if !p.CanManageAccounts && !p.CanManageRecords {
		return out, ErrForbidden
	}
	if err = tx.QueryRowContext(ctx, "SELECT id,state,captured_at,acknowledged_at,attempts,error_code,COALESCE((SELECT archived_at FROM offsite_custody_dispositions WHERE job_id=offsite_backup_jobs.id),0),COALESCE((SELECT note FROM offsite_custody_dispositions WHERE job_id=offsite_backup_jobs.id),'') FROM offsite_backup_jobs WHERE id=?", id).Scan(&out.ID, &out.State, &out.CapturedAt, &out.AcknowledgedAt, &out.Attempts, &out.ErrorCode, &out.ArchivedAt, &out.CustodyNote); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM offsite_backup_events WHERE job_id=?", id).Scan(&out.EventTotal); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.EventTotal, out.PageSize)
	rows, err := tx.QueryContext(ctx, "SELECT action,actor_kind,note,occurred_at FROM offsite_backup_events WHERE job_id=? ORDER BY id DESC LIMIT ? OFFSET ?", id, out.PageSize, (out.Page-1)*out.PageSize)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var event OffsiteEvent
		if err = rows.Scan(&event.Action, &event.ActorKind, &event.Note, &event.OccurredAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Events = append(out.Events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func (s *Store) OffsiteStatusFor(ctx context.Context, token, state string, page int) (OffsiteStatus, error) {
	out := OffsiteStatus{Mode: "NONE", Phase: "NOT_CONFIGURED", AsOf: time.Now().Unix(), Page: page, PageSize: 12, Counts: map[string]int{}, Items: []OffsiteHistory{}}
	if page < 1 || page > 10000 || (state != "" && state != "VERIFIED" && state != "FAILED" && state != "UNCERTAIN" && state != "PENDING") {
		return out, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return out, err
	}
	if !p.CanManageAccounts && !p.CanManageRecords {
		return out, ErrForbidden
	}
	var highest int64
	err = tx.QueryRowContext(ctx, "SELECT storage_mode,highest_clock_at FROM offsite_policy WHERE id=1").Scan(&out.Mode, &highest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	out.Configured = err == nil
	rows, err := tx.QueryContext(ctx, "SELECT state,COUNT(*) FROM offsite_backup_jobs GROUP BY state")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var state string
		var count int
		if err = rows.Scan(&state, &count); err != nil {
			rows.Close()
			return out, err
		}
		out.Counts[state] = count
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	var latest, slots int64
	var physicalFull bool
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(captured_at),0),COALESCE(SUM(state!='VERIFIED' AND encrypted_bytes>0 AND NOT EXISTS(SELECT 1 FROM offsite_custody_dispositions d WHERE d.job_id=offsite_backup_jobs.id)),0),COALESCE((SELECT error_code='STAGING_FULL' FROM offsite_backup_jobs ORDER BY captured_at DESC,id DESC LIMIT 1),0) FROM offsite_backup_jobs").Scan(&latest, &slots, &physicalFull); err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM offsite_custody_dispositions").Scan(&out.ArchivedCount); err != nil {
		return out, err
	}
	if latest > 0 {
		out.NextCaptureAt = latest + int64(OffsiteInterval/time.Second)
	}
	out.StagingFull = slots >= OffsiteStagingSlots || physicalFull
	err = tx.QueryRowContext(ctx, "SELECT captured_at,acknowledged_at FROM offsite_backup_jobs WHERE state='VERIFIED' ORDER BY captured_at DESC,id DESC LIMIT 1").Scan(&out.CaptureAt, &out.AcknowledgedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if out.Configured {
		out.Phase, out.FinancePaused, out.AgeSeconds = offsitePhase(out.AsOf, highest, out.CaptureAt, out.AcknowledgedAt)
	} else {
		var allowed bool
		if err = tx.QueryRowContext(ctx, "SELECT allowed FROM offsite_financial_protection").Scan(&allowed); err != nil {
			return out, err
		}
		out.FinancePaused = !allowed
	}
	where := "1=1"
	if state == "PENDING" {
		where = "state IN('CAPTURING','SEALED','UPLOADING')"
	} else if state != "" {
		where = "state=?"
	}
	args := []any{}
	if state != "" && state != "PENDING" {
		args = append(args, state)
	}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM offsite_backup_jobs WHERE "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	args = append(args, out.PageSize, (out.Page-1)*out.PageSize)
	rows, err = tx.QueryContext(ctx, "SELECT id,state,captured_at,acknowledged_at,attempts,error_code,COALESCE((SELECT archived_at FROM offsite_custody_dispositions WHERE job_id=offsite_backup_jobs.id),0) FROM offsite_backup_jobs WHERE "+where+" ORDER BY captured_at DESC,id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item OffsiteHistory
		if err = rows.Scan(&item.ID, &item.State, &item.CapturedAt, &item.AcknowledgedAt, &item.Attempts, &item.ErrorCode, &item.ArchivedAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	key, _ := json.Marshal([]any{out.Configured, out.Mode, out.Phase, out.FinancePaused, out.CaptureAt, out.AcknowledgedAt, out.NextCaptureAt, out.StagingFull, out.Counts, out.ArchivedCount})
	out.StatusKey = TokenHash(string(key))
	return out, tx.Commit()
}

func offsitePhase(now, highest, captured, acknowledged int64) (string, bool, *int64) {
	if highest > now || captured > now || acknowledged > now {
		return "CLOCK_ERROR", true, nil
	}
	if captured == 0 || acknowledged == 0 {
		return "PAUSED", true, nil
	}
	age := now - captured
	if age >= 3600 {
		return "PAUSED", true, &age
	}
	if age >= 2700 {
		return "WARNING", false, &age
	}
	return "HEALTHY", false, &age
}
