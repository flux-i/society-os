package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type MaintenanceLineInput struct {
	FlatID string `json:"flat_id"`
	Amount string `json:"amount"`
}
type MaintenanceInput struct {
	OperationKey    string                 `json:"operation_key"`
	Title           string                 `json:"title"`
	PeriodStart     string                 `json:"period_start"`
	PeriodEnd       string                 `json:"period_end"`
	DueDate         string                 `json:"due_date"`
	SourceReference string                 `json:"source_reference"`
	Note            string                 `json:"note"`
	Lines           []MaintenanceLineInput `json:"lines"`
	Confirmed       bool                   `json:"confirmed"`
}
type MaintenanceAction struct {
	OperationKey string `json:"operation_key"`
	Version      int    `json:"version"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type CreditAllocationInput struct {
	OperationKey string `json:"operation_key"`
	SourceID     string `json:"source_id"`
	ChargeID     string `json:"charge_id"`
	Amount       string `json:"amount"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type AllocationCorrection struct {
	OperationKey string `json:"operation_key"`
	Reason       string `json:"reason"`
	Confirmed    bool   `json:"confirmed"`
}
type MaintenanceTotals struct {
	RequestedPaise   int64 `json:"requested_paise"`
	ActivePaise      int64 `json:"active_paise"`
	AllocatedPaise   int64 `json:"allocated_paise"`
	OutstandingPaise int64 `json:"outstanding_paise"`
	OverduePaise     int64 `json:"overdue_paise"`
	ReversedPaise    int64 `json:"reversed_paise"`
}
type MaintenanceCycle struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PeriodStart     string `json:"period_start"`
	PeriodEnd       string `json:"period_end"`
	DueDate         string `json:"due_date"`
	SourceReference string `json:"source_reference,omitempty"`
	Note            string `json:"note,omitempty"`
	State           string `json:"state"`
	Version         int    `json:"version"`
	AuthorID        string `json:"author_id,omitempty"`
	Author          string `json:"author,omitempty"`
	SubmittedAt     int64  `json:"submitted_at"`
	DecidedBy       string `json:"decided_by,omitempty"`
	DecidedAt       int64  `json:"decided_at"`
	DecisionReason  string `json:"decision_reason,omitempty"`
	Participants    int    `json:"participants"`
	MaintenanceTotals
}
type MaintenanceLine struct {
	FlatID           string `json:"flat_id"`
	Home             string `json:"home"`
	EntryID          string `json:"entry_id"`
	State            string `json:"state"`
	AmountPaise      int64  `json:"amount_paise"`
	AllocatedPaise   int64  `json:"allocated_paise"`
	OutstandingPaise int64  `json:"outstanding_paise"`
}
type MaintenanceEvent struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
	Actor   string `json:"actor"`
	Reason  string `json:"reason"`
	At      int64  `json:"at"`
}
type MaintenanceDetails struct {
	MaintenanceCycle
	Lines      []MaintenanceLine  `json:"lines"`
	LinePage   int                `json:"line_page"`
	PageSize   int                `json:"page_size"`
	Events     []MaintenanceEvent `json:"events,omitempty"`
	EventTotal int                `json:"event_total,omitempty"`
	EventPage  int                `json:"event_page,omitempty"`
}
type MaintenancePage struct {
	Items    []MaintenanceCycle `json:"items"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Homes    []RecordHome       `json:"homes"`
	Totals   MaintenanceTotals  `json:"totals"`
}
type AllocationRecord struct {
	ID               string `json:"id"`
	SourceID         string `json:"source_id"`
	ChargeID         string `json:"charge_id"`
	AmountPaise      int64  `json:"amount_paise"`
	Receipt          string `json:"receipt"`
	SourceKind       string `json:"source_kind"`
	Description      string `json:"description"`
	State            string `json:"state"`
	Reason           string `json:"reason,omitempty"`
	Actor            string `json:"actor,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	CorrectionReason string `json:"correction_reason,omitempty"`
	CorrectedBy      string `json:"corrected_by,omitempty"`
	CorrectedAt      int64  `json:"corrected_at"`
}
type StatementEntry struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	Date           string `json:"date"`
	AmountPaise    int64  `json:"amount_paise"`
	AllocatedPaise int64  `json:"allocated_paise"`
	RemainingPaise int64  `json:"remaining_paise"`
	Receipt        string `json:"receipt"`
	CycleID        string `json:"cycle_id"`
	DueDate        string `json:"due_date"`
}
type HomeStatement struct {
	FlatID           string             `json:"flat_id"`
	Home             string             `json:"home"`
	DebitPaise       int64              `json:"debit_paise"`
	CreditPaise      int64              `json:"credit_paise"`
	AllocatedPaise   int64              `json:"allocated_paise"`
	OutstandingPaise int64              `json:"outstanding_paise"`
	UnallocatedPaise int64              `json:"unallocated_paise"`
	OverduePaise     int64              `json:"overdue_paise"`
	Charges          []StatementEntry   `json:"charges"`
	Credits          []StatementEntry   `json:"credits"`
	Allocations      []AllocationRecord `json:"allocations"`
	ChargeTotal      int                `json:"charge_total"`
	CreditTotal      int                `json:"credit_total"`
	AllocationTotal  int                `json:"allocation_total"`
	ChargePage       int                `json:"charge_page"`
	CreditPage       int                `json:"credit_page"`
	AllocationPage   int                `json:"allocation_page"`
	PageSize         int                `json:"page_size"`
}

