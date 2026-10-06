package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MessageTarget struct {
	Kind string   `json:"kind"`
	Wing string   `json:"wing"`
	IDs  []string `json:"ids"`
}
type MessageInput struct {
	OperationKey string        `json:"operation_key"`
	SourceKind   string        `json:"source_kind"`
	SourceID     string        `json:"source_id"`
	Channel      string        `json:"channel"`
	Target       MessageTarget `json:"target"`
	PreviewHash  string        `json:"preview_hash"`
	Reason       string        `json:"reason"`
	Confirmed    bool          `json:"confirmed"`
	PortalOrigin string        `json:"-"`
}
type MessageAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
	Outcome      string `json:"outcome"`
}
type MessageSource struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Version  string `json:"version"`
	Title    string `json:"title"`
	Audience string `json:"audience"`
	Wing     string `json:"wing"`
	HomeID   string `json:"home_id"`
	EntryID  string `json:"entry_id"`
	Link     string `json:"link"`
}
type MessageCounts struct {
	TargetPeople    int            `json:"target_people"`
	SourcePeople    int            `json:"source_people"`
	ConsentedPeople int            `json:"consented_people"`
	EligiblePeople  int            `json:"eligible_people"`
	Destinations    int            `json:"destinations"`
	OmittedPeople   int            `json:"omitted_people"`
	Reasons         map[string]int `json:"reasons"`
}
type MessageRecipient struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ContactVersion int    `json:"contact_version"`
	Destination    string `json:"destination,omitempty"`
	Reason         string `json:"reason"`
	DeliveryID     string `json:"delivery_id,omitempty"`
	State          string `json:"state,omitempty"`
}
type MessagePreview struct {
	Source      MessageSource      `json:"source"`
	Channel     string             `json:"channel"`
	Purpose     string             `json:"purpose"`
	Target      MessageTarget      `json:"target"`
	Counts      MessageCounts      `json:"counts"`
	Envelope    string             `json:"envelope"`
	PreviewHash string             `json:"preview_hash"`
	Recipients  []MessageRecipient `json:"recipients"`
	Page        int                `json:"page"`
	PageSize    int                `json:"page_size"`
	Simulation  bool               `json:"simulation"`
}
type MessageBatch struct {
	ID                  string         `json:"id"`
	Source              MessageSource  `json:"source"`
	Target              MessageTarget  `json:"target"`
	Counts              MessageCounts  `json:"counts"`
	PreviewHash         string         `json:"preview_hash"`
	Channel             string         `json:"channel"`
	Purpose             string         `json:"purpose"`
	PortalOrigin        string         `json:"portal_origin"`
	Envelope            string         `json:"envelope"`
	State               string         `json:"state"`
	ProposedBy          string         `json:"proposed_by"`
	ProposedAt          int64          `json:"proposed_at"`
	ReviewedBy          string         `json:"reviewed_by"`
	ReviewedAt          int64          `json:"reviewed_at"`
	UpdatedAt           int64          `json:"updated_at"`
	SnapshotVersion     int            `json:"snapshot_version"`
	Version             int            `json:"version"`
	Simulation          bool           `json:"simulation"`
	CanApprove          bool           `json:"can_approve"`
	CanRefresh          bool           `json:"can_refresh"`
	CanWithdraw         bool           `json:"can_withdraw"`
	CanDispatch         bool           `json:"can_dispatch"`
	CanCancel           bool           `json:"can_cancel"`
	ReviewProblem       string         `json:"review_problem"`
	Outcomes            map[string]int `json:"outcomes"`
	RetryableDeliveries int            `json:"retryable_deliveries"`
}
type MessageDelivery struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	State       string `json:"state"`
	Attempts    int    `json:"attempts"`
	ProviderID  string `json:"provider_id"`
	Reason      string `json:"reason"`
	AcceptedAt  int64  `json:"accepted_at"`
	DeliveredAt int64  `json:"delivered_at"`
	ReadAt      int64  `json:"read_at"`
	UpdatedAt   int64  `json:"updated_at"`
}
type MessageEvent struct {
	Version  int          `json:"version"`
	Action   string       `json:"action"`
	Actor    string       `json:"actor"`
	Reason   string       `json:"reason"`
	At       int64        `json:"at"`
	Snapshot MessageBatch `json:"snapshot"`
}
type MessageDetail struct {
	MessageBatch
	Recipients     []MessageRecipient `json:"recipients"`
	RecipientPage  int                `json:"recipient_page"`
	RecipientTotal int                `json:"recipient_total"`
	Deliveries     []MessageDelivery  `json:"deliveries"`
	DeliveryPage   int                `json:"delivery_page"`
	DeliveryTotal  int                `json:"delivery_total"`
	Events         []MessageEvent     `json:"events"`
	EventPage      int                `json:"event_page"`
	EventTotal     int                `json:"event_total"`
	PageSize       int                `json:"page_size"`
	Staff          bool               `json:"staff"`
}
type MessagePage struct {
	Items    []MessageBatch `json:"items"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}
type MessageSourcePage struct {
	Items    []MessageSource `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}
