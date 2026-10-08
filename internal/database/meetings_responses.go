package database

import (
	"context"
	"database/sql"
	"encoding/json"
)

const meetingExpectedPeople = `SELECT DISTINCT r.id,r.full_name FROM residents r WHERE EXISTS(SELECT 1 FROM users u WHERE u.resident_id=r.id AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL) AND EXISTS(SELECT 1 FROM json_each(?) h JOIN flat_memberships m ON m.flat_id=json_extract(h.value,'$.id') WHERE m.resident_id=r.id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`

func (s *Store) MeetingResponsesFor(ctx context.Context, token, id string, page, historyPage int) (MeetingResponses, error) {
	out := MeetingResponses{Items: []MeetingResponse{}, History: []MeetingRetainedResponse{}, PageSize: 20}
	if len(id) < 1 || len(id) > 100 || !boundedCommunityPage(page) || !boundedCommunityPage(historyPage) {
		return out, ErrInvalid
	}
	tx, _, e := s.beginCommunityRead(ctx, token, true)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	h, e := meetingHeadIn(ctx, tx, id)
	if e != nil {
		return out, e
	}
	out.PublicationVersion = h.Published
	if h.Published > 0 {
		x, err := meetingSnapshotIn(ctx, tx, id, h.Published)
		if err != nil {
			return out, err
		}
		if x.AckRequired && (x.Action == "AGENDA" || x.Action == "MINUTES") {
			homes, err := json.Marshal(x.Homes)
			if err != nil {
				return out, err
			}
			args := []any{string(homes), today(), today()}
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+meetingExpectedPeople+")", args...).Scan(&out.Expected); e != nil {
				return out, e
			}
			if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+meetingExpectedPeople+") p JOIN meeting_acknowledgements a ON a.resident_id=p.id AND a.resource_id=? AND a.publication_version=?", append(args, id, h.Published)...).Scan(&out.Acknowledged); e != nil {
				return out, e
			}
			out.Outstanding = out.Expected - out.Acknowledged
			out.Page = clampMaintenancePage(page, out.Expected, 20)
			rows, err := tx.QueryContext(ctx, "SELECT id,full_name FROM ("+meetingExpectedPeople+") ORDER BY full_name,id LIMIT 20 OFFSET ?", append(args, (out.Page-1)*20)...)
			if err != nil {
				return out, err
			}
			for rows.Next() {
				var item MeetingResponse
				if e = rows.Scan(&item.ResidentID, &item.Name); e != nil {
					rows.Close()
					return out, e
				}
				out.Items = append(out.Items, item)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, e
			}
			for i := range out.Items {
				item := &out.Items[i]
				item.Homes, e = meetingPersonalHomes(ctx, tx, Principal{ResidentID: item.ResidentID}, x)
				if e != nil {
					return out, e
				}
				e = tx.QueryRowContext(ctx, "SELECT occurred_at FROM meeting_acknowledgements WHERE resource_id=? AND publication_version=? AND resident_id=?", id, h.Published, item.ResidentID).Scan(&item.At)
				if e != nil && e != sql.ErrNoRows {
					return out, e
				}
				item.Acknowledged = e == nil
			}
		}
	}
	if out.Page == 0 {
		out.Page = 1
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM meeting_acknowledgements WHERE resource_id=?", id).Scan(&out.HistoricalTotal); e != nil {
		return out, e
	}
	out.HistoryPage = clampMaintenancePage(historyPage, out.HistoricalTotal, 20)
	rows, e := tx.QueryContext(ctx, `SELECT a.publication_version,a.resident_id,r.full_name,a.occurred_at,a.homes_json FROM meeting_acknowledgements a JOIN residents r ON r.id=a.resident_id WHERE a.resource_id=? ORDER BY a.occurred_at DESC,a.publication_version DESC,a.id LIMIT 20 OFFSET ?`, id, (out.HistoryPage-1)*20)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var item MeetingRetainedResponse
		var homes string
		if e = rows.Scan(&item.Version, &item.ResidentID, &item.Name, &item.At, &homes); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(homes), &item.Homes); e != nil {
			rows.Close()
			return out, e
		}
		out.History = append(out.History, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
