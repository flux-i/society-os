package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (s *Store) beginCommunityRead(ctx context.Context, token string, desk bool) (*sql.Tx, Principal, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, Principal{}, e
	}
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e == nil && desk && !p.CanReviewRequests {
		e = ErrForbidden
	}
	if e == nil {
		e = communityReadPrincipal(ctx, tx, p)
	}
	if e != nil {
		tx.Rollback()
		return nil, p, e
	}
	return tx, p, nil
}
func communityVisibleScope(p Principal) (string, []any) {
	if p.CanReviewRequests {
		return "1=1", nil
	}
	return `EXISTS(SELECT 1 FROM json_each(v.homes_json) ch JOIN flat_memberships cm ON cm.flat_id=json_extract(ch.value,'$.id') WHERE cm.resident_id=? AND cm.start_date<=? AND (cm.end_date IS NULL OR cm.end_date>?))`, []any{p.ResidentID, today(), today()}
}

const communityStateSQL = `CASE WHEN v.action='WITHDRAW' THEN 'WITHDRAWN' WHEN v.kind='CONTACT' THEN 'AVAILABLE' WHEN v.action='RESOLVE' THEN 'RESOLVED' WHEN v.start_at>:now THEN 'PLANNED' WHEN v.estimated_end>0 AND v.estimated_end<=:now THEN 'UPDATE_NEEDED' ELSE 'ACTIVE' END`

