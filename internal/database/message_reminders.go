package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

type MessageReminder struct {
	Target             *MessageTarget `json:"target,omitempty"`
	Basis              string         `json:"basis"`
	DueDate            string         `json:"due_date,omitempty"`
	DeadlineAt         int64          `json:"deadline_at,omitempty"`
	PublicationVersion int            `json:"publication_version,omitempty"`
}
type ReminderObligation struct {
	HomeID           string `json:"home_id"`
	Home             string `json:"home"`
	ChargeID         string `json:"charge_id"`
	AmountPaise      int64  `json:"amount_paise"`
	AllocatedPaise   int64  `json:"allocated_paise"`
	OutstandingPaise int64  `json:"outstanding_paise"`
}
type MessageReminderBinding struct {
	Fingerprint      string               `json:"fingerprint"`
	OutstandingPaise int64                `json:"outstanding_paise"`
	Homes            []string             `json:"homes"`
	Obligations      []ReminderObligation `json:"obligations"`
}

func reminderKind(kind string) bool {
	return kind == "MAINTENANCE_REMINDER" || kind == "FUND_REMINDER" || kind == "MEETING_REMINDER"
}
func financeMessageKind(kind string) bool {
	return kind == "RECEIPT" || kind == "STATEMENT" || kind == "MAINTENANCE_REMINDER" || kind == "FUND_REMINDER"
}

// ValidMessageSourceKind is also used by the separately configured provider
// adapter. It validates vocabulary only; each operation rechecks authority.
func ValidMessageSourceKind(kind string) bool {
	return kind == "NOTICE" || kind == "RECEIPT" || kind == "STATEMENT" || reminderKind(kind)
}
func reminderHash(value any) (string, error) {
	blob, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(blob)
	return hex.EncodeToString(sum[:]), nil
}

