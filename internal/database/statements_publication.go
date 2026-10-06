package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type StatementPublicationInput struct {
	OperationKey string        `json:"operation_key"`
	FileID       string        `json:"file_id"`
	FileVersion  int           `json:"file_version"`
	Target       MessageTarget `json:"target"`
	PreviewHash  string        `json:"preview_hash"`
	Reason       string        `json:"reason"`
	Confirmed    bool          `json:"confirmed"`
}
type StatementPublicationPreview struct {
	File         StatementFile `json:"file"`
	Target       MessageTarget `json:"target"`
	TargetPeople int           `json:"target_people"`
	PreviewHash  string        `json:"preview_hash"`
}
type StatementPublication struct {
	ID           string           `json:"id"`
	GroupID      string           `json:"-"`
	FileID       string           `json:"file_id"`
	Target       MessageTarget    `json:"target"`
	TargetPeople int              `json:"target_people"`
	PreviewHash  string           `json:"preview_hash"`
	State        string           `json:"state"`
	ProposerID   string           `json:"proposer_id"`
	ProposedAt   int64            `json:"proposed_at"`
	ReviewerID   string           `json:"reviewer_id"`
	ReviewedAt   int64            `json:"reviewed_at"`
	Version      int              `json:"version"`
	Current      bool             `json:"current"`
	CanApprove   bool             `json:"can_approve"`
	CanDecline   bool             `json:"can_decline"`
	CanWithdraw  bool             `json:"can_withdraw"`
	CanRevoke    bool             `json:"can_revoke"`
	Problem      string           `json:"problem"`
	Events       []StatementEvent `json:"events,omitempty"`
	File         StatementFile    `json:"file"`
}

const statementPublicationSelect = `SELECT sp.id,sp.group_id,sp.file_id,sp.target_json,sp.target_people,sp.preview_hash,sp.state,sp.proposed_by,sp.proposed_at,COALESCE(sp.reviewed_by,''),sp.reviewed_at,sp.version,COALESCE(sg.current_publication_id=sp.id,0) FROM statement_publications sp JOIN statement_groups sg ON sg.id=sp.group_id`