func communityQuery(p Principal, desk bool, kind, query, state string, now int64) (string, []any) {
	selected := "h.published_version"
	where := "h.published_version IS NOT NULL AND v.action<>'WITHDRAW'"
	stateSQL := communityStateSQL
	args := []any{sql.Named("now", now)}
	if desk {
		selected = "COALESCE(h.pending_version,h.published_version,h.latest_version)"
		where = "1=1"
		stateSQL = "CASE WHEN h.pending_version IS NOT NULL THEN 'PENDING' WHEN h.published_version IS NULL THEN h.decision ELSE " + communityStateSQL + " END"
	} else {
		scope, scoped := communityVisibleScope(p)
		where += " AND " + scope
		args = append(args, scoped...)
	}
	if kind != "" {
		where += " AND h.kind=?"
		args = append(args, kind)
	}
	if query != "" {
		where += " AND (instr(lower(v.title),?)>0 OR instr(lower(v.body),?)>0)"
		term := strings.ToLower(strings.TrimSpace(query))
		args = append(args, term, term)
	}
	querySQL := `SELECT h.id,` + stateSQL + ` AS current_state FROM community_resources h JOIN community_versions v ON v.resource_id=h.id AND v.version=` + selected + ` WHERE ` + where
	if state != "" {
		querySQL = "SELECT * FROM (" + querySQL + ") WHERE current_state=?"
		args = append(args, state)
	}
	return querySQL, args
}
func communityResourceIn(ctx context.Context, q identityReader, p Principal, id string, desk bool, now int64) (CommunityResource, error) {
	h, e := communityHeadIn(ctx, q, id)
	if e != nil {
		return CommunityResource{}, e
	}
	out := CommunityResource{ID: id, Version: h.Version}
	selected := h.Published
	if desk {
		selected = h.Latest
		if h.Published > 0 {
			selected = h.Published
		}
		if h.Pending > 0 {
			selected = h.Pending
		}
	}
	if selected == 0 {
		return out, sql.ErrNoRows
	}
	out.Snapshot, e = communitySnapshotIn(ctx, q, id, selected)
	if e != nil {
		return out, e
	}
	if !desk && out.Snapshot.Action == "WITHDRAW" {
		return CommunityResource{}, sql.ErrNoRows
	}
	if h.Published > 0 {
		pub, err := communitySnapshotIn(ctx, q, id, h.Published)
		if err != nil {
			return out, err
		}
		out.Published = &pub
		if e = q.QueryRowContext(ctx, "SELECT occurred_at FROM community_events WHERE resource_id=? AND proposal_version=? AND action='APPROVED'", id, h.Published).Scan(&out.PublishedAt); e != nil {
			return out, e
		}
	}
	if !desk {
		if e = communityAudience(ctx, q, p, out.Snapshot); e != nil {
			return CommunityResource{}, e
		}
		out.Version = out.Snapshot.Version
		out.Snapshot = communityPublic(out.Snapshot)
		out.Published = nil
		out.State = communityState(out.Snapshot, now)
		return out, nil
	}
	out.State = communityState(out.Snapshot, now)
	if h.Pending > 0 {
		out.State = "PENDING"
		out.CanCancel = out.Snapshot.SubmittedBy == p.ID
		out.CanDecide = !out.CanCancel
	}
	if h.Published == 0 && h.Pending == 0 {
		out.State = h.Decision
	}
	out.CanRevise = (h.Pending == 0 || out.CanCancel) && !(h.Kind == "INTERRUPTION" && out.Published != nil && out.Published.Action == "RESOLVE")
	out.CanWithdraw = h.Pending == 0 && out.Published != nil && out.Published.Action != "WITHDRAW"
	out.CanResolve = out.CanWithdraw && h.Kind == "INTERRUPTION" && out.Published.Action != "RESOLVE" && out.Published.StartAt <= now
	return out, nil
}
func (s *Store) CommunityFor(ctx context.Context, token, kind, query, state string, desk bool, page int) (CommunityPage, error) {
	out := CommunityPage{Items: []CommunityResource{}, PageSize: 12, Counts: map[string]int64{}}
	allowed := map[string]bool{"": true, "PENDING": desk, "DECLINED": desk, "CANCELLED": desk, "WITHDRAWN": desk, "AVAILABLE": true, "ACTIVE": true, "UPDATE_NEEDED": true, "PLANNED": true, "RESOLVED": true}
	if (kind != "" && kind != "CONTACT" && kind != "INTERRUPTION") || !validText(query, 0, 100) || !allowed[state] || !boundedCommunityPage(page) {
		return out, ErrInvalid
	}
	tx, p, e := s.beginCommunityRead(ctx, token, desk)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	querySQL, args := communityQuery(p, desk, kind, query, state, now)
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+querySQL+")", args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.Page = clampMaintenancePage(page, out.Total, 12)
	rows, e := tx.QueryContext(ctx, "SELECT current_state,COUNT(*) FROM ("+querySQL+") GROUP BY current_state", args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var label string
		var n int64
		if e = rows.Scan(&label, &n); e != nil {
			rows.Close()
			return out, e
		}
		out.Counts[label] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	rows, e = tx.QueryContext(ctx, "SELECT id FROM ("+querySQL+") ORDER BY CASE current_state WHEN 'UPDATE_NEEDED' THEN 0 WHEN 'ACTIVE' THEN 1 WHEN 'PENDING' THEN 2 WHEN 'PLANNED' THEN 3 WHEN 'AVAILABLE' THEN 4 ELSE 5 END,id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		x, err := communityResourceIn(ctx, tx, p, id, desk, now)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	return out, tx.Commit()
}
func (s *Store) CommunityResourceFor(ctx context.Context, token, id string, desk bool, page int) (CommunityDetail, error) {
	out := CommunityDetail{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedCommunityPage(page) {
		return out, ErrInvalid
	}
	tx, p, e := s.beginCommunityRead(ctx, token, desk)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.CommunityResource, e = communityResourceIn(ctx, tx, p, id, desk, time.Now().Unix())
	if e != nil {
		return out, e
	}
	if desk {
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM community_events WHERE resource_id=?", id).Scan(&out.EventTotal); e != nil {
			return out, e
		}
		out.EventPage = clampMaintenancePage(page, out.EventTotal, 20)
		rows, err := tx.QueryContext(ctx, `SELECT e.version,e.action,u.display_name,e.reason,e.occurred_at,e.proposal_version FROM community_events e JOIN users u ON u.id=e.actor_id WHERE e.resource_id=? ORDER BY e.version DESC LIMIT 20 OFFSET ?`, id, (out.EventPage-1)*20)
		if err != nil {
			return out, err
		}
		proposals := []int{}
		out.Events = []CommunityEvent{}
		for rows.Next() {
			var event CommunityEvent
			var proposal int
			if e = rows.Scan(&event.Version, &event.Action, &event.Actor, &event.Reason, &event.At, &proposal); e != nil {
				rows.Close()
				return out, e
			}
			out.Events = append(out.Events, event)
			proposals = append(proposals, proposal)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
		for i, version := range proposals {
			out.Events[i].Snapshot, e = communitySnapshotIn(ctx, tx, id, version)
			if e != nil {
				return out, e
			}
		}
	}
	return out, tx.Commit()
}
func (s *Store) CommunityOptionsFor(ctx context.Context, token string) (CommunityOptions, error) {
	out := CommunityOptions{Homes: []RecordHome{}, Buildings: []RecordHome{}}
	tx, _, e := s.beginCommunityRead(ctx, token, true)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.Homes, e = reviewHomes(ctx, tx, Principal{CanReviewRequests: true})
	if e != nil {
		return out, e
	}
	data, e := json.Marshal(out.Homes)
	if e != nil {
		return out, e
	}
	out.AreaKey = TokenHash(string(data))
	rows, e := tx.QueryContext(ctx, "SELECT b.code,'Wing '||b.code FROM buildings b WHERE EXISTS(SELECT 1 FROM flats f WHERE f.building_id=b.id) ORDER BY b.code")
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var b RecordHome
		if e = rows.Scan(&b.ID, &b.Label); e != nil {
			rows.Close()
			return out, e
		}
		out.Buildings = append(out.Buildings, b)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func boundedCommunityPage(page int) bool { return page >= 1 && page <= 10000 }