func validMaintenanceDate(value string) bool {
	date, err := time.Parse("2006-01-02", value)
	return err == nil && date.Format("2006-01-02") == value && value >= "1900-01-01" && value <= "2100-12-31"
}

func validateMaintenance(in MaintenanceInput) ([]int64, error) {
	if !in.Confirmed || !validText(in.Title, 5, 120) || !validText(in.SourceReference, 5, 300) || !paragraph(in.Note, 0, 2000) {
		return nil, invalid("Supply a title, approved source reference and confirmation before submitting.")
	}
	if !validMaintenanceDate(in.PeriodStart) || !validMaintenanceDate(in.PeriodEnd) || !validMaintenanceDate(in.DueDate) || in.PeriodEnd < in.PeriodStart || in.DueDate < in.PeriodStart {
		return nil, invalid("Use valid period dates and an explicit due date on or after the period starts.")
	}
	if len(in.Lines) < 1 || len(in.Lines) > 500 {
		return nil, invalid("Include between one and 500 participating homes.")
	}
	seen, amounts := map[string]bool{}, make([]int64, len(in.Lines))
	for i, line := range in.Lines {
		if line.FlatID == "" || len(line.FlatID) > 100 || seen[line.FlatID] {
			return nil, invalid("Include each participating home once with its supplied amount.")
		}
		seen[line.FlatID] = true
		amount, err := ParseAmount(line.Amount)
		if err != nil {
			return nil, err
		}
		amounts[i] = amount
	}
	return amounts, nil
}

func maintenanceEvent(ctx context.Context, tx *sql.Tx, id, actor, action, reason string, version int, at int64) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO maintenance_events(cycle_id,action,version,actor_id,reason,occurred_at) VALUES(?,?,?,?,?,?)", id, action, version, actor, reason, at)
	return err
}

