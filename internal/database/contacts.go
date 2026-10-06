package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

type ContactPreferences struct {
	CommunityWhatsApp bool `json:"community_whatsapp"`
	CommunityEmail    bool `json:"community_email"`
	FinanceWhatsApp   bool `json:"finance_whatsapp"`
	FinanceEmail      bool `json:"finance_email"`
}
type ContactInput struct {
	ContactPreferences
	OperationKey     string `json:"operation_key"`
	Version          int    `json:"version"`
	Phone            string `json:"phone"`
	Email            string `json:"email"`
	PreferredChannel string `json:"preferred_channel"`
	ConsentSource    string `json:"consent_source"`
	Reason           string `json:"reason"`
	Confirmed        bool   `json:"confirmed"`
}
type ContactAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Channel      string `json:"channel"`
	Purpose      string `json:"purpose"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type Contact struct {
	ContactPreferences
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Phone            string             `json:"phone"`
	Email            string             `json:"email"`
	PreferredChannel string             `json:"preferred_channel"`
	ConsentSource    string             `json:"consent_source"`
	State            string             `json:"state"`
	SubmittedBy      string             `json:"submitted_by"`
	SubmittedAt      int64              `json:"submitted_at"`
	ReviewedBy       string             `json:"reviewed_by"`
	ReviewedAt       int64              `json:"reviewed_at"`
	DecisionReason   string             `json:"decision_reason"`
	CreatedAt        int64              `json:"created_at"`
	UpdatedAt        int64              `json:"updated_at"`
	Version          int                `json:"version"`
	Current          bool               `json:"current"`
	CanRegister      bool               `json:"can_register"`
	CanVerify        bool               `json:"can_verify"`
	CanWithdraw      bool               `json:"can_withdraw"`
	CanOptOut        bool               `json:"can_opt_out"`
	Eligible         ContactPreferences `json:"eligible"`
	Homes            []RecordHome       `json:"homes"`
	Relationships    []string           `json:"relationships"`
}
type ContactEvent struct {
	Version  int     `json:"version"`
	Action   string  `json:"action"`
	Actor    string  `json:"actor"`
	At       int64   `json:"at"`
	Reason   string  `json:"reason"`
	Snapshot Contact `json:"snapshot"`
}
type ContactDetail struct {
	Contact
	Events     []ContactEvent `json:"events"`
	EventTotal int            `json:"event_total"`
	EventPage  int            `json:"event_page"`
	PageSize   int            `json:"page_size"`
}
type ContactPage struct {
	Items    []Contact `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

const contactSelect = `SELECT r.id,r.full_name,COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(c.preferred_channel,'NONE'),COALESCE(c.community_whatsapp,0),COALESCE(c.community_email,0),COALESCE(c.finance_whatsapp,0),COALESCE(c.finance_email,0),COALESCE(c.consent_source,''),COALESCE(c.state,'NONE'),COALESCE(c.submitted_by,''),COALESCE(c.submitted_at,0),COALESCE(c.reviewed_by,''),COALESCE(c.reviewed_at,0),COALESCE(c.decision_reason,''),COALESCE(c.created_at,0),COALESCE(c.updated_at,0),COALESCE(c.version,0) FROM residents r LEFT JOIN resident_contacts c ON c.resident_id=r.id`
const contactCurrent = `EXISTS(SELECT 1 FROM flat_memberships cm WHERE cm.resident_id=r.id AND cm.start_date<=? AND (cm.end_date IS NULL OR cm.end_date>?))`