func sameReminderSource(frozen, current MessageSource) bool {
	before, _ := json.Marshal(frozen)
	after, _ := json.Marshal(current)
	return string(before) == string(after)
}
func reminderSourceIn(ctx context.Context, q identityReader, kind, id string) (MessageSource, error) {
	src := MessageSource{Kind: kind, ID: id, Audience: "FROZEN_HOMES", Reminder: &MessageReminder{Basis: "OUTSTANDING"}}
	var version int
	homes := []string{}
	if kind == "MEETING_REMINDER" {
		h, e := meetingHeadIn(ctx, q, id)
		if e != nil {
			return src, e
		}
		if h.Published == 0 {
			return src, sql.ErrNoRows
		}
		x, e := meetingSnapshotIn(ctx, q, id, h.Published)
		if e != nil {
			return src, e
		}
		if !x.AckRequired || (x.Action != "AGENDA" && x.Action != "MINUTES") {
			return src, sql.ErrNoRows
		}
		for _, home := range x.Homes {
			homes = append(homes, home.ID)
		}
		src.Title = "Meeting acknowledgement · " + x.Title
		e = q.QueryRowContext(ctx, "SELECT publication_fingerprint FROM meeting_events WHERE resource_id=? AND proposal_version=? AND action='APPROVED'", id, x.Version).Scan(&src.Version)
		if e != nil {
			return src, e
		}
		src.Link = "/#community?meeting=" + id
		src.Reminder.PublicationVersion, src.Reminder.DeadlineAt = x.Version, x.AckDeadline
	} else {
		query, lines := "SELECT title,version,due_date FROM maintenance_cycles WHERE id=? AND state='PUBLISHED'", "SELECT flat_id FROM maintenance_lines WHERE cycle_id=? ORDER BY flat_id"
		src.Link = "/#maintenance?cycle=" + id
		if kind == "FUND_REMINDER" {
			query = "SELECT title,version,due_date FROM fund_campaigns WHERE id=? AND state='PUBLISHED' AND contribution_type='FIXED'"
			lines = "SELECT flat_id FROM fund_participants WHERE campaign_id=? ORDER BY flat_id"
			src.Link = "/#collections?campaign=" + id
		}
		if e := q.QueryRowContext(ctx, query, id).Scan(&src.Title, &version, &src.Reminder.DueDate); e != nil {
			return src, e
		}
		src.Title = "Outstanding reminder · " + src.Title
		src.Version = strconv.Itoa(version)
		rows, e := q.QueryContext(ctx, lines, id)
		if e != nil {
			return src, e
		}
		for rows.Next() {
			var home string
			if e = rows.Scan(&home); e != nil {
				rows.Close()
				return src, e
			}
			homes = append(homes, home)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return src, e
		}
	}
	sort.Strings(homes)
	src.PublicationTarget = &MessageTarget{Kind: "HOMES", IDs: homes}
	return src, nil
}
func reminderInput(x MessageBatch) MessageInput {
	in := MessageInput{SourceKind: x.Source.Kind, SourceID: x.Source.ID, Channel: x.Channel, Target: x.Target, PortalOrigin: x.PortalOrigin, Provider: x.Provider}
	if x.Source.Reminder != nil {
		in.ReminderBasis = x.Source.Reminder.Basis
	}
	return in
}
func reminderHomeMatches(home messageMembership, src MessageSource) bool {
	if src.PublicationTarget == nil || (financeMessageKind(src.Kind) && !home.Finance) {
		return false
	}
	if src.Reminder != nil && src.Reminder.Target != nil && src.Reminder.Target.Kind != "PEOPLE" && !messageTargetMatches(messagePerson{Homes: []messageMembership{home}}, *src.Reminder.Target) {
		return false
	}
	return messageTargetMatches(messagePerson{Homes: []messageMembership{home}}, *src.PublicationTarget)
}
func reminderEligibility(ctx context.Context, q identityReader, p messagePerson, src MessageSource) (*MessageReminderBinding, string, error) {
	if src.Reminder == nil {
		return nil, "", ErrInvalid
	}
	if src.Reminder.Basis == "DEADLINE_PASSED" {
		passed := src.Reminder.DueDate != "" && src.Reminder.DueDate < today()
		if src.Kind == "MEETING_REMINDER" {
			passed = src.Reminder.DeadlineAt > 0 && src.Reminder.DeadlineAt < time.Now().Unix()
		}
		if !passed {
			return nil, "DEADLINE_NOT_PASSED", nil
		}
	}
	b := &MessageReminderBinding{Homes: []string{}, Obligations: []ReminderObligation{}}
	seen := map[string]bool{}
	for _, home := range p.Homes {
		if reminderHomeMatches(home, src) && !seen[home.ID] {
			seen[home.ID] = true
			b.Homes = append(b.Homes, home.ID)
		}
	}
	sort.Strings(b.Homes)
	if len(b.Homes) == 0 {
		return nil, "NO_SOURCE_ACCESS", nil
	}
	if src.Kind == "MEETING_REMINDER" {
		var acknowledged bool
		if e := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM meeting_acknowledgements WHERE resource_id=? AND publication_version=? AND resident_id=?)", src.ID, src.Reminder.PublicationVersion, p.ID).Scan(&acknowledged); e != nil {
			return nil, "", e
		}
		if acknowledged {
			return nil, "ACKNOWLEDGED", nil
		}
	} else {
		membership, _ := json.Marshal(b.Homes)
		from, where := "maintenance_lines l JOIN entries e ON e.id=l.entry_id", "l.cycle_id=?"
		if src.Kind == "FUND_REMINDER" {
			from = "fund_participants l JOIN entries e ON e.id=l.current_entry_id"
			where = "l.campaign_id=?"
		}
		rows, e := q.QueryContext(ctx, `SELECT e.flat_id,b.code||'-'||f.flat_number,e.id,e.amount_paise,COALESCE((SELECT SUM(a.amount_paise) FROM live_entry_allocations a WHERE a.charge_id=e.id),0)
 FROM `+from+` JOIN flats f ON f.id=e.flat_id JOIN buildings b ON b.id=f.building_id WHERE `+where+` AND e.state='POSTED' AND e.kind='CHARGE' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id) AND e.flat_id IN(SELECT value FROM json_each(?)) ORDER BY e.flat_id,e.id`, src.ID, string(membership))
		if e != nil {
			return nil, "", e
		}
		for rows.Next() {
			var line ReminderObligation
			if e = rows.Scan(&line.HomeID, &line.Home, &line.ChargeID, &line.AmountPaise, &line.AllocatedPaise); e != nil {
				rows.Close()
				return nil, "", e
			}
			line.OutstandingPaise = line.AmountPaise - line.AllocatedPaise
			if line.OutstandingPaise > 0 {
				b.Obligations = append(b.Obligations, line)
				b.OutstandingPaise += line.OutstandingPaise
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, "", e
		}
		if b.OutstandingPaise == 0 {
			return nil, "NO_OUTSTANDING", nil
		}
	}
	fingerprint, e := reminderHash(struct {
		Source  MessageSource
		Person  string
		Binding *MessageReminderBinding
	}{src, p.ID, b})
	if e != nil {
		return nil, "", e
	}
	b.Fingerprint = fingerprint
	return b, "", nil
}
func storeReminderBinding(ctx context.Context, tx *sql.Tx, x MessageBatch, r MessageRecipient) error {
	if r.Reminder == nil {
		return nil
	}
	blob, e := json.Marshal(r.Reminder)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO message_reminder_recipients(batch_id,snapshot_version,resident_id,source_kind,source_id,basis,fingerprint,outstanding_paise,binding_json) VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, x.SnapshotVersion, r.ID, x.Source.Kind, x.Source.ID, x.Source.Reminder.Basis, r.Reminder.Fingerprint, r.Reminder.OutstandingPaise, string(blob))
	return e
}
func reminderDispatchReason(ctx context.Context, q identityReader, x MessageBatch, p messagePerson) (string, error) {
	current, reason, e := reminderEligibility(ctx, q, p, x.Source)
	if e != nil || reason != "" {
		return reason, e
	}
	var frozen string
	if e = q.QueryRowContext(ctx, "SELECT fingerprint FROM message_reminder_recipients WHERE batch_id=? AND snapshot_version=? AND resident_id=?", x.ID, x.SnapshotVersion, p.ID).Scan(&frozen); e != nil {
		return "", e
	}
	if frozen != current.Fingerprint {
		return "REMINDER_CHANGED", nil
	}
	return "", nil
}