type messageMembership struct {
	ID, Wing, Relationship string
	Finance                bool
}
type messagePerson struct {
	ID, Name      string
	ActiveAccount bool
	Contact       Contact
	Homes         []messageMembership
}
type messageResolution struct {
	MessagePreview
	People []MessageRecipient
}

func messageStaff(p Principal, kind string) bool {
	return !p.MFAPending && ((kind == "NOTICE" && p.CanReviewRequests) || (kind == "RECEIPT" && p.CanManageRecords))
}
func messageAuthority(p Principal, kind string, fresh bool) error {
	if !messageStaff(p, kind) {
		return ErrForbidden
	}
	if fresh && !p.Fresh {
		return ErrReauthRequired
	}
	return nil
}
func validateMessageInput(in MessageInput) (MessageInput, error) {
	if (in.SourceKind != "NOTICE" && in.SourceKind != "RECEIPT") || in.SourceID == "" || len(in.SourceID) > 100 || (in.Channel != "WHATSAPP" && in.Channel != "EMAIL") {
		return in, ErrInvalid
	}
	u, e := url.Parse(in.PortalOrigin)
	if e != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Host == "" {
		return in, ErrInvalid
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return in, invalid("Delivery simulation uses the current local portal origin.")
	}
	if len(in.Target.IDs) > 200 {
		return in, invalid("Choose at most 200 explicit people or homes.")
	}
	switch in.Target.Kind {
	case "ALL", "OWNERS", "TENANTS":
		if in.Target.Wing != "" || len(in.Target.IDs) != 0 {
			return in, ErrInvalid
		}
	case "WING":
		if (in.Target.Wing != "A" && in.Target.Wing != "B" && in.Target.Wing != "C") || len(in.Target.IDs) != 0 {
			return in, ErrInvalid
		}
	case "HOMES", "PEOPLE":
		if in.Target.Wing != "" || len(in.Target.IDs) == 0 {
			return in, ErrInvalid
		}
	default:
		return in, ErrInvalid
	}
	in.Target.IDs = append([]string{}, in.Target.IDs...)
	sort.Strings(in.Target.IDs)
	for i, id := range in.Target.IDs {
		if id == "" || len(id) > 100 || (i > 0 && id == in.Target.IDs[i-1]) {
			return in, ErrInvalid
		}
	}
	return in, nil
}
func messageSourceIn(ctx context.Context, q identityReader, kind, id string) (MessageSource, error) {
	src := MessageSource{Kind: kind, ID: id}
	switch kind {
	case "NOTICE":
		x, e := scanReview(q.QueryRowContext(ctx, reviewSelect+" WHERE x.id=? AND x.kind='NOTICE' AND x.state='APPROVED' AND x.audience<>'COMMITTEE_ONLY'", id))
		if e != nil {
			return src, e
		}
		src.Version, src.Title, src.Audience, src.Wing, src.Link = strconv.Itoa(x.Version), x.Title, x.Audience, x.BuildingCode, "/#community?notice="+x.ID
	case "RECEIPT":
		var snapshot string
		e := q.QueryRowContext(ctx, `SELECT r.entry_id,r.number,r.snapshot_json,e.flat_id FROM receipts r JOIN entries e ON e.id=r.entry_id WHERE r.id=? AND e.kind='RECEIVED' AND e.state='POSTED'`, id).Scan(&src.EntryID, &src.Title, &snapshot, &src.HomeID)
		if e != nil {
			return src, e
		}
		hash := sha256.Sum256([]byte(snapshot))
		src.Version, src.Title, src.Audience, src.Link = hex.EncodeToString(hash[:]), "Original receipt "+src.Title, "FINANCIAL_HOME", "/#receipts?entry="+src.EntryID
	default:
		return src, ErrInvalid
	}
	return src, nil
}
func messagePeople(ctx context.Context, q identityReader) ([]messagePerson, error) {
	rows, e := q.QueryContext(ctx, `SELECT r.id,r.full_name,EXISTS(SELECT 1 FROM users u WHERE u.resident_id=r.id AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL),
 COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(c.state,'NONE'),COALESCE(c.version,0),COALESCE(c.community_whatsapp,0),COALESCE(c.community_email,0),COALESCE(c.finance_whatsapp,0),COALESCE(c.finance_email,0),
 COALESCE(m.flat_id,''),COALESCE(b.code,''),COALESCE(m.relationship,''),COALESCE(m.can_view_finances,0)
 FROM residents r LEFT JOIN resident_contacts c ON c.resident_id=r.id
 LEFT JOIN flat_memberships m ON m.resident_id=r.id AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)
 LEFT JOIN flats f ON f.id=m.flat_id LEFT JOIN buildings b ON b.id=f.building_id ORDER BY r.id,m.flat_id,m.relationship`, today(), today())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []messagePerson{}
	for rows.Next() {
		var p messagePerson
		var h messageMembership
		if e = rows.Scan(&p.ID, &p.Name, &p.ActiveAccount, &p.Contact.Phone, &p.Contact.Email, &p.Contact.State, &p.Contact.Version, &p.Contact.CommunityWhatsApp, &p.Contact.CommunityEmail, &p.Contact.FinanceWhatsApp, &p.Contact.FinanceEmail, &h.ID, &h.Wing, &h.Relationship, &h.Finance); e != nil {
			return nil, e
		}
		if len(out) == 0 || out[len(out)-1].ID != p.ID {
			p.Homes = []messageMembership{}
			out = append(out, p)
		}
		if h.ID != "" {
			out[len(out)-1].Homes = append(out[len(out)-1].Homes, h)
		}
		if len(out) > 2000 {
			return nil, invalid("This preview supports at most 2,000 people; narrow the registered audience.")
		}
	}
	return out, rows.Err()
}
func messageTargetMatches(p messagePerson, t MessageTarget) bool {
	if t.Kind == "PEOPLE" {
		for _, id := range t.IDs {
			if id == p.ID {
				return true
			}
		}
		return false
	}
	for _, h := range p.Homes {
		if t.Kind == "ALL" || (t.Kind == "OWNERS" && h.Relationship == "OWNER") || (t.Kind == "TENANTS" && h.Relationship == "TENANT") || (t.Kind == "WING" && h.Wing == t.Wing) {
			return true
		}
		if t.Kind == "HOMES" {
			for _, id := range t.IDs {
				if id == h.ID {
					return true
				}
			}
		}
	}
	return false
}
func messageSourceMatches(p messagePerson, src MessageSource) bool {
	for _, h := range p.Homes {
		if src.Kind == "RECEIPT" {
			if h.ID == src.HomeID && h.Finance {
				return true
			}
			continue
		}
		if src.Audience == "ALL_RESIDENTS" || (src.Audience == "OWNERS_ONLY" && h.Relationship == "OWNER") || (src.Audience == "TENANTS_ONLY" && h.Relationship == "TENANT") || (src.Audience == "BUILDING" && h.Wing == src.Wing) {
			return true
		}
	}
	return false
}
func messageConsent(c Contact, channel, purpose string) bool {
	if purpose == "COMMUNITY" {
		if channel == "WHATSAPP" {
			return c.CommunityWhatsApp
		}
		return c.CommunityEmail
	}
	if channel == "WHATSAPP" {
		return c.FinanceWhatsApp
	}
	return c.FinanceEmail
}
func messageRecipientReason(p messagePerson, src MessageSource, channel, purpose string) (string, string) {
	if len(p.Homes) == 0 {
		return "NO_CURRENT_HOME", ""
	}
	if !messageSourceMatches(p, src) {
		return "NO_SOURCE_ACCESS", ""
	}
	if p.Contact.State != "VERIFIED" {
		return "CONTACT_" + p.Contact.State, ""
	}
	dest := p.Contact.Email
	if channel == "WHATSAPP" {
		dest = p.Contact.Phone
	}
	if dest == "" {
		return "NO_DESTINATION", ""
	}
	if !messageConsent(p.Contact, channel, purpose) {
		return "OPTED_OUT", ""
	}
	if !p.ActiveAccount {
		return "NO_ACTIVE_ACCOUNT", ""
	}
	return "", dest
}
func resolveMessage(ctx context.Context, q identityReader, in MessageInput) (messageResolution, error) {
	src, e := messageSourceIn(ctx, q, in.SourceKind, in.SourceID)
	if e != nil {
		return messageResolution{}, e
	}
	people, e := messagePeople(ctx, q)
	if e != nil {
		return messageResolution{}, e
	}
	purpose := "COMMUNITY"
	title := src.Title
	if src.Kind == "RECEIPT" {
		purpose = "FINANCE"
		title = "A receipt is available in your society portal."
	}
	x := messageResolution{MessagePreview: MessagePreview{Source: src, Channel: in.Channel, Purpose: purpose, Target: in.Target, Envelope: title + "\n" + in.PortalOrigin + src.Link, Simulation: true, Counts: MessageCounts{Reasons: map[string]int{}}}, People: []MessageRecipient{}}
	destinations := map[string]bool{}
	seen := map[string]bool{}
	for _, p := range people {
		if !messageTargetMatches(p, in.Target) {
			continue
		}
		seen[p.ID] = true
		for _, h := range p.Homes {
			seen[h.ID] = true
		}
		x.Counts.TargetPeople++
		r := MessageRecipient{ID: p.ID, Name: p.Name}
		entitled := messageSourceMatches(p, src)
		if entitled {
			x.Counts.SourcePeople++
			r.ContactVersion = p.Contact.Version
		}
		dest := p.Contact.Email
		if in.Channel == "WHATSAPP" {
			dest = p.Contact.Phone
		}
		if entitled && p.Contact.State == "VERIFIED" && dest != "" && messageConsent(p.Contact, in.Channel, purpose) {
			x.Counts.ConsentedPeople++
		}
		r.Reason, r.Destination = messageRecipientReason(p, src, in.Channel, purpose)
		if r.Reason == "" {
			x.Counts.EligiblePeople++
			destinations[r.Destination] = true
		} else {
			x.Counts.OmittedPeople++
			x.Counts.Reasons[r.Reason]++
		}
		x.People = append(x.People, r)
	}
	if in.Target.Kind == "PEOPLE" {
		for _, id := range in.Target.IDs {
			if !seen[id] {
				return x, invalid("One selected person is not in the registered audience.")
			}
		}
	}
	if in.Target.Kind == "HOMES" {
		for _, id := range in.Target.IDs {
			var exists bool
			if e = q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)", id).Scan(&exists); e != nil {
				return x, e
			}
			if !exists {
				return x, ErrInvalid
			}
		}
	}
	x.Counts.Destinations = len(destinations)
	blob, e := json.Marshal(struct {
		Source                             MessageSource
		Channel, Purpose, Origin, Envelope string
		Target                             MessageTarget
		Counts                             MessageCounts
		People                             []MessageRecipient
	}{src, in.Channel, purpose, in.PortalOrigin, x.Envelope, in.Target, x.Counts, x.People})
	if e != nil {
		return x, e
	}
	hash := sha256.Sum256(blob)
	x.PreviewHash = hex.EncodeToString(hash[:])
	return x, nil
}
func maskMessageDestination(dest string) string {
	if strings.Contains(dest, "@") {
		parts := strings.SplitN(dest, "@", 2)
		return "••••@" + parts[1]
	}
	if len(dest) > 4 {
		return "••••" + dest[len(dest)-4:]
	}
	return "••••"
}
func publicMessageRecipient(r MessageRecipient, p Principal, kind string) MessageRecipient {
	if kind == "RECEIPT" && !p.CanManageContacts {
		if r.Reason == "NO_SOURCE_ACCESS" || r.Reason == "NO_CURRENT_HOME" {
			r.ID, r.Name = "", ""
			r.ContactVersion = 0
		}
		if r.Destination != "" {
			r.Destination = maskMessageDestination(r.Destination)
		}
	}
	return r
}
func (s *Store) MessagePreviewFor(ctx context.Context, token string, input MessageInput, page int) (MessagePreview, error) {
	out := MessagePreview{Recipients: []MessageRecipient{}, Page: page, PageSize: 20, Simulation: true}
	if page < 1 || page > 10000 {
		return out, ErrInvalid
	}
	in, e := validateMessageInput(input)
	if e != nil {
		return out, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e != nil {
		return out, e
	}
	if e = messageAuthority(p, in.SourceKind, false); e != nil {
		return out, e
	}
	x, e := resolveMessage(ctx, tx, in)
	if e != nil {
		return out, e
	}
	out = x.MessagePreview
	out.Page, out.PageSize, out.Recipients = page, 20, []MessageRecipient{}
	first := (page - 1) * 20
	for i := first; i < len(x.People) && i < first+20; i++ {
		out.Recipients = append(out.Recipients, publicMessageRecipient(x.People[i], p, in.SourceKind))
	}
	return out, tx.Commit()
}
func (s *Store) MessageSourcesFor(ctx context.Context, token, kind, query string, page int) (MessageSourcePage, error) {
	out := MessageSourcePage{Items: []MessageSource{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || len(query) > 100 {
		return out, ErrInvalid
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e != nil {
		return out, e
	}
	if e = messageAuthority(p, kind, false); e != nil {
		return out, e
	}
	from, condition, id := "review_requests x", "x.kind='NOTICE' AND x.state='APPROVED' AND x.audience<>'COMMITTEE_ONLY' AND x.title LIKE ?", "x.id"
	if kind == "RECEIPT" {
		from = "receipts x JOIN entries e ON e.id=x.entry_id"
		condition = "e.kind='RECEIVED' AND e.state='POSTED' AND x.number LIKE ?"
	}
	args := []any{"%" + query + "%"}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+from+" WHERE "+condition, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, "SELECT "+id+" FROM "+from+" WHERE "+condition+" ORDER BY "+id+" LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
	if e != nil {
		return out, e
	}
	ids := []string{}
	for rows.Next() {
		var found string
		if e = rows.Scan(&found); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, found)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		x, e := messageSourceIn(ctx, tx, kind, id)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	return out, tx.Commit()
}

const messageBatchSelect = `SELECT id,source_json,target_json,counts_json,preview_hash,channel,purpose,portal_origin,envelope,state,proposed_by,proposed_at,COALESCE(reviewed_by,''),reviewed_at,updated_at,snapshot_version,version FROM message_batches`

func scanMessageBatch(row interface{ Scan(...any) error }) (MessageBatch, error) {
	x := MessageBatch{Simulation: true, Outcomes: map[string]int{}}
	var source, target, counts string
	e := row.Scan(&x.ID, &source, &target, &counts, &x.PreviewHash, &x.Channel, &x.Purpose, &x.PortalOrigin, &x.Envelope, &x.State, &x.ProposedBy, &x.ProposedAt, &x.ReviewedBy, &x.ReviewedAt, &x.UpdatedAt, &x.SnapshotVersion, &x.Version)
	if e != nil {
		return x, e
	}
	for _, v := range []struct {
		s string
		p any
	}{{source, &x.Source}, {target, &x.Target}, {counts, &x.Counts}} {
		if e = json.Unmarshal([]byte(v.s), v.p); e != nil {
			return x, e
		}
	}
	return x, nil
}
func messageBatchIn(ctx context.Context, q identityReader, id string) (MessageBatch, error) {
	return scanMessageBatch(q.QueryRowContext(ctx, messageBatchSelect+" WHERE id=?", id))
}
func messageBatchEvent(ctx context.Context, tx *sql.Tx, p Principal, x MessageBatch, action, reason string) error {
	blob, e := json.Marshal(x)
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO message_events(batch_id,version,actor_id,action,reason,occurred_at,snapshot_json) VALUES(?,?,?,?,?,?,?)`, x.ID, x.Version, p.ID, action, reason, time.Now().Unix(), string(blob)); e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "MESSAGE_"+action, "Message decision recorded.", nil, map[string]any{"id": x.ID, "version": x.Version, "state": x.State, "kind": x.Source.Kind, "counts": x.Counts, "simulation": true})
}
func storeMessageRecipients(ctx context.Context, tx *sql.Tx, x MessageBatch, people []MessageRecipient) error {
	groups := map[string]string{}
	for _, r := range people {
		delivery, disposition := "", "OMITTED"
		if r.Reason == "" {
			disposition = "ELIGIBLE"
			delivery = groups[r.Destination]
			if delivery == "" {
				delivery = randomToken()
				groups[r.Destination] = delivery
				if _, e := tx.ExecContext(ctx, `INSERT INTO message_deliveries(id,batch_id,snapshot_version,destination,state,updated_at) VALUES(?,?,?,?,'QUEUED',?)`, delivery, x.ID, x.SnapshotVersion, r.Destination, time.Now().Unix()); e != nil {
					return e
				}
			}
		}
		var nullable any
		if delivery != "" {
			nullable = delivery
		}
		if _, e := tx.ExecContext(ctx, `INSERT INTO message_recipients(batch_id,snapshot_version,resident_id,name,contact_version,frozen_reason,delivery_id,disposition,reason) VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, x.SnapshotVersion, r.ID, r.Name, r.ContactVersion, r.Reason, nullable, disposition, r.Reason); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) ProposeMessage(ctx context.Context, token string, input MessageInput) (string, error) {
	in, e := validateMessageInput(input)
	if e != nil {
		return "", e
	}
	if !in.Confirmed || !validText(in.Reason, 5, 300) || len(in.PreviewHash) != 64 {
		return "", invalid("Review the content, recipients and omissions, confirm and record a reason.")
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if e = messageAuthority(p, in.SourceKind, true); e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MESSAGE_PROPOSE", in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	resolved, e := resolveMessage(ctx, tx, in)
	if e != nil {
		return "", e
	}
	if resolved.PreviewHash != in.PreviewHash {
		return "", ErrConflict
	}
	if resolved.Counts.Destinations == 0 {
		return "", invalid("No eligible destinations. Update the audience, registered permission or portal identities first.")
	}
	now := time.Now().Unix()
	x := MessageBatch{ID: randomToken(), Source: resolved.Source, Target: in.Target, Counts: resolved.Counts, PreviewHash: resolved.PreviewHash, Channel: in.Channel, Purpose: resolved.Purpose, PortalOrigin: in.PortalOrigin, Envelope: resolved.Envelope, State: "PENDING", ProposedBy: p.ID, ProposedAt: now, UpdatedAt: now, SnapshotVersion: 1, Version: 1, Simulation: true}
	source, _ := json.Marshal(x.Source)
	target, _ := json.Marshal(x.Target)
	counts, _ := json.Marshal(x.Counts)
	if _, e = tx.ExecContext(ctx, `INSERT INTO message_batches(id,source_kind,source_id,source_json,target_json,counts_json,preview_hash,channel,purpose,portal_origin,envelope,state,proposed_by,proposed_at,updated_at,snapshot_version,version) VALUES(?,?,?,?,?,?,?,?,?,?,?,'PENDING',?,?,?,1,1)`, x.ID, in.SourceKind, in.SourceID, string(source), string(target), string(counts), x.PreviewHash, x.Channel, x.Purpose, x.PortalOrigin, x.Envelope, p.ID, now, now); e != nil {
		return "", e
	}
	if e = storeMessageRecipients(ctx, tx, x, resolved.People); e != nil {
		return "", e
	}
	if e = messageBatchEvent(ctx, tx, p, x, "PROPOSED", in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, x.ID); e != nil {
		return "", e
	}
	return x.ID, tx.Commit()
}
func (s *Store) ActOnMessage(ctx context.Context, token, id string, in MessageAction) (string, error) {
	if in.Version < 1 || !in.Confirmed || !validText(in.Reason, 5, 300) || in.Outcome != "" {
		return "", ErrInvalid
	}
	if in.Action != "APPROVED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "REFRESH" && in.Action != "CANCELLED" {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := messageBatchIn(ctx, tx, id)
	if e != nil {
		return "", e
	}
	if e = messageAuthority(p, x.Source.Kind, true); e != nil {
		return "", e
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "MESSAGE_ACTION:"+id, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	switch in.Action {
	case "APPROVED", "DECLINED":
		if p.ID == x.ProposedBy {
			return "", ErrForbidden
		}
		if x.State != "PENDING" {
			return "", ErrConflict
		}
		if in.Action == "APPROVED" {
			current, err := messageOperatorCurrent(ctx, tx, x.ProposedBy, x.Source.Kind)
			if err != nil {
				return "", err
			}
			if !current {
				return "", ErrConflict
			}
			resolved, e := resolveMessage(ctx, tx, MessageInput{SourceKind: x.Source.Kind, SourceID: x.Source.ID, Channel: x.Channel, Target: x.Target, PortalOrigin: x.PortalOrigin})
			if e != nil {
				return "", e
			}
			if resolved.PreviewHash != x.PreviewHash {
				return "", ErrConflict
			}
		}
		x.State, x.ReviewedBy, x.ReviewedAt = in.Action, p.ID, time.Now().Unix()
	case "WITHDRAWN", "REFRESH":
		if p.ID != x.ProposedBy {
			return "", ErrForbidden
		}
		if x.State != "PENDING" {
			return "", ErrConflict
		}
		if in.Action == "WITHDRAWN" {
			x.State = "WITHDRAWN"
		} else {
			resolved, e := resolveMessage(ctx, tx, MessageInput{SourceKind: x.Source.Kind, SourceID: x.Source.ID, Channel: x.Channel, Target: x.Target, PortalOrigin: x.PortalOrigin})
			if e != nil {
				return "", e
			}
			if resolved.Counts.Destinations == 0 {
				return "", invalid("No eligible destination remains in this proposed audience.")
			}
			if e = cancelMessageUnsent(ctx, tx, x, "SNAPSHOT_REFRESHED"); e != nil {
				return "", e
			}
			x.Source, x.Counts, x.PreviewHash, x.Envelope = resolved.Source, resolved.Counts, resolved.PreviewHash, resolved.Envelope
			x.SnapshotVersion++
			if e = storeMessageRecipients(ctx, tx, x, resolved.People); e != nil {
				return "", e
			}
		}
	case "CANCELLED":
		if x.State != "APPROVED" {
			return "", ErrConflict
		}
		x.State = "CANCELLED"
	}
	if in.Action == "DECLINED" || in.Action == "WITHDRAWN" || in.Action == "CANCELLED" {
		if e = cancelMessageUnsent(ctx, tx, x, in.Action); e != nil {
			return "", e
		}
	}
	x.Version++
	x.UpdatedAt = time.Now().Unix()
	source, _ := json.Marshal(x.Source)
	counts, _ := json.Marshal(x.Counts)
	if _, e = tx.ExecContext(ctx, `UPDATE message_batches SET source_json=?,counts_json=?,preview_hash=?,envelope=?,state=?,reviewed_by=NULLIF(?,''),reviewed_at=?,updated_at=?,snapshot_version=?,version=? WHERE id=? AND version=?`, string(source), string(counts), x.PreviewHash, x.Envelope, x.State, x.ReviewedBy, x.ReviewedAt, x.UpdatedAt, x.SnapshotVersion, x.Version, x.ID, in.Version); e != nil {
		return "", e
	}
	if e = messageBatchEvent(ctx, tx, p, x, in.Action, in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, id); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