func (s *Store) CreateMaintenanceCycle(ctx context.Context, token string, in MaintenanceInput) (string, error) {
	amounts, err := validateMaintenance(in)
	if err != nil {
		return "", err
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "MAINTENANCE_SUBMIT", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	id, now := randomToken(), time.Now().Unix()
	_, err = tx.ExecContext(ctx, `INSERT INTO maintenance_cycles(id,title,period_start,period_end,due_date,source_reference,note,state,version,submitted_by,submitted_at)
        VALUES(?,?,?,?,?,?,?,'PENDING',1,?,?)`, id, in.Title, in.PeriodStart, in.PeriodEnd, in.DueDate, in.SourceReference, in.Note, p.ID, now)
	if err != nil {
		return "", err
	}
	for i, line := range in.Lines {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM flats WHERE id=?)", line.FlatID).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return "", invalid("A selected home is unavailable. Reload the participant list.")
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO maintenance_lines(cycle_id,flat_id,amount_paise) VALUES(?,?,?)", id, line.FlatID, amounts[i]); err != nil {
			return "", err
		}
	}
	if err := maintenanceEvent(ctx, tx, id, p.ID, "SUBMITTED", in.SourceReference, 1, now); err != nil {
		return "", err
	}
	if err := appendAudit(ctx, tx, p.ID, "", "MAINTENANCE_SUBMITTED", in.SourceReference, map[string]any{}, map[string]any{"cycle_id": id, "participants": len(in.Lines)}); err != nil {
		return "", err
	}
	if err := saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) DecideMaintenanceCycle(ctx context.Context, token, id string, in MaintenanceAction) (string, error) {
	if !in.Confirmed || in.Version < 1 || !validText(in.Reason, 10, 300) || (in.Decision != "PUBLISHED" && in.Decision != "DECLINED" && in.Decision != "WITHDRAWN") {
		return "", invalid("Choose a valid decision and record a confirmed reason of 10–300 characters.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "MAINTENANCE_DECIDE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var author, state, title, date, start, end string
	var version int
	var submitted int64
	err = tx.QueryRowContext(ctx, "SELECT submitted_by,state,version,title,period_start,period_end,submitted_at FROM maintenance_cycles WHERE id=?", id).Scan(&author, &state, &version, &title, &start, &end, &submitted)
	if err != nil {
		return "", err
	}
	if (in.Decision == "WITHDRAWN" && p.ID != author) || (in.Decision != "WITHDRAWN" && p.ID == author) {
		return "", ErrForbidden
	}
	if state != "PENDING" || in.Version != version {
		return "", ErrConflict
	}
	now := time.Now().Unix()
	date = start
	if in.Decision == "PUBLISHED" {
		rows, err := tx.QueryContext(ctx, "SELECT flat_id,amount_paise FROM maintenance_lines WHERE cycle_id=? ORDER BY flat_id", id)
		if err != nil {
			return "", err
		}
		type line struct {
			home   string
			amount int64
		}
		lines := []line{}
		for rows.Next() {
			var value line
			if err := rows.Scan(&value.home, &value.amount); err != nil {
				rows.Close()
				return "", err
			}
			lines = append(lines, value)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
		for _, line := range lines {
			entryID := randomToken()
			// A given charge uses the verified immutable ledger. This does not
			// create a received-money receipt or copy private approval notes.
			_, err := tx.ExecContext(ctx, `INSERT INTO entries(id,flat_id,kind,amount_paise,entry_date,description,payer,method,reference,source_note,state,created_by,created_at,posted_by,posted_at)
                VALUES(?,?,'CHARGE',?,?,?,'','','',?,'POSTED',?,?,?,?)`, entryID, line.home, line.amount, date, "Maintenance · "+title, "Published maintenance period "+start+" to "+end, author, submitted, p.ID, now)
			if err != nil {
				return "", err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE maintenance_lines SET entry_id=? WHERE cycle_id=? AND flat_id=?", entryID, id, line.home); err != nil {
				return "", err
			}
			if err := appendAudit(ctx, tx, p.ID, line.home, "ENTRY_POSTED", "Separately reviewed supplied maintenance charge", map[string]any{"cycle_id": id}, map[string]any{"cycle_id": id, "entry_id": entryID, "amount_paise": line.amount}); err != nil {
				return "", err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "UPDATE maintenance_cycles SET state=?,version=version+1,decided_by=?,decided_at=?,decision_reason=? WHERE id=? AND version=?", in.Decision, p.ID, now, in.Reason, id, version); err != nil {
		return "", err
	}
	if err := maintenanceEvent(ctx, tx, id, p.ID, in.Decision, in.Reason, version+1, now); err != nil {
		return "", err
	}
	if err := appendAudit(ctx, tx, p.ID, "", "MAINTENANCE_"+in.Decision, in.Reason, map[string]any{"cycle_id": id, "state": state}, map[string]any{"cycle_id": id, "state": in.Decision}); err != nil {
		return "", err
	}
	if err := saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) AllocateCredit(ctx context.Context, token string, in CreditAllocationInput) (string, error) {
	amount, err := ParseAmount(in.Amount)
	if err != nil {
		return "", err
	}
	if !in.Confirmed || !validText(in.Reason, 5, 300) || in.SourceID == "" || in.ChargeID == "" || len(in.SourceID) > 100 || len(in.ChargeID) > 100 {
		return "", invalid("Select the confirmed source and charge, review the amount and give an allocation reason.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "ALLOCATE_CREDIT", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	source, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", in.SourceID))
	if err != nil {
		return "", err
	}
	charge, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", in.ChargeID))
	if err != nil {
		return "", err
	}
	if source.FlatID != charge.FlatID || source.State != "POSTED" || charge.State != "POSTED" || (source.Kind != "RECEIVED" && source.Kind != "OPENING_CREDIT") || (charge.Kind != "CHARGE" && charge.Kind != "OPENING_DEBIT") {
		return "", invalid("Allocate only confirmed unreversed credit and charges from the same home.")
	}
	var sourceUsed, chargeUsed int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(CASE WHEN source_id=? THEN amount_paise ELSE 0 END),0),COALESCE(SUM(CASE WHEN charge_id=? THEN amount_paise ELSE 0 END),0) FROM live_entry_allocations WHERE source_id=? OR charge_id=?", source.ID, charge.ID, source.ID, charge.ID).Scan(&sourceUsed, &chargeUsed); err != nil {
		return "", err
	}
	if amount > source.AmountPaise-sourceUsed || amount > charge.AmountPaise-chargeUsed {
		return "", fmt.Errorf("%w: available credit or charge balance changed; reload before allocating", ErrConflict)
	}
	id := randomToken()
	if _, err := tx.ExecContext(ctx, "INSERT INTO entry_allocations VALUES(?,?,?,?,?,?,?)", id, source.ID, charge.ID, amount, p.ID, in.Reason, time.Now().Unix()); err != nil {
		return "", err
	}
	if err := appendAudit(ctx, tx, p.ID, source.FlatID, "CREDIT_ALLOCATED", in.Reason, map[string]any{"source_id": source.ID, "charge_id": charge.ID}, map[string]any{"allocation_id": id, "amount_paise": amount}); err != nil {
		return "", err
	}
	if err := saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) ReverseAllocation(ctx context.Context, token, id string, in AllocationCorrection) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", invalid("Review the allocation correction and give a reason of 5–300 characters.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "REVERSE_ALLOCATION:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var home string
	err = tx.QueryRowContext(ctx, "SELECT e.flat_id FROM entry_allocations a JOIN entries e ON e.id=a.source_id WHERE a.id=?", id).Scan(&home)
	if err != nil {
		return "", err
	}
	var reversed bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM allocation_reversals WHERE allocation_id=?)", id).Scan(&reversed); err != nil {
		return "", err
	}
	if reversed {
		return "", ErrConflict
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO allocation_reversals VALUES(?,?,?,?)", id, p.ID, in.Reason, time.Now().Unix()); err != nil {
		return "", err
	}
	if err := appendAudit(ctx, tx, p.ID, home, "ALLOCATION_REVERSED", in.Reason, map[string]any{"allocation_id": id}, map[string]any{"allocation_id": id, "state": "CORRECTED"}); err != nil {
		return "", err
	}
	if err := saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func beginMaintenanceRead(ctx context.Context, s *Store, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && !p.CanReadRecords {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}

func financialHomes(ctx context.Context, tx *sql.Tx, p Principal) ([]RecordHome, error) {
	query := "SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id"
	args := []any{}
	if !p.CanReadAllRecords {
		query += ` WHERE EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = []any{p.ResidentID, today(), today()}
	}
	rows, err := tx.QueryContext(ctx, query+" ORDER BY b.code,f.floor,f.flat_number", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RecordHome{}
	for rows.Next() {
		var value RecordHome
		if err := rows.Scan(&value.ID, &value.Label); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}

func permittedFinancialHome(ctx context.Context, tx *sql.Tx, p Principal, id string) (string, error) {
	query := "SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?"
	args := []any{id}
	if !p.CanReadAllRecords {
		query += ` AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		args = append(args, p.ResidentID, today(), today())
	}
	var home string
	err := tx.QueryRowContext(ctx, query, args...).Scan(&home)
	return home, err
}

func boundedMaintenancePage(value int) bool { return value >= 1 && value <= 100000 }
func clampMaintenancePage(page, count, size int) int {
	last := (count + size - 1) / size
	if last < 1 {
		last = 1
	}
	if page > last {
		return last
	}
	return page
}