func scanStatementPublication(row interface{ Scan(...any) error }) (StatementPublication, error) {
	var p StatementPublication
	var target string
	err := row.Scan(&p.ID, &p.GroupID, &p.FileID, &target, &p.TargetPeople, &p.PreviewHash, &p.State, &p.ProposerID, &p.ProposedAt, &p.ReviewerID, &p.ReviewedAt, &p.Version, &p.Current)
	if err == nil {
		err = json.Unmarshal([]byte(target), &p.Target)
	}
	return p, err
}
func statementAudienceSQL(alias string) string {
	return `EXISTS(SELECT 1 FROM flat_memberships sm JOIN flats sf ON sf.id=sm.flat_id JOIN buildings sb ON sb.id=sf.building_id WHERE sm.resident_id=:resident AND sm.start_date<=:day AND (sm.end_date IS NULL OR sm.end_date>:day) AND (json_extract(` + alias + `.target_json,'$.kind')='ALL' OR (json_extract(` + alias + `.target_json,'$.kind')='OWNERS' AND sm.relationship='OWNER') OR (json_extract(` + alias + `.target_json,'$.kind')='TENANTS' AND sm.relationship='TENANT') OR (json_extract(` + alias + `.target_json,'$.kind')='WING' AND sb.code=json_extract(` + alias + `.target_json,'$.wing')) OR (json_extract(` + alias + `.target_json,'$.kind')='HOMES' AND sm.flat_id IN(SELECT value FROM json_each(` + alias + `.target_json,'$.ids'))) OR (json_extract(` + alias + `.target_json,'$.kind')='PEOPLE' AND sm.resident_id IN(SELECT value FROM json_each(` + alias + `.target_json,'$.ids')))))`
}
func statementScope(p Principal) (string, []any) {
	return `(:staff=1 OR (x.validation='AVAILABLE' AND x.state='APPROVED' AND EXISTS(SELECT 1 FROM statement_publications sp WHERE sp.id=g.current_publication_id AND sp.file_id=x.id AND sp.state='PUBLISHED' AND ` + statementAudienceSQL("sp") + `)))`, []any{sql.Named("staff", statementStaff(p)), sql.Named("resident", p.ResidentID), sql.Named("day", today())}
}
func normalizedStatementTarget(target MessageTarget) (MessageTarget, error) {
	if len(target.IDs) > 200 {
		return target, ErrInvalid
	}
	switch target.Kind {
	case "ALL", "OWNERS", "TENANTS":
		if target.Wing != "" || len(target.IDs) > 0 {
			return target, ErrInvalid
		}
	case "WING":
		if (target.Wing != "A" && target.Wing != "B" && target.Wing != "C") || len(target.IDs) > 0 {
			return target, ErrInvalid
		}
	case "HOMES", "PEOPLE":
		if target.Wing != "" || len(target.IDs) == 0 {
			return target, ErrInvalid
		}
	default:
		return target, ErrInvalid
	}
	target.IDs = append([]string{}, target.IDs...)
	sort.Strings(target.IDs)
	for i, id := range target.IDs {
		if id == "" || len(id) > 100 || (i > 0 && id == target.IDs[i-1]) {
			return target, ErrInvalid
		}
	}
	return target, nil
}
func resolveStatementPublication(ctx context.Context, q identityReader, in StatementPublicationInput) (StatementPublicationPreview, error) {
	var out StatementPublicationPreview
	x, err := scanStatement(q.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", in.FileID))
	if err != nil {
		return out, err
	}
	if x.Validation != "AVAILABLE" || x.State != "APPROVED" || !x.Current || x.Version != in.FileVersion {
		return out, ErrConflict
	}
	target, err := normalizedStatementTarget(in.Target)
	if err != nil {
		return out, err
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return out, err
	}
	for _, id := range target.IDs {
		var exists bool
		if target.Kind == "HOMES" {
			err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)`, id).Scan(&exists)
		} else {
			err = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, id, today(), today()).Scan(&exists)
		}
		if err != nil {
			return out, err
		}
		if !exists {
			return out, ErrConflict
		}
	}
	// Materialize a bounded identity set so an approval notices changed members,
	// even when the number of residents is unchanged.
	where := strings.ReplaceAll(statementAudienceSQL("audience"), "sm.resident_id=:resident", "sm.resident_id=r.id")
	rows, err := q.QueryContext(ctx, `SELECT r.id FROM residents r CROSS JOIN (SELECT ? AS target_json) audience WHERE `+where+` ORDER BY r.id LIMIT 2001`, string(encoded), sql.Named("day", today()))
	if err != nil {
		return out, err
	}
	people := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		people = append(people, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(people) == 0 || len(people) > 2000 {
		return out, invalid("Choose an audience with 1–2000 current people.")
	}
	frozen, err := json.Marshal(struct {
		ID, SHA, Title, Kind, Start, End, Preparer, Source string
		Version                                            int
		Target                                             MessageTarget
		People                                             []string
	}{x.ID, x.SHA256, x.Title, x.Kind, x.PeriodStart, x.PeriodEnd, x.PreparedBy, x.Source, x.Version, target, people})
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(frozen)
	out = StatementPublicationPreview{File: x, Target: target, TargetPeople: len(people), PreviewHash: hex.EncodeToString(hash[:])}
	return out, nil
}
func statementPublicationEvent(ctx context.Context, tx *sql.Tx, p Principal, x StatementPublication, action, reason string, at int64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO statement_publication_events VALUES(?,?,?,?,?,?,?)`, randomToken(), x.ID, p.ID, action, reason, at, x.Version); err != nil {
		return err
	}
	return appendAudit(ctx, tx, p.ID, "", "STATEMENT_PUBLICATION_"+action, "Deliberate statement publication", map[string]any{}, map[string]any{"id": x.ID, "version": x.Version, "state": x.State})
}

func (s *Store) PreviewStatementPublication(ctx context.Context, token string, in StatementPublicationInput) (StatementPublicationPreview, error) {
	tx, p, err := beginStatementRead(ctx, s, token)
	if err != nil {
		return StatementPublicationPreview{}, err
	}
	defer tx.Rollback()
	if !statementStaff(p) {
		return StatementPublicationPreview{}, ErrForbidden
	}
	out, err := resolveStatementPublication(ctx, tx, in)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) ProposeStatementPublication(ctx context.Context, token string, in StatementPublicationInput) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 5, 300) || !checksumPattern.MatchString(in.PreviewHash) {
		return "", ErrInvalid
	}
	target, err := normalizedStatementTarget(in.Target)
	if err != nil {
		return "", err
	}
	in.Target = target
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = statementAuthority(p); err != nil {
		return "", err
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "STATEMENT_PUBLICATION", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	preview, err := resolveStatementPublication(ctx, tx, in)
	if err != nil {
		return "", err
	}
	if preview.PreviewHash != in.PreviewHash {
		return "", ErrConflict
	}
	var pending bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM statement_publications WHERE group_id=? AND state='PENDING')`, preview.File.GroupID).Scan(&pending); err != nil {
		return "", err
	}
	if pending {
		return "", ErrConflict
	}
	id, now := randomToken(), time.Now().Unix()
	encoded, err := json.Marshal(in.Target)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO statement_publications(id,group_id,file_id,target_json,preview_hash,target_people,proposed_by,proposed_at) VALUES(?,?,?,?,?,?,?,?)`, id, preview.File.GroupID, in.FileID, string(encoded), in.PreviewHash, preview.TargetPeople, p.ID, now)
	if err != nil {
		return "", err
	}
	x := StatementPublication{ID: id, Version: 1, State: "PENDING"}
	if err = statementPublicationEvent(ctx, tx, p, x, "PROPOSED", in.Reason, now); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) DecideStatementPublication(ctx context.Context, token, id string, in StatementAction) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 5, 300) || in.Version < 1 || (in.Action != "PUBLISHED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "REVOKED") {
		return "", ErrInvalid
	}
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = statementAuthority(p); err != nil {
		return "", err
	}
	x, err := scanStatementPublication(tx.QueryRowContext(ctx, statementPublicationSelect+" WHERE sp.id=?", id))
	if err != nil {
		return "", err
	}
	if in.Action == "WITHDRAWN" {
		if x.ProposerID != p.ID {
			return "", ErrForbidden
		}
	} else if in.Action != "REVOKED" && x.ProposerID == p.ID {
		return "", ErrForbidden
	}
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "STATEMENT_PUBLISH_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	if in.Action == "REVOKED" {
		if x.State != "PUBLISHED" || !x.Current {
			return "", ErrConflict
		}
	} else if x.State != "PENDING" {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	if in.Action == "PUBLISHED" {
		file, e := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", x.FileID))
		if e != nil {
			return "", e
		}
		preview, e := resolveStatementPublication(ctx, tx, StatementPublicationInput{FileID: x.FileID, FileVersion: file.Version, Target: x.Target})
		if e != nil {
			return "", e
		}
		if preview.PreviewHash != x.PreviewHash {
			return "", ErrConflict
		}
		// Publication is also suppressed when its preparer's finance authority ends.
		proposerCurrent, e := messageOperatorCurrent(ctx, tx, x.ProposerID, "RECEIPT")
		if e != nil {
			return "", e
		}
		if !proposerCurrent {
			return "", ErrForbidden
		}
		var old string
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(current_publication_id,'') FROM statement_groups WHERE id=?`, x.GroupID).Scan(&old); err != nil {
			return "", err
		}
		if old != "" {
			prior, e := scanStatementPublication(tx.QueryRowContext(ctx, statementPublicationSelect+" WHERE sp.id=?", old))
			if e != nil {
				return "", e
			}
			if _, err = tx.ExecContext(ctx, `UPDATE statement_publications SET state='SUPERSEDED',version=version+1 WHERE id=?`, old); err != nil {
				return "", err
			}
			prior.Version++
			prior.State = "SUPERSEDED"
			if err = statementPublicationEvent(ctx, tx, p, prior, "SUPERSEDED", "Replaced by a separately approved publication", now); err != nil {
				return "", err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE statement_publications SET state=?,reviewed_by=?,reviewed_at=?,version=version+1 WHERE id=?`, in.Action, p.ID, now, id); err != nil {
		return "", err
	}
	x.Version++
	x.State = in.Action
	if in.Action == "PUBLISHED" {
		_, err = tx.ExecContext(ctx, `UPDATE statement_groups SET current_publication_id=? WHERE id=?`, id, x.GroupID)
	} else if in.Action == "REVOKED" {
		_, err = tx.ExecContext(ctx, `UPDATE statement_groups SET current_publication_id=NULL WHERE id=? AND current_publication_id=?`, x.GroupID, id)
	}
	if err != nil {
		return "", err
	}
	if err = statementPublicationEvent(ctx, tx, p, x, in.Action, in.Reason, now); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