// This opaque read guard is excluded from native output. It binds an in-flight
// read to current eligibility without returning private financial bindings.
func reminderReadKey(ctx context.Context, q identityReader, p Principal, x MessageBatch) (string, error) {
	if messageStaff(p, x.Source.Kind) {
		current, e := resolveMessage(ctx, q, reminderInput(x))
		if e == sql.ErrNoRows {
			return "SOURCE_UNAVAILABLE", nil
		}
		if e != nil {
			return "", e
		}
		return current.PreviewHash, nil
	}
	src, e := messageSourceIn(ctx, q, x.Source.Kind, x.Source.ID)
	if e == sql.ErrNoRows {
		return "SOURCE_UNAVAILABLE", nil
	}
	if e != nil {
		return "", e
	}
	if src.Version != x.Source.Version {
		return "SOURCE_CHANGED", nil
	}
	if x.Source.Reminder != nil {
		src.Reminder.Basis = x.Source.Reminder.Basis
		src.Reminder.Target = x.Source.Reminder.Target
	}
	people, e := messagePeople(ctx, q)
	if e != nil {
		return "", e
	}
	for _, person := range people {
		if person.ID == p.ResidentID {
			reason, _ := messageRecipientReason(person, src, x.Channel, x.Purpose)
			if reason != "" {
				return reason, nil
			}
			binding, reason, e := reminderEligibility(ctx, q, person, src)
			if e != nil {
				return "", e
			}
			if reason != "" {
				return reason, nil
			}
			return binding.Fingerprint, nil
		}
	}
	return "NO_CURRENT_HOME", nil
}