func scanContact(row interface{ Scan(...any) error }) (Contact, error) {
	var x Contact
	e := row.Scan(&x.ID, &x.Name, &x.Phone, &x.Email, &x.PreferredChannel, &x.CommunityWhatsApp, &x.CommunityEmail, &x.FinanceWhatsApp, &x.FinanceEmail, &x.ConsentSource, &x.State, &x.SubmittedBy, &x.SubmittedAt, &x.ReviewedBy, &x.ReviewedAt, &x.DecisionReason, &x.CreatedAt, &x.UpdatedAt, &x.Version)
	return x, e
}
func contactTarget(p Principal, id string) (string, error) {
	if id == "me" {
		id = p.ResidentID
	}
	if id == "" || len(id) > 100 {
		return "", sql.ErrNoRows
	}
	if !p.CanReviewRequests && p.ResidentID != id {
		return "", sql.ErrNoRows
	}
	return id, nil
}
func decorateContact(ctx context.Context, q identityReader, p Principal, x *Contact) error {
	if e := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flat_memberships WHERE resident_id=? AND start_date<=? AND (end_date IS NULL OR end_date>?))`, x.ID, today(), today()).Scan(&x.Current); e != nil {
		return e
	}
	x.CanRegister = x.Current && (p.ResidentID == x.ID || p.CanManageRegistry)
	x.CanVerify = x.Current && x.State == "PENDING" && p.CanReviewRequests && x.SubmittedBy != p.ID && x.ID != p.ResidentID
	x.CanWithdraw = x.State != "NONE" && x.State != "WITHDRAWN" && (p.ResidentID == x.ID || x.SubmittedBy == p.ID || p.CanManageRegistry)
	x.CanOptOut = x.State != "NONE" && (p.ResidentID == x.ID || p.CanReviewRequests)
	if x.Current && x.State == "VERIFIED" {
		x.Eligible = x.ContactPreferences
	}
	x.Homes, x.Relationships = []RecordHome{}, []string{}
	rows, e := q.QueryContext(ctx, `SELECT DISTINCT f.id,b.code||'-'||f.flat_number,m.relationship FROM flat_memberships m JOIN flats f ON f.id=m.flat_id JOIN buildings b ON b.id=f.building_id WHERE m.resident_id=? AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?) ORDER BY b.code,f.flat_number,m.relationship`, x.ID, today(), today())
	if e != nil {
		return e
	}
	defer rows.Close()
	seenHome, seenRole := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var h RecordHome
		var role string
		if e = rows.Scan(&h.ID, &h.Label, &role); e != nil {
			return e
		}
		if !seenHome[h.ID] {
			x.Homes = append(x.Homes, h)
			seenHome[h.ID] = true
		}
		if !seenRole[role] {
			x.Relationships = append(x.Relationships, role)
			seenRole[role] = true
		}
	}
	return rows.Err()
}
func contactIn(ctx context.Context, q identityReader, p Principal, id string) (Contact, error) {
	id, e := contactTarget(p, id)
	if e != nil {
		return Contact{}, e
	}
	x, e := scanContact(q.QueryRowContext(ctx, contactSelect+" WHERE r.id=?", id))
	if e == nil {
		e = decorateContact(ctx, q, p, &x)
	}
	return x, e
}
func (s *Store) ContactsFor(ctx context.Context, token, search, relationship, building, state string, page int) (ContactPage, error) {
	out := ContactPage{Items: []Contact{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || len(search) > 100 || (relationship != "" && relationship != "OWNER" && relationship != "TENANT") || (building != "" && building != "A" && building != "B" && building != "C") || (state != "" && state != "NONE" && state != "PENDING" && state != "VERIFIED" && state != "DECLINED" && state != "WITHDRAWN") {
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
	where, args := " WHERE 1=1", []any{}
	if p.CanReviewRequests {
		where += " AND " + contactCurrent
		args = append(args, today(), today())
	} else {
		where += " AND r.id=?"
		args = append(args, p.ResidentID)
	}
	if search != "" {
		where += " AND r.full_name LIKE ?"
		args = append(args, "%"+search+"%")
	}
	if relationship != "" || building != "" {
		where += ` AND EXISTS(SELECT 1 FROM flat_memberships fm JOIN flats ff ON ff.id=fm.flat_id JOIN buildings fb ON fb.id=ff.building_id WHERE fm.resident_id=r.id AND fm.start_date<=? AND (fm.end_date IS NULL OR fm.end_date>?)`
		args = append(args, today(), today())
		if relationship != "" {
			where += " AND fm.relationship=?"
			args = append(args, relationship)
		}
		if building != "" {
			where += " AND fb.code=?"
			args = append(args, building)
		}
		where += ")"
	}
	if state != "" {
		where += " AND COALESCE(c.state,'NONE')=?"
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM residents r LEFT JOIN resident_contacts c ON c.resident_id=r.id"+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, contactSelect+where+" ORDER BY r.full_name,r.id LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, err := scanContact(rows)
		if err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		if e = decorateContact(ctx, tx, p, &out.Items[i]); e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func (s *Store) ContactFor(ctx context.Context, token, id string, page int) (ContactDetail, error) {
	out := ContactDetail{Events: []ContactEvent{}, EventPage: page, PageSize: 20}
	if page < 1 || page > 10000 {
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
	out.Contact, e = contactIn(ctx, tx, p, id)
	if e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM contact_events WHERE resident_id=?", out.ID).Scan(&out.EventTotal); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT e.version,e.action,u.display_name,e.occurred_at,e.reason,e.snapshot_json FROM contact_events e JOIN users u ON u.id=e.actor_id WHERE e.resident_id=? ORDER BY e.version DESC LIMIT 20 OFFSET ?`, out.ID, (page-1)*20)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var event ContactEvent
		var blob string
		if e = rows.Scan(&event.Version, &event.Action, &event.Actor, &event.At, &event.Reason, &blob); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(blob), &event.Snapshot); e != nil {
			rows.Close()
			return out, e
		}
		out.Events = append(out.Events, event)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}

