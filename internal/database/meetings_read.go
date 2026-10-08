package database

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const meetingStateSQL = `CASE WHEN v.action='WITHDRAW' THEN 'WITHDRAWN' WHEN v.action='CANCEL' THEN 'CANCELLED' WHEN v.action='MINUTES' THEN 'MINUTES' WHEN v.start_at>:now THEN 'UPCOMING' ELSE 'PAST' END`

func meetingQuery(p Principal, desk bool, query, state string, now int64) (string, []any) {
	selected := "h.published_version"
	where := "h.published_version IS NOT NULL AND v.action<>'WITHDRAW'"
	stateSQL := meetingStateSQL
	args := []any{sql.Named("now", now)}
	if desk {
		selected = "COALESCE(h.pending_version,h.published_version,h.latest_version)"
		where = "1=1"
		stateSQL = "CASE WHEN h.pending_version IS NOT NULL THEN 'PENDING' WHEN h.published_version IS NULL AND h.decision='CANCELLED' THEN 'PROPOSAL_CANCELLED' WHEN h.published_version IS NULL THEN h.decision ELSE " + meetingStateSQL + " END"
	} else {
		scope, scoped := communityVisibleScope(p)
		where += " AND " + scope
		args = append(args, scoped...)
	}
	if query != "" {
		where += " AND (instr(lower(v.title),?)>0 OR instr(lower(v.body),?)>0 OR instr(lower(v.minutes),?)>0 OR instr(lower(v.update_text),?)>0)"
		term := strings.ToLower(strings.TrimSpace(query))
		args = append(args, term, term, term, term)
	}
	result := `SELECT h.id,` + stateSQL + ` AS current_state FROM meeting_resources h JOIN meeting_versions v ON v.resource_id=h.id AND v.version=` + selected + ` WHERE ` + where
	if state != "" {
		result = "SELECT * FROM (" + result + ") WHERE current_state=?"
		args = append(args, state)
	}
	return result, args
}
func meetingResourceIn(ctx context.Context, q identityReader, p Principal, id string, desk bool, now int64) (MeetingResource, error) {
	h, e := meetingHeadIn(ctx, q, id)
	if e != nil {
		return MeetingResource{}, e
	}
	out := MeetingResource{ID: id, Version: h.Version}
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
	out.Snapshot, e = meetingSnapshotIn(ctx, q, id, selected)
	if e != nil {
		return out, e
	}
	if !desk && out.Snapshot.Action == "WITHDRAW" {
		return MeetingResource{}, sql.ErrNoRows
	}
	if h.Published > 0 {
		pub, err := meetingSnapshotIn(ctx, q, id, h.Published)
		if err != nil {
			return out, err
		}
		out.Published = &pub
		if e = q.QueryRowContext(ctx, "SELECT occurred_at,publication_fingerprint FROM meeting_events WHERE resource_id=? AND proposal_version=? AND action='APPROVED'", id, h.Published).Scan(&out.PublishedAt, &out.Acknowledgement.Fingerprint); e != nil {
			return out, e
		}
		out.Acknowledgement.Required = pub.AckRequired
		out.Acknowledgement.Deadline = pub.AckDeadline
		e = q.QueryRowContext(ctx, "SELECT occurred_at FROM meeting_acknowledgements WHERE resource_id=? AND publication_version=? AND resident_id=?", id, h.Published, p.ResidentID).Scan(&out.Acknowledgement.At)
		if e != nil && e != sql.ErrNoRows {
			return out, e
		}
		out.Acknowledgement.Acknowledged = e == nil
		personal, err := meetingPersonalHomes(ctx, q, p, pub)
		if err != nil {
			return out, err
		}
		out.Acknowledgement.CanAcknowledge = len(personal) > 0 && pub.AckRequired && (pub.Action == "AGENDA" || pub.Action == "MINUTES") && !out.Acknowledgement.Acknowledged
	}
	if !desk {
		if e = meetingAudience(ctx, q, p, out.Snapshot); e != nil {
			return MeetingResource{}, e
		}
		out.Version = out.Snapshot.Version
		out.Snapshot = meetingPublic(out.Snapshot)
		out.Published = nil
		out.State = meetingState(out.Snapshot, now)
		return out, nil
	}
	out.State = meetingState(out.Snapshot, now)
	if h.Pending > 0 {
		out.State = "PENDING"
		out.CanCancel = out.Snapshot.SubmittedBy == p.ID
		out.CanDecide = !out.CanCancel
	}
	if h.Published == 0 && h.Pending == 0 {
		out.State = h.Decision
		if out.State == "CANCELLED" {
			out.State = "PROPOSAL_CANCELLED"
		}
	}
	out.CanRevise = (h.Pending == 0 || out.CanCancel) && (out.Published == nil || out.Published.Action == "AGENDA" || out.Published.Action == "MINUTES")
	out.CanWithdraw = h.Pending == 0 && out.Published != nil && out.Published.Action != "WITHDRAW"
	out.CanPrepareMinutes = out.CanWithdraw && (out.Published.Action == "AGENDA" || out.Published.Action == "MINUTES") && out.Published.StartAt <= now
	out.CanPrepareCancellation = out.CanWithdraw && out.Published.Action == "AGENDA"
	return out, nil
}
func (s *Store) MeetingsFor(ctx context.Context, token, query, state string, desk bool, page int) (MeetingPage, error) {
	out := MeetingPage{Items: []MeetingResource{}, PageSize: 12, Counts: map[string]int64{}}
	allowed := map[string]bool{"": true, "UPCOMING": true, "PAST": true, "MINUTES": true, "CANCELLED": true, "PENDING": desk, "DECLINED": desk, "PROPOSAL_CANCELLED": desk, "WITHDRAWN": desk}
	if !validText(query, 0, 100) || !allowed[state] || !boundedCommunityPage(page) {
		return out, ErrInvalid
	}
	tx, p, e := s.beginCommunityRead(ctx, token, desk)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	querySQL, args := meetingQuery(p, desk, query, state, now)
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
	rows, e = tx.QueryContext(ctx, "SELECT id FROM ("+querySQL+") ORDER BY CASE current_state WHEN 'PENDING' THEN 0 WHEN 'UPCOMING' THEN 1 ELSE 2 END,id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
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
		x, err := meetingResourceIn(ctx, tx, p, id, desk, now)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	return out, tx.Commit()
}
func (s *Store) MeetingFor(ctx context.Context, token, id string, desk bool, page int) (MeetingDetail, error) {
	out := MeetingDetail{PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedCommunityPage(page) {
		return out, ErrInvalid
	}
	tx, p, e := s.beginCommunityRead(ctx, token, desk)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	out.MeetingResource, e = meetingResourceIn(ctx, tx, p, id, desk, time.Now().Unix())
	if e != nil {
		return out, e
	}
	if desk {
		if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM meeting_events WHERE resource_id=?", id).Scan(&out.EventTotal); e != nil {
			return out, e
		}
		out.EventPage = clampMaintenancePage(page, out.EventTotal, 20)
		rows, err := tx.QueryContext(ctx, `SELECT e.version,e.action,u.display_name,e.reason,e.occurred_at,e.proposal_version FROM meeting_events e JOIN users u ON u.id=e.actor_id WHERE e.resource_id=? ORDER BY e.version DESC LIMIT 20 OFFSET ?`, id, (out.EventPage-1)*20)
		if err != nil {
			return out, err
		}
		proposals := []int{}
		out.Events = []MeetingEvent{}
		for rows.Next() {
			var event MeetingEvent
			var version int
			if e = rows.Scan(&event.Version, &event.Action, &event.Actor, &event.Reason, &event.At, &version); e != nil {
				rows.Close()
				return out, e
			}
			out.Events = append(out.Events, event)
			proposals = append(proposals, version)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
		for i, version := range proposals {
			out.Events[i].Snapshot, e = meetingSnapshotIn(ctx, tx, id, version)
			if e != nil {
				return out, e
			}
		}
	}
	return out, tx.Commit()
}
