package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/mail"
	"strings"
	"time"
)

type UpkeepRegisterInput struct {
	OperationKey    string `json:"operation_key"`
	Version         int    `json:"version"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Location        string `json:"location"`
	Contact         string `json:"contact"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	VendorID        string `json:"vendor_id"`
	SourceReference string `json:"source_reference"`
	AMCStart        string `json:"amc_start"`
	AMCEnd          string `json:"amc_end"`
	InspectionDate  string `json:"inspection_date"`
	State           string `json:"state"`
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed"`
}
type UpkeepRegister struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	Location        string `json:"location"`
	Contact         string `json:"contact"`
	Phone           string `json:"phone"`
	Email           string `json:"email"`
	VendorID        string `json:"vendor_id"`
	Vendor          string `json:"vendor"`
	SourceReference string `json:"source_reference"`
	AMCStart        string `json:"amc_start"`
	AMCEnd          string `json:"amc_end"`
	InspectionDate  string `json:"inspection_date"`
	State           string `json:"state"`
	Version         int    `json:"version"`
	CreatedBy       string `json:"created_by"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}
type UpkeepTaskInput struct {
	OperationKey string `json:"operation_key"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Category     string `json:"category"`
	Priority     string `json:"priority"`
	DueDate      string `json:"due_date"`
	VisitDate    string `json:"visit_date"`
	AssetID      string `json:"asset_id"`
	VendorID     string `json:"vendor_id"`
	AssignedTo   string `json:"assigned_to"`
	ComplaintID  string `json:"complaint_id"`
	RepeatDays   int    `json:"repeat_days"`
	ParentID     string `json:"parent_id"`
	Confirmed    bool   `json:"confirmed"`
}
type UpkeepAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	AssignedTo   string `json:"assigned_to"`
	DueDate      string `json:"due_date"`
	VisitDate    string `json:"visit_date"`
	Audience     string `json:"audience"`
	BuildingCode string `json:"building_code"`
	PublicTitle  string `json:"public_title"`
	PublicBody   string `json:"public_body"`
	Confirmed    bool   `json:"confirmed"`
}
type UpkeepTask struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	Body             string             `json:"body"`
	Category         string             `json:"category"`
	Priority         string             `json:"priority"`
	DueDate          string             `json:"due_date"`
	VisitDate        string             `json:"visit_date"`
	AssetID          string             `json:"asset_id,omitempty"`
	Asset            string             `json:"asset,omitempty"`
	VendorID         string             `json:"vendor_id,omitempty"`
	Vendor           string             `json:"vendor,omitempty"`
	AssignedTo       string             `json:"assigned_to,omitempty"`
	AssignedName     string             `json:"assigned_name,omitempty"`
	AssigneeEligible bool               `json:"assignee_eligible,omitempty"`
	ComplaintID      string             `json:"complaint_id,omitempty"`
	ComplaintNumber  string             `json:"complaint_number,omitempty"`
	RepeatDays       int                `json:"repeat_days,omitempty"`
	ParentID         string             `json:"parent_id,omitempty"`
	State            string             `json:"state"`
	Version          int                `json:"version"`
	CreatedBy        string             `json:"created_by,omitempty"`
	CreatedAt        int64              `json:"created_at"`
	UpdatedAt        int64              `json:"updated_at"`
	ReadyBy          string             `json:"ready_by,omitempty"`
	ReadyName        string             `json:"ready_name,omitempty"`
	ReadyAt          int64              `json:"ready_at,omitempty"`
	CheckedBy        string             `json:"checked_by,omitempty"`
	CheckedName      string             `json:"checked_name,omitempty"`
	CheckedAt        int64              `json:"checked_at,omitempty"`
	Audience         string             `json:"audience"`
	BuildingCode     string             `json:"building_code"`
	PublicVersion    int                `json:"public_version,omitempty"`
	PublicSnapshot   *UpkeepPublication `json:"public_snapshot,omitempty"`
	PublishedAt      int64              `json:"published_at"`
}
type UpkeepPublication struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Category  string `json:"category"`
	Priority  string `json:"priority"`
	DueDate   string `json:"due_date"`
	VisitDate string `json:"visit_date"`
	State     string `json:"state"`
}
type UpkeepEvent struct {
	Action  string `json:"action"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
	Version int    `json:"version"`
	At      int64  `json:"at"`
}
type UpkeepDetail struct {
	UpkeepTask
	Events     []UpkeepEvent `json:"events,omitempty"`
	EventTotal int           `json:"event_total,omitempty"`
	EventPage  int           `json:"event_page,omitempty"`
	PageSize   int           `json:"page_size"`
}
type UpkeepRegisterDetail struct {
	UpkeepRegister
	Events     []UpkeepEvent `json:"events"`
	EventTotal int           `json:"event_total"`
	EventPage  int           `json:"event_page"`
	PageSize   int           `json:"page_size"`
}
type UpkeepTaskPage struct {
	Items    []UpkeepTask     `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Counts   map[string]int64 `json:"counts"`
}
type UpkeepRegisterPage struct {
	Items    []UpkeepRegister `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
type UpkeepOptions struct {
	Vendors   []RecordHome       `json:"vendors"`
	Assets    []RecordHome       `json:"assets"`
	Handlers  []ComplaintHandler `json:"handlers"`
	Buildings []RecordHome       `json:"buildings"`
}

func beginUpkeepRead(ctx context.Context, s *Store, token string, operator bool) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && operator && !p.CanHandleComplaints {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func (s *Store) beginUpkeepWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, p, err := s.beginReviewWrite(ctx, token)
	if err != nil {
		return nil, p, err
	}
	if !p.CanHandleComplaints {
		tx.Rollback()
		return nil, p, ErrForbidden
	}
	return tx, p, nil
}
func optionalID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
func optionalUpkeepDate(value string) bool { return value == "" || validMaintenanceDate(value) }
func upkeepKind(kind string) bool          { return kind == "VENDOR" || kind == "ASSET" }
func upkeepState(state string) bool {
	return state == "PLANNED" || state == "IN_PROGRESS" || state == "WAITING" || state == "READY_FOR_CHECK" || state == "DONE" || state == "CANCELLED"
}
func activeUpkeepReference(ctx context.Context, q identityReader, id, kind string) error {
	if id == "" {
		return nil
	}
	if len(id) > 100 {
		return ErrInvalid
	}
	var active bool
	if err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM upkeep_register WHERE id=? AND kind=? AND state='ACTIVE')", id, kind).Scan(&active); err != nil {
		return err
	}
	if !active {
		return invalid("A selected asset or vendor is no longer active. Reload and choose a current record.")
	}
	return nil
}
func currentUpkeepAssignee(ctx context.Context, q identityReader, id string) error {
	if id == "" {
		return nil
	}
	if len(id) > 100 {
		return ErrInvalid
	}
	now := time.Now().Unix()
	var eligible bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users u WHERE u.id=? AND u.status='ACTIVE' AND u.suspended_at IS NULL AND u.verified_at IS NOT NULL AND EXISTS(SELECT 1 FROM mfa_factors WHERE user_id=u.id) AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=u.id AND g.role IN ('ADMINISTRATOR','COMMITTEE') AND g.valid_from<=? AND g.valid_until>? AND g.revoked_at IS NULL))", id, now, now).Scan(&eligible)
	if err != nil {
		return err
	}
	if !eligible {
		return invalid("Choose a currently eligible operational assignee.")
	}
	return nil
}
func upkeepEvent(ctx context.Context, tx *sql.Tx, p Principal, table, field, id, action, reason string, version int, snapshot any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO "+table+"("+field+",version,actor_id,action,reason,occurred_at,snapshot_json) VALUES(?,?,?,?,?,?,?)", id, version, p.ID, action, reason, time.Now().Unix(), string(data))
	if err != nil {
		return err
	}
	return appendAudit(ctx, tx, p.ID, "", "UPKEEP_"+action, reason, map[string]any{"id": id}, map[string]any{"id": id, "version": version})
}
func validateUpkeepRegister(kind string, in UpkeepRegisterInput) error {
	if !upkeepKind(kind) || !in.Confirmed || !validText(in.Name, 3, 120) || !validText(in.Category, 2, 50) || !validText(in.SourceReference, 5, 300) || !validText(in.Reason, 10, 300) || (in.State != "ACTIVE" && in.State != "INACTIVE") {
		return invalid("Supply the name, category, source reference, state and a confirmed reason.")
	}
	if kind == "VENDOR" {
		if in.Location != "" || in.VendorID != "" || in.AMCStart != "" || in.AMCEnd != "" || in.InspectionDate != "" || !validText(in.Contact, 0, 120) {
			return ErrInvalid
		}
		if in.Phone != "" {
			phone := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(in.Phone)
			digits := strings.TrimPrefix(phone, "+")
			if len(digits) < 7 || len(digits) > 15 || strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return invalid("Use a phone number of 7–15 digits, with an optional country prefix.")
			}
		}
		if in.Email != "" {
			a, err := mail.ParseAddress(in.Email)
			if err != nil || a.Address != in.Email || len(in.Email) > 254 {
				return invalid("Supply a valid email address.")
			}
		}
	} else {
		if !validText(in.Location, 3, 200) || in.Contact != "" || in.Phone != "" || in.Email != "" || !optionalUpkeepDate(in.AMCStart) || !optionalUpkeepDate(in.AMCEnd) || !optionalUpkeepDate(in.InspectionDate) || (in.AMCStart == "") != (in.AMCEnd == "") || in.AMCEnd < in.AMCStart {
			return invalid("Supply a location and valid paired AMC dates, with an optional explicit inspection date.")
		}
	}
	return nil
}
func (s *Store) SaveUpkeepRegister(ctx context.Context, token, kind, id string, in UpkeepRegisterInput) (string, error) {
	if err := validateUpkeepRegister(kind, in); err != nil {
		return "", err
	}
	if len(id) > 100 || (id == "" && in.Version != 0) || (id != "" && in.Version < 1) {
		return "", ErrInvalid
	}
	tx, p, err := s.beginUpkeepWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "UPKEEP_REGISTER:"+kind+":"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	now, action := time.Now().Unix(), "REGISTER_CREATED"
	if id == "" {
		if err := activeUpkeepReference(ctx, tx, in.VendorID, "VENDOR"); err != nil {
			return "", err
		}
		id = randomToken()
		_, err = tx.ExecContext(ctx, "INSERT INTO upkeep_register(id,kind,name,category,location,contact,phone,email,vendor_id,source_reference,amc_start,amc_end,inspection_date,state,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", id, kind, in.Name, in.Category, in.Location, in.Contact, in.Phone, in.Email, optionalID(in.VendorID), in.SourceReference, in.AMCStart, in.AMCEnd, in.InspectionDate, in.State, p.ID, now, now)
	} else {
		old, e := scanUpkeepRegister(tx.QueryRowContext(ctx, upkeepRegisterSelect+" WHERE r.id=? AND r.kind=?", id, kind))
		if e != nil {
			return "", e
		}
		if old.Version != in.Version {
			return "", ErrConflict
		}
		if old.VendorID != in.VendorID {
			if err := activeUpkeepReference(ctx, tx, in.VendorID, "VENDOR"); err != nil {
				return "", err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE upkeep_register SET name=?,category=?,location=?,contact=?,phone=?,email=?,vendor_id=?,source_reference=?,amc_start=?,amc_end=?,inspection_date=?,state=?,version=version+1,updated_at=? WHERE id=? AND version=?", in.Name, in.Category, in.Location, in.Contact, in.Phone, in.Email, optionalID(in.VendorID), in.SourceReference, in.AMCStart, in.AMCEnd, in.InspectionDate, in.State, now, id, in.Version)
		action = "REGISTER_UPDATED"
	}
	if err != nil {
		return "", err
	}
	if in.State == "ACTIVE" {
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM upkeep_register WHERE kind=? AND state='ACTIVE'", kind).Scan(&count); err != nil {
			return "", err
		}
		if count > 500 {
			return "", invalid("The active register supports up to 500 records of each kind. Inactivate an unused record before adding another.")
		}
	}
	saved, err := scanUpkeepRegister(tx.QueryRowContext(ctx, upkeepRegisterSelect+" WHERE r.id=?", id))
	if err != nil {
		return "", err
	}
	if err := upkeepEvent(ctx, tx, p, "upkeep_register_events", "register_id", id, action, in.Reason, saved.Version, saved); err != nil {
		return "", err
	}
	if err := saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