var internationalPhone = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func normaliseContact(in ContactInput) (ContactInput, error) {
	in.Phone = strings.TrimSpace(in.Phone)
	if len(in.Phone) > 40 || len(in.Email) > 254 {
		return in, ErrInvalid
	}
	in.Phone = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(in.Phone)
	in.Email = strings.TrimSpace(in.Email)
	if in.Phone != "" && !internationalPhone.MatchString(in.Phone) {
		return in, invalid("Use an international phone number beginning with + and its country code.")
	}
	if in.Email != "" {
		address, e := mail.ParseAddress(in.Email)
		if e != nil || address.Address != in.Email || address.Name != "" || strings.ContainsAny(in.Email, "\r\n\x00") {
			return in, invalid("Use a plain email address without a name or extra header text.")
		}
		for _, c := range in.Email {
			if c <= 32 || c >= 127 {
				return in, invalid("Use a plain ASCII email address for this contact.")
			}
		}
		at := strings.LastIndexByte(in.Email, '@')
		if at < 1 || at == len(in.Email)-1 {
			return in, ErrInvalid
		}
		in.Email = in.Email[:at+1] + strings.ToLower(in.Email[at+1:])
	}
	if in.Phone == "" && in.Email == "" {
		return in, invalid("Supply at least one phone number or email address.")
	}
	if (in.Phone == "" && (in.CommunityWhatsApp || in.FinanceWhatsApp || in.PreferredChannel == "WHATSAPP")) || (in.Email == "" && (in.CommunityEmail || in.FinanceEmail || in.PreferredChannel == "EMAIL")) {
		return in, invalid("Select permission only for a supplied destination.")
	}
	if in.PreferredChannel != "NONE" && in.PreferredChannel != "WHATSAPP" && in.PreferredChannel != "EMAIL" {
		return in, ErrInvalid
	}
	if !in.Confirmed || in.Version < 0 || !validText(in.ConsentSource, 5, 300) || !paragraph(in.Reason, 10, 1000) {
		return in, invalid("Review the supplied identity/permission source and give a reason before registering.")
	}
	return in, nil
}
func contactEvent(ctx context.Context, tx *sql.Tx, p Principal, id, action, reason string) error {
	x, e := scanContact(tx.QueryRowContext(ctx, contactSelect+" WHERE r.id=?", id))
	if e != nil {
		return e
	}
	blob, e := json.Marshal(x)
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO contact_events(resident_id,version,actor_id,action,reason,occurred_at,snapshot_json) VALUES(?,?,?,?,?,?,?)`, id, x.Version, p.ID, action, reason, time.Now().Unix(), string(blob)); e != nil {
		return e
	}
	return appendAudit(ctx, tx, p.ID, "", "CONTACT_"+action, "Private contact decision retained", map[string]any{"resident_id": id}, map[string]any{"resident_id": id, "version": x.Version})
}
func (s *Store) RegisterContact(ctx context.Context, token, id string, input ContactInput) (string, error) {
	in, e := normaliseContact(input)
	if e != nil {
		return "", e
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := contactIn(ctx, tx, p, id)
	if e != nil {
		return "", e
	}
	if !x.CanRegister {
		return "", ErrForbidden
	}
	if !p.Fresh {
		return "", ErrReauthRequired
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "CONTACT_REGISTER:"+x.ID, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	_, e = tx.ExecContext(ctx, `INSERT INTO resident_contacts(resident_id,phone,email,preferred_channel,community_whatsapp,community_email,finance_whatsapp,finance_email,consent_source,state,submitted_by,submitted_at,created_at,updated_at,version) VALUES(?,?,?,?,?,?,?,?,?,'PENDING',?,?,?,?,1) ON CONFLICT(resident_id) DO UPDATE SET phone=excluded.phone,email=excluded.email,preferred_channel=excluded.preferred_channel,community_whatsapp=excluded.community_whatsapp,community_email=excluded.community_email,finance_whatsapp=excluded.finance_whatsapp,finance_email=excluded.finance_email,consent_source=excluded.consent_source,state='PENDING',submitted_by=excluded.submitted_by,submitted_at=excluded.submitted_at,reviewed_by=NULL,reviewed_at=0,decision_reason='',updated_at=excluded.updated_at,version=resident_contacts.version+1`, x.ID, in.Phone, in.Email, in.PreferredChannel, in.CommunityWhatsApp, in.CommunityEmail, in.FinanceWhatsApp, in.FinanceEmail, in.ConsentSource, p.ID, now, now, now)
	if e != nil {
		return "", e
	}
	if e = contactEvent(ctx, tx, p, x.ID, "REGISTERED", in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, x.ID); e != nil {
		return "", e
	}
	return x.ID, tx.Commit()
}
func (s *Store) ActOnContact(ctx context.Context, token, id string, in ContactAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !paragraph(in.Reason, 10, 1000) || (in.Action != "VERIFIED" && in.Action != "DECLINED" && in.Action != "WITHDRAWN" && in.Action != "OPTED_OUT") {
		return "", ErrInvalid
	}
	if in.Action == "OPTED_OUT" {
		if (in.Channel != "ALL" && in.Channel != "WHATSAPP" && in.Channel != "EMAIL") || (in.Purpose != "ALL" && in.Purpose != "COMMUNITY" && in.Purpose != "FINANCE") {
			return "", ErrInvalid
		}
	} else if in.Channel != "" || in.Purpose != "" {
		return "", ErrInvalid
	}
	tx, p, e := s.beginReviewWrite(ctx, token)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	x, e := contactIn(ctx, tx, p, id)
	if e != nil {
		return "", e
	}
	// Authority is checked before replay, even after the successful action has
	// changed the state. State/version validation applies only to a new action.
	if in.Action == "VERIFIED" || in.Action == "DECLINED" {
		if !p.CanReviewRequests || !x.Current || p.ID == x.SubmittedBy || p.ResidentID == x.ID {
			return "", ErrForbidden
		}
	} else if in.Action == "WITHDRAWN" {
		if p.ResidentID != x.ID && p.ID != x.SubmittedBy && !p.CanManageRegistry {
			return "", ErrForbidden
		}
	} else if !x.CanOptOut {
		return "", ErrForbidden
	}
	if (p.ResidentID != x.ID || in.Action == "VERIFIED" || in.Action == "DECLINED") && !p.Fresh {
		return "", ErrReauthRequired
	}
	result, hash, e := replayOperation(ctx, tx, p, in.OperationKey, "CONTACT_ACTION:"+x.ID, in)
	if e != nil {
		return "", e
	}
	if result != "" {
		return result, tx.Commit()
	}
	if x.Version != in.Version || x.State == "NONE" {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	if in.Action == "VERIFIED" || in.Action == "DECLINED" {
		if x.State != "PENDING" {
			return "", ErrConflict
		}
		_, e = tx.ExecContext(ctx, "UPDATE resident_contacts SET state=?,reviewed_by=?,reviewed_at=?,decision_reason=?,updated_at=?,version=version+1 WHERE resident_id=?", in.Action, p.ID, now, in.Reason, now, x.ID)
	} else if in.Action == "WITHDRAWN" {
		if x.State == "WITHDRAWN" {
			return "", ErrConflict
		}
		_, e = tx.ExecContext(ctx, "UPDATE resident_contacts SET state='WITHDRAWN',community_whatsapp=0,community_email=0,finance_whatsapp=0,finance_email=0,updated_at=?,version=version+1 WHERE resident_id=?", now, x.ID)
	} else {
		if in.Channel == "ALL" || in.Channel == "WHATSAPP" {
			if in.Purpose == "ALL" || in.Purpose == "COMMUNITY" {
				x.CommunityWhatsApp = false
			}
			if in.Purpose == "ALL" || in.Purpose == "FINANCE" {
				x.FinanceWhatsApp = false
			}
		}
		if in.Channel == "ALL" || in.Channel == "EMAIL" {
			if in.Purpose == "ALL" || in.Purpose == "COMMUNITY" {
				x.CommunityEmail = false
			}
			if in.Purpose == "ALL" || in.Purpose == "FINANCE" {
				x.FinanceEmail = false
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE resident_contacts SET community_whatsapp=?,community_email=?,finance_whatsapp=?,finance_email=?,updated_at=?,version=version+1 WHERE resident_id=?", x.CommunityWhatsApp, x.CommunityEmail, x.FinanceWhatsApp, x.FinanceEmail, now, x.ID)
	}
	if e != nil {
		return "", e
	}
	if e = contactEvent(ctx, tx, p, x.ID, in.Action, in.Reason); e != nil {
		return "", e
	}
	if e = saveOperation(ctx, tx, p, in.OperationKey, hash, x.ID); e != nil {
		return "", e
	}
	return x.ID, tx.Commit()
}
