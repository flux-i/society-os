package database

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

type CommunityInput struct {
	OperationKey string   `json:"operation_key"`
	Version      int      `json:"version"`
	Kind         string   `json:"kind"`
	Action       string   `json:"action"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Service      string   `json:"service"`
	Phone        string   `json:"phone"`
	Availability string   `json:"availability"`
	Attestation  string   `json:"attestation"`
	Scope        string   `json:"scope"`
	BuildingCode string   `json:"building_code"`
	AreaKey      string   `json:"area_key"`
	HomeIDs      []string `json:"home_ids"`
	StartAt      int64    `json:"start_at"`
	EstimatedEnd int64    `json:"estimated_end"`
	ResolvedAt   int64    `json:"resolved_at"`
	UpdateText   string   `json:"update_text"`
	Reason       string   `json:"reason"`
	Confirmed    bool     `json:"confirmed"`
}
type CommunityAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type CommunitySnapshot struct {
	Version      int          `json:"version"`
	Kind         string       `json:"kind"`
	Action       string       `json:"action"`
	Title        string       `json:"title"`
	Body         string       `json:"body"`
	Service      string       `json:"service"`
	Phone        string       `json:"phone,omitempty"`
	Availability string       `json:"availability,omitempty"`
	Attestation  string       `json:"attestation,omitempty"`
	Scope        string       `json:"scope"`
	BuildingCode string       `json:"building_code"`
	Homes        []RecordHome `json:"homes"`
	StartAt      int64        `json:"start_at"`
	EstimatedEnd int64        `json:"estimated_end"`
	ResolvedAt   int64        `json:"resolved_at"`
	UpdateText   string       `json:"update_text,omitempty"`
	SubmittedBy  string       `json:"submitted_by,omitempty"`
	SubmittedAt  int64        `json:"submitted_at,omitempty"`
	Reason       string       `json:"reason,omitempty"`
}
type CommunityResource struct {
	ID          string             `json:"id"`
	Version     int                `json:"version"`
	State       string             `json:"state"`
	PublishedAt int64              `json:"published_at"`
	Snapshot    CommunitySnapshot  `json:"snapshot"`
	Published   *CommunitySnapshot `json:"published,omitempty"`
	CanRevise   bool               `json:"can_revise"`
	CanDecide   bool               `json:"can_decide"`
	CanCancel   bool               `json:"can_cancel"`
	CanResolve  bool               `json:"can_resolve"`
	CanWithdraw bool               `json:"can_withdraw"`
}
type CommunityEvent struct {
	Version  int               `json:"version"`
	Action   string            `json:"action"`
	Actor    string            `json:"actor"`
	Reason   string            `json:"reason"`
	At       int64             `json:"at"`
	Snapshot CommunitySnapshot `json:"snapshot"`
}
type CommunityDetail struct {
	CommunityResource
	Events     []CommunityEvent `json:"events,omitempty"`
	EventTotal int              `json:"event_total,omitempty"`
	EventPage  int              `json:"event_page,omitempty"`
	PageSize   int              `json:"page_size"`
}
type CommunityPage struct {
	Items    []CommunityResource `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Counts   map[string]int64    `json:"counts"`
}
type CommunityOptions struct {
	Homes     []RecordHome `json:"homes"`
	Buildings []RecordHome `json:"buildings"`
	AreaKey   string       `json:"area_key"`
}
type communityHead struct {
	ID, Kind, Decision                  string
	Version, Latest, Pending, Published int
}

func communityHeadIn(ctx context.Context, q identityReader, id string) (communityHead, error) {
	var h communityHead
	e := q.QueryRowContext(ctx, "SELECT id,kind,decision,version,latest_version,COALESCE(pending_version,0),COALESCE(published_version,0) FROM community_resources WHERE id=?", id).Scan(&h.ID, &h.Kind, &h.Decision, &h.Version, &h.Latest, &h.Pending, &h.Published)
	return h, e
}

const communitySnapshotColumns = `v.version,v.kind,v.action,v.title,v.body,v.service,v.phone,v.availability,v.attestation,v.scope,v.building_code,v.homes_json,v.start_at,v.estimated_end,v.resolved_at,v.update_text,v.submitted_by,v.submitted_at,v.reason`

