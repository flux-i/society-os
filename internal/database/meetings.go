package database

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

type MeetingInput struct {
	OperationKey string   `json:"operation_key"`
	Version      int      `json:"version"`
	Action       string   `json:"action"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Location     string   `json:"location"`
	Scope        string   `json:"scope"`
	BuildingCode string   `json:"building_code"`
	AreaKey      string   `json:"area_key"`
	HomeIDs      []string `json:"home_ids"`
	StartAt      int64    `json:"start_at"`
	EndAt        int64    `json:"end_at"`
	HeldAt       int64    `json:"held_at"`
	Minutes      string   `json:"minutes"`
	UpdateText   string   `json:"update_text"`
	AckRequired  bool     `json:"ack_required"`
	AckDeadline  int64    `json:"ack_deadline"`
	Reason       string   `json:"reason"`
	Confirmed    bool     `json:"confirmed"`
}
type MeetingAction = CommunityAction
type MeetingAcknowledgementInput struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Fingerprint  string `json:"fingerprint"`
	Confirmed    bool   `json:"confirmed"`
}
type MeetingSnapshot struct {
	Version      int          `json:"version"`
	Action       string       `json:"action"`
	Title        string       `json:"title"`
	Body         string       `json:"body"`
	Location     string       `json:"location"`
	Scope        string       `json:"scope"`
	BuildingCode string       `json:"building_code"`
	Homes        []RecordHome `json:"homes"`
	StartAt      int64        `json:"start_at"`
	EndAt        int64        `json:"end_at"`
	HeldAt       int64        `json:"held_at"`
	Minutes      string       `json:"minutes,omitempty"`
	UpdateText   string       `json:"update_text,omitempty"`
	AckRequired  bool         `json:"ack_required"`
	AckDeadline  int64        `json:"ack_deadline"`
	SubmittedBy  string       `json:"submitted_by,omitempty"`
	SubmittedAt  int64        `json:"submitted_at,omitempty"`
	Reason       string       `json:"reason,omitempty"`
}
type MeetingAcknowledgement struct {
	Required       bool   `json:"required"`
	Deadline       int64  `json:"deadline"`
	Fingerprint    string `json:"fingerprint"`
	Acknowledged   bool   `json:"acknowledged"`
	At             int64  `json:"at"`
	CanAcknowledge bool   `json:"can_acknowledge"`
}
type MeetingResource struct {
	ID                     string                 `json:"id"`
	Version                int                    `json:"version"`
	State                  string                 `json:"state"`
	PublishedAt            int64                  `json:"published_at"`
	Snapshot               MeetingSnapshot        `json:"snapshot"`
	Published              *MeetingSnapshot       `json:"published,omitempty"`
	Acknowledgement        MeetingAcknowledgement `json:"acknowledgement"`
	CanRevise              bool                   `json:"can_revise"`
	CanDecide              bool                   `json:"can_decide"`
	CanCancel              bool                   `json:"can_cancel"`
	CanPrepareMinutes      bool                   `json:"can_prepare_minutes"`
	CanPrepareCancellation bool                   `json:"can_prepare_cancellation"`
	CanWithdraw            bool                   `json:"can_withdraw"`
}
type MeetingEvent struct {
	Version  int             `json:"version"`
	Action   string          `json:"action"`
	Actor    string          `json:"actor"`
	Reason   string          `json:"reason"`
	At       int64           `json:"at"`
	Snapshot MeetingSnapshot `json:"snapshot"`
}
type MeetingDetail struct {
	MeetingResource
	Events     []MeetingEvent `json:"events,omitempty"`
	EventTotal int            `json:"event_total,omitempty"`
	EventPage  int            `json:"event_page,omitempty"`
	PageSize   int            `json:"page_size"`
}
type MeetingPage struct {
	Items    []MeetingResource `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Counts   map[string]int64  `json:"counts"`
}
type MeetingResponse struct {
	ResidentID   string       `json:"resident_id"`
	Name         string       `json:"name"`
	Acknowledged bool         `json:"acknowledged"`
	At           int64        `json:"at"`
	Homes        []RecordHome `json:"homes"`
}
type MeetingRetainedResponse struct {
	Version    int          `json:"version"`
	ResidentID string       `json:"resident_id"`
	Name       string       `json:"name"`
	At         int64        `json:"at"`
	Homes      []RecordHome `json:"homes"`
}
type MeetingResponses struct {
	PublicationVersion int                       `json:"publication_version"`
	Expected           int                       `json:"expected"`
	Acknowledged       int                       `json:"acknowledged"`
	Outstanding        int                       `json:"outstanding"`
	HistoricalTotal    int                       `json:"historical_total"`
	Items              []MeetingResponse         `json:"items"`
	History            []MeetingRetainedResponse `json:"history"`
	Page               int                       `json:"page"`
	HistoryPage        int                       `json:"history_page"`
	PageSize           int                       `json:"page_size"`
}
type meetingHead struct {
	ID, Decision                        string
	Version, Latest, Pending, Published int
}

