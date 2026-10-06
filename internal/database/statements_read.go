package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type StatementDetail struct {
	StatementFile
	Staff            bool                   `json:"staff"`
	Events           []StatementEvent       `json:"events"`
	EventTotal       int                    `json:"event_total"`
	Versions         []StatementFile        `json:"versions"`
	VersionTotal     int                    `json:"version_total"`
	Publications     []StatementPublication `json:"publications"`
	PublicationTotal int                    `json:"publication_total"`
	EventPage        int                    `json:"event_page"`
	VersionPage      int                    `json:"version_page"`
	PublicationPage  int                    `json:"publication_page"`
	PageSize         int                    `json:"page_size"`
}

func beginStatementRead(ctx context.Context, s *Store, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && p.MFAPending {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func statementCapabilities(ctx context.Context, q identityReader, p Principal, x *StatementFile) error {
	x.CanDownload = x.Validation == "AVAILABLE"
	if !statementStaff(p) {
		x.Replaces, x.UploaderID, x.ReviewerID = "", "", ""
		x.CreatedAt, x.ExpiresAt, x.UploadedAt, x.AvailableAt, x.ReviewedAt = 0, 0, 0, 0, 0
		x.Current = true
		x.Version = 0
		return nil
	}
	x.CanUpload = x.State == "PENDING" && x.Validation == "PENDING" && x.UploadedAt == 0 && x.ExpiresAt > time.Now().Unix() && x.UploaderID == p.ID
	x.CanReview = x.State == "PENDING" && x.UploaderID != p.ID
	x.CanWithdraw = x.State == "PENDING" && x.UploaderID == p.ID
	x.CanRetry = x.State == "PENDING" && x.Validation == "REJECTED" && x.ValidationCode == "CHECKS_UNAVAILABLE"
	var pendingFile, pendingPublication bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM statement_files WHERE group_id=? AND state='PENDING' AND validation IN('PENDING','VALIDATING','AVAILABLE')),EXISTS(SELECT 1 FROM statement_publications WHERE group_id=? AND state='PENDING')`, x.GroupID, x.GroupID).Scan(&pendingFile, &pendingPublication); err != nil {
		return err
	}
	x.CanReplace = x.Current && x.State == "APPROVED" && !pendingFile
	x.CanPublish = x.Current && x.State == "APPROVED" && x.Validation == "AVAILABLE" && !pendingPublication
	return nil
}
func (s *Store) StatementsFor(ctx context.Context, token, query, kind, state string, page int) (StatementPage, error) {
	out := StatementPage{Items: []StatementFile{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || len(query) > 100 || (kind != "" && kind != "INCOME" && kind != "BALANCE" && kind != "BUDGET" && kind != "AUDIT") || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "WITHDRAWN") {
		return out, ErrInvalid
	}
	tx, p, err := beginStatementRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	out.CanPrepare = statementStaff(p)
	scope, args := statementScope(p)
	where := " WHERE " + scope
	if statementStaff(p) {
		where += ` AND x.id=(SELECT latest.id FROM statement_files latest WHERE latest.group_id=x.group_id ORDER BY latest.created_at DESC,latest.rowid DESC LIMIT 1)`
	}
	if query != "" {
		where += " AND (x.title LIKE :query OR x.prepared_by LIKE :query)"
		args = append(args, sql.Named("query", "%"+query+"%"))
	}
	if kind != "" {
		where += " AND x.kind=:kind"
		args = append(args, sql.Named("kind", kind))
	}
	if state != "" {
		where += " AND x.state=:state"
		args = append(args, sql.Named("state", state))
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statement_files x JOIN statement_groups g ON g.id=x.group_id`+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, statementFileSelect+where+" ORDER BY x.created_at DESC,x.rowid DESC LIMIT 12 OFFSET :offset", append(args, sql.Named("offset", (page-1)*12))...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanStatement(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range out.Items {
		if err = statementCapabilities(ctx, tx, p, &out.Items[i]); err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}
func (s *Store) StatementFor(ctx context.Context, token, id string, eventPage, versionPage, publicationPage int) (StatementDetail, error) {
	out := StatementDetail{Events: []StatementEvent{}, Versions: []StatementFile{}, Publications: []StatementPublication{}, EventPage: eventPage, VersionPage: versionPage, PublicationPage: publicationPage, PageSize: 20}
	for _, page := range []int{eventPage, versionPage, publicationPage} {
		if page < 1 || page > 10000 {
			return out, ErrInvalid
		}
	}
	tx, p, err := beginStatementRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	scope, args := statementScope(p)
	out.StatementFile, err = scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=:id AND "+scope, append(args, sql.Named("id", id))...))
	if err != nil {
		return out, err
	}
	out.Staff = statementStaff(p)
	if err = statementCapabilities(ctx, tx, p, &out.StatementFile); err != nil {
		return out, err
	}
	if !out.Staff {
		return out, tx.Commit()
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statement_events WHERE file_id=?`, id).Scan(&out.EventTotal); err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.action,COALESCE(u.display_name,'Content checker'),e.reason,e.occurred_at,e.version FROM statement_events e LEFT JOIN users u ON u.id=e.actor_id WHERE e.file_id=? ORDER BY e.version DESC LIMIT 20 OFFSET ?`, id, (eventPage-1)*20)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e StatementEvent
		if err = rows.Scan(&e.Action, &e.Actor, &e.Reason, &e.At, &e.Version); err != nil {
			rows.Close()
			return out, err
		}
		out.Events = append(out.Events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statement_files WHERE group_id=?`, out.GroupID).Scan(&out.VersionTotal); err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, statementFileSelect+" WHERE x.group_id=? ORDER BY x.created_at DESC,x.rowid DESC LIMIT 20 OFFSET ?", out.GroupID, (versionPage-1)*20)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanStatement(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Versions = append(out.Versions, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range out.Versions {
		if err = statementCapabilities(ctx, tx, p, &out.Versions[i]); err != nil {
			return out, err
		}
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM statement_publications WHERE group_id=?`, out.GroupID).Scan(&out.PublicationTotal); err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, statementPublicationSelect+" WHERE sp.group_id=? ORDER BY sp.proposed_at DESC,sp.rowid DESC LIMIT 20 OFFSET ?", out.GroupID, (publicationPage-1)*20)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanStatementPublication(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Publications = append(out.Publications, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range out.Publications {
		x := &out.Publications[i]
		file, e := scanStatement(tx.QueryRowContext(ctx, statementFileSelect+" WHERE x.id=?", x.FileID))
		if e != nil {
			return out, e
		}
		x.File = file
		x.Events = []StatementEvent{}
		// A proposal has at most three transitions: proposed, decided, then
		// revoked or superseded. Keep this history bounded independently of
		// the paged proposal register.
		events, e := tx.QueryContext(ctx, `SELECT e.action,u.display_name,e.reason,e.occurred_at,e.version FROM statement_publication_events e JOIN users u ON u.id=e.actor_id WHERE e.publication_id=? ORDER BY e.version DESC LIMIT 3`, x.ID)
		if e != nil {
			return out, e
		}
		for events.Next() {
			var event StatementEvent
			if e = events.Scan(&event.Action, &event.Actor, &event.Reason, &event.At, &event.Version); e != nil {
				events.Close()
				return out, e
			}
			x.Events = append(x.Events, event)
		}
		e = events.Err()
		events.Close()
		if e != nil {
			return out, e
		}
		x.CanWithdraw = x.State == "PENDING" && x.ProposerID == p.ID
		x.CanDecline = x.State == "PENDING" && x.ProposerID != p.ID
		x.CanRevoke = x.Current && x.State == "PUBLISHED"
		if x.State == "PENDING" {
			preview, e := resolveStatementPublication(ctx, tx, StatementPublicationInput{FileID: file.ID, FileVersion: file.Version, Target: x.Target})
			if errors.Is(e, sql.ErrNoRows) || errors.Is(e, ErrConflict) {
				x.Problem = "SOURCE_CHANGED"
			} else if errors.Is(e, ErrInvalid) {
				x.Problem = "AUDIENCE_CHANGED"
			} else if e != nil {
				return out, e
			} else if preview.PreviewHash != x.PreviewHash {
				x.Problem = "AUDIENCE_CHANGED"
			}
			current, e := messageOperatorCurrent(ctx, tx, x.ProposerID, "RECEIPT")
			if e != nil {
				return out, e
			}
			if !current {
				x.Problem = "PROPOSER_AUTHORITY_ENDED"
			}
			x.CanApprove = x.ProposerID != p.ID && x.Problem == ""
		}
	}
	return out, tx.Commit()
}