func scanCommunitySnapshot(row interface{ Scan(...any) error }) (CommunitySnapshot, error) {
	var x CommunitySnapshot
	var homes string
	e := row.Scan(&x.Version, &x.Kind, &x.Action, &x.Title, &x.Body, &x.Service, &x.Phone, &x.Availability, &x.Attestation, &x.Scope, &x.BuildingCode, &homes, &x.StartAt, &x.EstimatedEnd, &x.ResolvedAt, &x.UpdateText, &x.SubmittedBy, &x.SubmittedAt, &x.Reason)
	if e == nil {
		e = json.Unmarshal([]byte(homes), &x.Homes)
	}
	return x, e
}
func communitySnapshotIn(ctx context.Context, q identityReader, id string, version int) (CommunitySnapshot, error) {
	return scanCommunitySnapshot(q.QueryRowContext(ctx, "SELECT "+communitySnapshotColumns+" FROM community_versions v WHERE v.resource_id=? AND v.version=?", id, version))
}
func communityState(x CommunitySnapshot, now int64) string {
	if x.Action == "WITHDRAW" {
		return "WITHDRAWN"
	}
	if x.Kind == "CONTACT" {
		return "AVAILABLE"
	}
	if x.Action == "RESOLVE" {
		return "RESOLVED"
	}
	if x.StartAt > now {
		return "PLANNED"
	}
	if x.EstimatedEnd > 0 && x.EstimatedEnd <= now {
		return "UPDATE_NEEDED"
	}
	return "ACTIVE"
}
func communityPublic(x CommunitySnapshot) CommunitySnapshot {
	x.Attestation = ""
	x.SubmittedBy = ""
	x.SubmittedAt = 0
	x.Reason = ""
	return x
}
func communityReadPrincipal(ctx context.Context, q identityReader, p Principal) error {
	if p.CanReviewRequests {
		return nil
	}
	var current bool
	e := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))", p.ResidentID, today(), today()).Scan(&current)
	if e != nil {
		return e
	}
	if !current {
		return ErrForbidden
	}
	return nil
}
func communityAudience(ctx context.Context, q identityReader, p Principal, x CommunitySnapshot) error {
	if p.CanReviewRequests {
		return nil
	}
	data, e := json.Marshal(x.Homes)
	if e != nil {
		return e
	}
	var permitted bool
	e = q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM json_each(?) h JOIN flat_memberships m ON m.flat_id=json_extract(h.value,'$.id') WHERE m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, string(data), p.ResidentID, today(), today()).Scan(&permitted)
	if e != nil {
		return e
	}
	if !permitted {
		return ErrForbidden
	}
	return nil
}
func normaliseCommunity(in CommunityInput) (CommunityInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	in.Availability = strings.TrimSpace(in.Availability)
	in.Attestation = strings.TrimSpace(in.Attestation)
	in.Reason = strings.TrimSpace(in.Reason)
	in.UpdateText = strings.TrimSpace(in.UpdateText)
	if !in.Confirmed || !paragraph(in.Reason, 10, 800) || in.Version < 0 || in.Version > 1000000 {
		return in, invalid("Confirm the exact proposal with a reason of 10–800 characters.")
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
	if in.Action == "RESOLVE" || in.Action == "WITHDRAW" {
		if in.Kind != "" || in.Title != "" || in.Body != "" || in.Service != "" || in.Phone != "" || in.Availability != "" || in.Attestation != "" || in.Scope != "" || in.BuildingCode != "" || in.AreaKey != "" || len(in.HomeIDs) != 0 || in.StartAt != 0 || in.EstimatedEnd != 0 {
			return in, invalid("A resolution or withdrawal must retain the approved original and area.")
		}
		if in.Action == "WITHDRAW" && (in.ResolvedAt != 0 || in.UpdateText != "") {
			return in, ErrInvalid
		}
		if in.Action == "RESOLVE" && (!paragraph(in.UpdateText, 10, 2000) || in.ResolvedAt < 946684800 || in.ResolvedAt > 4102444799) {
			return in, invalid("Supply the restoration time and a public update of 10–2000 characters.")
		}
		return in, nil
	}
	if in.Action != "PUBLISH" || (in.Kind != "CONTACT" && in.Kind != "INTERRUPTION") || !validText(in.Title, 5, 120) || !paragraph(in.Body, 10, 2000) || (in.Service != "WATER" && in.Service != "POWER" && in.Service != "LIFT" && in.Service != "OTHER") || in.ResolvedAt != 0 || in.UpdateText != "" {
		return in, invalid("Choose a supported service and a title of 5–120 characters with a description of 10–2000 characters.")
	}
	if len(in.AreaKey) != 64 || strings.Trim(in.AreaKey, "0123456789abcdef") != "" {
		return in, invalid("Reload and review the current area options.")
	}
	if (in.Scope != "ALL" && in.Scope != "WING" && in.Scope != "HOMES") || (in.Scope == "WING" && !validText(in.BuildingCode, 1, 20)) || (in.Scope != "WING" && in.BuildingCode != "") || (in.Scope == "HOMES" && len(in.HomeIDs) == 0) || (in.Scope != "HOMES" && len(in.HomeIDs) > 0) {
		return in, invalid("Select all homes, one current wing or specific current homes.")
	}
	if in.Kind == "CONTACT" {
		if len(in.Phone) > 40 {
			return in, ErrInvalid
		}
		in.Phone = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(in.Phone))
		if !internationalPhone.MatchString(in.Phone) || !validText(in.Availability, 3, 200) || !paragraph(in.Attestation, 10, 800) || in.StartAt != 0 || in.EstimatedEnd != 0 {
			return in, invalid("Supply an international phone number, availability and the contact's permission/authority reference.")
		}
	} else if in.Phone != "" || in.Availability != "" || in.Attestation != "" || in.StartAt < 946684800 || in.StartAt > 4102444799 || (in.EstimatedEnd != 0 && (in.EstimatedEnd < in.StartAt || in.EstimatedEnd > 4102444799)) {
		return in, invalid("Supply valid start and estimated end times; an estimate cannot precede the start.")
	}
	return in, nil
}