func meetingHeadIn(ctx context.Context, q identityReader, id string) (meetingHead, error) {
	var h meetingHead
	e := q.QueryRowContext(ctx, "SELECT id,decision,version,latest_version,COALESCE(pending_version,0),COALESCE(published_version,0) FROM meeting_resources WHERE id=?", id).Scan(&h.ID, &h.Decision, &h.Version, &h.Latest, &h.Pending, &h.Published)
	return h, e
}

const meetingSnapshotColumns = `v.version,v.action,v.title,v.body,v.location,v.scope,v.building_code,v.homes_json,v.start_at,v.end_at,v.held_at,v.minutes,v.update_text,v.ack_required,v.ack_deadline,v.submitted_by,v.submitted_at,v.reason`

func scanMeetingSnapshot(row interface{ Scan(...any) error }) (MeetingSnapshot, error) {
	var x MeetingSnapshot
	var homes string
	e := row.Scan(&x.Version, &x.Action, &x.Title, &x.Body, &x.Location, &x.Scope, &x.BuildingCode, &homes, &x.StartAt, &x.EndAt, &x.HeldAt, &x.Minutes, &x.UpdateText, &x.AckRequired, &x.AckDeadline, &x.SubmittedBy, &x.SubmittedAt, &x.Reason)
	if e == nil {
		e = json.Unmarshal([]byte(homes), &x.Homes)
	}
	return x, e
}
func meetingSnapshotIn(ctx context.Context, q identityReader, id string, version int) (MeetingSnapshot, error) {
	return scanMeetingSnapshot(q.QueryRowContext(ctx, "SELECT "+meetingSnapshotColumns+" FROM meeting_versions v WHERE v.resource_id=? AND v.version=?", id, version))
}
func meetingPublic(x MeetingSnapshot) MeetingSnapshot {
	x.SubmittedBy = ""
	x.SubmittedAt = 0
	x.Reason = ""
	return x
}
func meetingState(x MeetingSnapshot, now int64) string {
	switch x.Action {
	case "WITHDRAW":
		return "WITHDRAWN"
	case "CANCEL":
		return "CANCELLED"
	case "MINUTES":
		return "MINUTES"
	}
	if x.StartAt > now {
		return "UPCOMING"
	}
	return "PAST"
}
func supportedMeetingInstant(at int64) bool { return at >= 946684800 && at <= 4102444799 }
func normaliseMeeting(in MeetingInput) (MeetingInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	in.Location = strings.TrimSpace(in.Location)
	in.Minutes = strings.TrimSpace(in.Minutes)
	in.UpdateText = strings.TrimSpace(in.UpdateText)
	in.Reason = strings.TrimSpace(in.Reason)
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 0 || in.Version > 1000000 {
		return in, invalid("Confirm the exact proposal with a reason of 10–800 characters.")
	}
	if (!in.AckRequired && in.AckDeadline != 0) || (in.AckDeadline != 0 && !supportedMeetingInstant(in.AckDeadline)) {
		return in, invalid("Supply an acknowledgement deadline only when acknowledgement is requested.")
	}
	in.HomeIDs = append([]string{}, in.HomeIDs...)
	if len(in.HomeIDs) > 118 {
		return in, ErrInvalid
	}
	sort.Strings(in.HomeIDs)
	for i, id := range in.HomeIDs {
		if !validText(id, 1, 100) || (i > 0 && id == in.HomeIDs[i-1]) {
			return in, invalid("Choose each home once from the current area options.")
		}
	}
	if in.Action != "AGENDA" {
		if in.Action != "MINUTES" && in.Action != "CANCEL" && in.Action != "WITHDRAW" {
			return in, ErrInvalid
		}
		if in.Title != "" || in.Body != "" || in.Location != "" || in.Scope != "" || in.BuildingCode != "" || in.AreaKey != "" || len(in.HomeIDs) > 0 || in.StartAt != 0 || in.EndAt != 0 {
			return in, invalid("Minutes, cancellation and withdrawal retain the approved original agenda and area.")
		}
		if in.Action == "MINUTES" {
			if !paragraph(in.Minutes, 10, 8000) || !supportedMeetingInstant(in.HeldAt) || in.UpdateText != "" {
				return in, invalid("Supply a meeting-held time and minutes of 10–8000 characters.")
			}
		} else {
			if in.HeldAt != 0 || in.Minutes != "" || in.AckRequired || in.AckDeadline != 0 {
				return in, ErrInvalid
			}
			if in.Action == "CANCEL" && !paragraph(in.UpdateText, 10, 2000) {
				return in, invalid("Supply a public cancellation update of 10–2000 characters.")
			}
			if in.Action == "WITHDRAW" && in.UpdateText != "" {
				return in, ErrInvalid
			}
		}
		return in, nil
	}
	if !validText(in.Title, 5, 120) || !paragraph(in.Body, 10, 4000) || !validText(in.Location, 3, 200) || !supportedMeetingInstant(in.StartAt) || (in.EndAt != 0 && (!supportedMeetingInstant(in.EndAt) || in.EndAt < in.StartAt)) || in.HeldAt != 0 || in.Minutes != "" || in.UpdateText != "" {
		return in, invalid("Supply a title, agenda, location and valid start/end times.")
	}
	if len(in.AreaKey) != 64 || strings.Trim(in.AreaKey, "0123456789abcdef") != "" {
		return in, invalid("Reload and review the current area options.")
	}
	if (in.Scope != "ALL" && in.Scope != "WING" && in.Scope != "HOMES") || (in.Scope == "WING" && !validText(in.BuildingCode, 1, 20)) || (in.Scope != "WING" && in.BuildingCode != "") || (in.Scope == "HOMES" && len(in.HomeIDs) == 0) || (in.Scope != "HOMES" && len(in.HomeIDs) > 0) {
		return in, invalid("Select all homes, one current wing or specific current homes.")
	}
	return in, nil
}

// Acknowledgements use the actor's real household intersection even when their
// operational appointment permits a broader published read.
func meetingPersonalHomes(ctx context.Context, q identityReader, p Principal, x MeetingSnapshot) ([]RecordHome, error) {
	current, e := reviewHomes(ctx, q, Principal{ResidentID: p.ResidentID})
	if e != nil {
		return nil, e
	}
	approved := map[string]bool{}
	for _, h := range x.Homes {
		approved[h.ID] = true
	}
	homes := []RecordHome{}
	for _, h := range current {
		if approved[h.ID] {
			homes = append(homes, h)
		}
	}
	return homes, nil
}
func meetingAudience(ctx context.Context, q identityReader, p Principal, x MeetingSnapshot) error {
	if p.CanReviewRequests {
		return nil
	}
	homes, e := meetingPersonalHomes(ctx, q, p, x)
	if e != nil {
		return e
	}
	if len(homes) == 0 {
		return ErrForbidden
	}
	return nil
}
