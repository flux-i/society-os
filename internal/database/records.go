package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Every amount crosses the API as a decimal string and is stored as integer paise.
var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,7})(\.[0-9]{1,2})?$`)
var operationPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)

func ParseAmount(value string) (int64, error) {
	if !amountPattern.MatchString(value) {
		return 0, invalid("Enter a positive rupee amount with at most two decimal places.")
	}
	parts := strings.Split(value, ".")
	rupees, _ := strconv.ParseInt(parts[0], 10, 64)
	var paise int64
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		paise, _ = strconv.ParseInt(fraction, 10, 64)
	}
	amount := rupees*100 + paise
	if amount <= 0 || amount > 1000000000 {
		return 0, invalid("The amount must be above zero and at most ₹1,00,00,000.")
	}
	return amount, nil
}

type EntryInput struct {
	OperationKey string `json:"operation_key"`
	FlatID       string `json:"flat_id"`
	Kind         string `json:"kind"`
	Amount       string `json:"amount"`
	Date         string `json:"date"`
	Description  string `json:"description"`
	Payer        string `json:"payer"`
	Method       string `json:"method"`
	Reference    string `json:"reference"`
	SourceNote   string `json:"source_note"`
}
type EntryAction struct {
	OperationKey string `json:"operation_key"`
	Confirmed    bool   `json:"confirmed"`
	Reason       string `json:"reason"`
}
type ReceiptSnapshot struct {
	Number      string `json:"number"`
	Home        string `json:"home"`
	Payer       string `json:"payer"`
	AmountPaise int64  `json:"amount_paise"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Method      string `json:"method"`
	Reference   string `json:"reference"`
	SourceNote  string `json:"source_note"`
	Operator    string `json:"operator"`
	IssuedAt    int64  `json:"issued_at"`
}
type Entry struct {
	ID             string `json:"id"`
	FlatID         string `json:"flat_id"`
	Home           string `json:"home"`
	Kind           string `json:"kind"`
	AmountPaise    int64  `json:"amount_paise"`
	Date           string `json:"date"`
	Description    string `json:"description"`
	Payer          string `json:"payer"`
	Method         string `json:"method"`
	Reference      string `json:"reference"`
	SourceNote     string `json:"source_note"`
	State          string `json:"state"`
	CreatedBy      string `json:"created_by"`
	CreatedAt      int64  `json:"created_at"`
	PostedBy       string `json:"posted_by"`
	PostedAt       int64  `json:"posted_at"`
	ReversalReason string `json:"reversal_reason"`
	ReversedBy     string `json:"reversed_by"`
	ReversedAt     int64  `json:"reversed_at"`
	ReceiptID      string `json:"receipt_id"`
	ReceiptNumber  string `json:"receipt_number"`
	PDFState       string `json:"pdf_state"`
	FileHash       string `json:"-"`
}
type EntryPage struct {
	Items        []Entry      `json:"items"`
	Total        int          `json:"total"`
	Page         int          `json:"page"`
	PageSize     int          `json:"page_size"`
	DebitPaise   int64        `json:"debit_paise"`
	CreditPaise  int64        `json:"credit_paise"`
	BalancePaise int64        `json:"balance_paise"`
	Drafts       int          `json:"drafts"`
	Homes        []RecordHome `json:"homes"`
}
type RecordHome struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// An explicit, additional preview grant. Registry administration never implies treasury access.
func (s *Store) SeedDemoTreasury(ctx context.Context) error {
	if err := s.RequireDemo(ctx); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO role_grants(id,user_id,role,valid_from,valid_until,revoked_at,granted_by) VALUES ('demo-treasury-grant','demo-user-admin','TREASURER',?,?,NULL,'demo-user-admin')`, time.Now().Unix(), time.Now().Add(365*24*time.Hour).Unix())
	return err
}

func (s *Store) beginRecordWrite(ctx context.Context, token string) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, Principal{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token)); err != nil {
		tx.Rollback()
		return nil, Principal{}, err
	}
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err == nil && !p.CanManageRecords {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func replayOperation(ctx context.Context, tx *sql.Tx, p Principal, key, action string, input any) (string, string, error) {
	if !operationPattern.MatchString(key) {
		return "", "", invalid("A valid retry identity is required. Reopen this form.")
	}
	bytes, err := json.Marshal(input)
	if err != nil {
		return "", "", err
	}
	hash := TokenHash(action + string(bytes))
	var previous, result string
	err = tx.QueryRowContext(ctx, "SELECT request_hash,result_id FROM record_operations WHERE actor_id=? AND operation_key=?", p.ID, key).Scan(&previous, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return "", hash, nil
	}
	if err != nil {
		return "", "", err
	}
	if previous != hash {
		return "", "", fmt.Errorf("%w: this retry identity was used with different details", ErrConflict)
	}
	return result, hash, nil
}
func saveOperation(ctx context.Context, tx *sql.Tx, p Principal, key, hash, id string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO record_operations VALUES(?,?,?,?,?)", p.ID, key, hash, id, time.Now().Unix())
	return err
}

func (s *Store) CreateEntry(ctx context.Context, token string, in EntryInput) (string, error) {
	amount, err := ParseAmount(in.Amount)
	if err != nil {
		return "", err
	}
	if in.Kind != "CHARGE" && in.Kind != "OPENING_DEBIT" && in.Kind != "OPENING_CREDIT" && in.Kind != "RECEIVED" {
		return "", invalid("Choose a supported entry type.")
	}
	if !validText(in.SourceNote, 0, 300) {
		return "", invalid("Use an evidence note of at most 300 characters without control characters.")
	}
	if !validDate(in.Date) || !validText(in.Description, 5, 300) {
		return "", invalid("Enter a valid date and a description of 5–300 characters.")
	}
	if in.Kind == "RECEIVED" {
		if !validText(in.Payer, 2, 120) || (in.Method != "CASH" && in.Method != "BANK_TRANSFER" && in.Method != "CHEQUE" && in.Method != "UPI") || !validText(in.Reference, 0, 120) {
			return "", invalid("Enter who paid, how it was received and a valid reference.")
		}
		if in.Method != "CASH" && in.Reference == "" {
			return "", invalid("Add the reference for this received amount.")
		}
	} else if in.Payer != "" || in.Method != "" || in.Reference != "" {
		return "", invalid("Payer and payment details apply only to money already received.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "CREATE", in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	var found int
	if err = tx.QueryRowContext(ctx, "SELECT 1 FROM flats WHERE id=?", in.FlatID).Scan(&found); err != nil {
		return "", err
	}
	id := randomToken()
	_, err = tx.ExecContext(ctx, `INSERT INTO entries VALUES(?,?,?,?,?,?,?,?,?,?,'DRAFT',?,?,NULL,NULL)`, id, in.FlatID, in.Kind, amount, in.Date, in.Description, in.Payer, in.Method, in.Reference, in.SourceNote, p.ID, time.Now().Unix())
	if err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, in.FlatID, "ENTRY_DRAFTED", in.Description, map[string]any{}, map[string]any{"entry_id": id, "kind": in.Kind, "amount_paise": amount}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

const entrySelect = `SELECT e.id,e.flat_id,b.code||'-'||f.flat_number,e.kind,e.amount_paise,e.entry_date,e.description,e.payer,e.method,e.reference,e.source_note,
 CASE WHEN v.entry_id IS NOT NULL THEN 'REVERSED' WHEN d.entry_id IS NOT NULL THEN 'DISCARDED' ELSE e.state END,u.display_name,e.created_at,COALESCE(p.display_name,''),COALESCE(e.posted_at,0),
 COALESCE(v.reason,d.reason,''),COALESCE(vu.display_name,du.display_name,''),COALESCE(v.created_at,d.created_at,0),COALESCE(r.id,''),COALESCE(r.number,''),COALESCE(j.state,''),COALESCE(j.file_hash,'')
 FROM entries e JOIN flats f ON f.id=e.flat_id JOIN buildings b ON b.id=f.building_id JOIN users u ON u.id=e.created_by
 LEFT JOIN draft_discards d ON d.entry_id=e.id LEFT JOIN users du ON du.id=d.actor_id LEFT JOIN users p ON p.id=e.posted_by LEFT JOIN entry_reversals v ON v.entry_id=e.id LEFT JOIN users vu ON vu.id=v.actor_id
 LEFT JOIN receipts r ON r.entry_id=e.id LEFT JOIN receipt_jobs j ON j.receipt_id=r.id`

func scanEntry(row interface{ Scan(...any) error }) (Entry, error) {
	var e Entry
	err := row.Scan(&e.ID, &e.FlatID, &e.Home, &e.Kind, &e.AmountPaise, &e.Date, &e.Description, &e.Payer, &e.Method, &e.Reference, &e.SourceNote, &e.State, &e.CreatedBy, &e.CreatedAt, &e.PostedBy, &e.PostedAt, &e.ReversalReason, &e.ReversedBy, &e.ReversedAt, &e.ReceiptID, &e.ReceiptNumber, &e.PDFState, &e.FileHash)
	return e, err
}
func recordScope(p Principal) (string, []any) {
	if p.CanReadAllRecords {
		return "1=1", nil
	}
	return `e.state='POSTED' AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=e.flat_id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`, []any{p.ResidentID, today(), today()}
}
func (s *Store) EntryFor(ctx context.Context, token, id string) (Entry, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return Entry{}, err
	}
	if !p.CanReadRecords {
		return Entry{}, ErrForbidden
	}
	scope, args := recordScope(p)
	args = append(args, id)
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE "+scope+" AND e.id=?", args...))
	if err != nil {
		return e, err
	}
	return e, tx.Commit()
}
func (s *Store) EntriesFor(ctx context.Context, token, home, query, state string, receipts bool, page int) (EntryPage, error) {
	out := EntryPage{Items: []Entry{}, Homes: []RecordHome{}, Page: page, PageSize: 12}
	if page < 1 || page > 100000 || len(query) > 100 {
		return out, invalid("Choose a valid page and a shorter search.")
	}
	if state != "" && state != "DRAFT" && state != "POSTED" && state != "REVERSED" && state != "DISCARDED" {
		return out, invalid("Choose a valid entry status.")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return out, err
	}
	if !p.CanReadRecords {
		return out, ErrForbidden
	}
	homesQuery := `SELECT f.id,b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id`
	homesArgs := []any{}
	if !p.CanReadAllRecords {
		homesQuery += ` WHERE EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=f.id AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?))`
		homesArgs = []any{p.ResidentID, today(), today()}
	}
	rows, err := tx.QueryContext(ctx, homesQuery+" ORDER BY b.code,f.floor,f.flat_number", homesArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			rows.Close()
			return out, err
		}
		out.Homes = append(out.Homes, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	scope, args := recordScope(p)
	if home != "" {
		scope += " AND e.flat_id=?"
		args = append(args, home)
	}
	// Totals reflect the selected home scope, independent of search, page and receipt filters.
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN e.state='POSTED' AND v.entry_id IS NULL AND e.kind IN ('CHARGE','OPENING_DEBIT') THEN e.amount_paise ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN e.state='POSTED' AND v.entry_id IS NULL AND e.kind IN ('RECEIVED','OPENING_CREDIT') THEN e.amount_paise ELSE 0 END),0),
 COALESCE(SUM(e.state='DRAFT' AND d.entry_id IS NULL),0) FROM entries e LEFT JOIN draft_discards d ON d.entry_id=e.id LEFT JOIN entry_reversals v ON v.entry_id=e.id WHERE `+scope, args...).Scan(&out.DebitPaise, &out.CreditPaise, &out.Drafts)
	if err != nil {
		return out, err
	}
	out.BalancePaise = out.DebitPaise - out.CreditPaise
	if query != "" {
		scope += ` AND (instr(lower(e.description||' '||e.payer||' '||b.code||'-'||f.flat_number||' '||COALESCE(r.number,'')),lower(?))>0)`
		args = append(args, query)
	}
	if state != "" {
		if state == "DISCARDED" {
			scope += " AND d.entry_id IS NOT NULL"
		} else if state == "REVERSED" {
			scope += " AND v.entry_id IS NOT NULL"
		} else {
			scope += " AND e.state=? AND v.entry_id IS NULL AND d.entry_id IS NULL"
			args = append(args, state)
		}
	}
	if receipts {
		scope += " AND r.id IS NOT NULL"
	}
	from := strings.SplitN(entrySelect, " FROM ", 2)[1]
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+from+" WHERE "+scope, args...).Scan(&out.Total)
	if err != nil {
		return out, err
	}
	maxPage := (out.Total + 11) / 12
	if maxPage < 1 {
		maxPage = 1
	}
	if out.Page > maxPage {
		out.Page = maxPage
	}
	listArgs := append(append([]any{}, args...), 12, (out.Page-1)*12)
	rows, err = tx.QueryContext(ctx, entrySelect+" WHERE "+scope+" ORDER BY e.created_at DESC,e.rowid DESC LIMIT ? OFFSET ?", listArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		e, eErr := scanEntry(rows)
		if eErr != nil {
			rows.Close()
			return out, eErr
		}
		out.Items = append(out.Items, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) PostEntry(ctx context.Context, token, id string, in EntryAction) (string, error) {
	if !in.Confirmed || in.Reason != "" {
		return "", invalid("Review and confirm these supplied details before posting.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "POST:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", id))
	if err != nil {
		return "", err
	}
	if e.State != "DRAFT" {
		return "", ErrConflict
	}
	now := time.Now()
	_, err = tx.ExecContext(ctx, "UPDATE entries SET state='POSTED',posted_by=?,posted_at=? WHERE id=? AND state='DRAFT'", p.ID, now.Unix(), id)
	if err != nil {
		return "", err
	}
	if e.Kind == "RECEIVED" {
		year := now.In(time.FixedZone("IST", 19800)).Format("2006")
		var seq int
		err = tx.QueryRowContext(ctx, `INSERT INTO receipt_counter VALUES (?,1) ON CONFLICT(year) DO UPDATE SET next_number=next_number+1 RETURNING next_number`, year).Scan(&seq)
		if err != nil {
			return "", err
		}
		snapshot := ReceiptSnapshot{Number: fmt.Sprintf("SOS-%s-%06d", year, seq), Home: e.Home, Payer: e.Payer, AmountPaise: e.AmountPaise, Date: e.Date, Description: e.Description, Method: e.Method, Reference: e.Reference, SourceNote: e.SourceNote, Operator: p.Name, IssuedAt: now.Unix()}
		blob, err := json.Marshal(snapshot)
		if err != nil {
			return "", err
		}
		receiptID := randomToken()
		if _, err = tx.ExecContext(ctx, "INSERT INTO receipts VALUES(?,?,?,?,?)", receiptID, id, snapshot.Number, string(blob), now.Unix()); err != nil {
			return "", err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO receipt_jobs(receipt_id,state,available_at) VALUES(?,'PENDING',?)", receiptID, now.Unix()); err != nil {
			return "", err
		}
	}
	if err = appendAudit(ctx, tx, p.ID, e.FlatID, "ENTRY_POSTED", e.Description, map[string]any{"entry_id": id, "state": "DRAFT"}, map[string]any{"entry_id": id, "state": "POSTED", "amount_paise": e.AmountPaise}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) ReverseEntry(ctx context.Context, token, id string, in EntryAction) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", invalid("Confirm the correction and provide a reason of 5–300 characters.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "REVERSE:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", id))
	if err != nil {
		return "", err
	}
	if e.State != "POSTED" {
		return "", ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO entry_reversals VALUES(?,?,?,?)", id, in.Reason, p.ID, time.Now().Unix()); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, e.FlatID, "ENTRY_REVERSED", in.Reason, map[string]any{"entry_id": id, "state": "POSTED"}, map[string]any{"entry_id": id, "state": "REVERSED"}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func (s *Store) DiscardDraft(ctx context.Context, token, id string, in EntryAction) (string, error) {
	if !in.Confirmed || !validText(in.Reason, 5, 300) {
		return "", invalid("Confirm discarding this draft and provide a reason of 5–300 characters.")
	}
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, hash, err := replayOperation(ctx, tx, p, in.OperationKey, "DISCARD:"+id, in)
	if err != nil {
		return "", err
	}
	if result != "" {
		return result, tx.Commit()
	}
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE e.id=?", id))
	if err != nil {
		return "", err
	}
	if e.State != "DRAFT" {
		return "", ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO draft_discards VALUES(?,?,?,?)", id, in.Reason, p.ID, time.Now().Unix()); err != nil {
		return "", err
	}
	if err = appendAudit(ctx, tx, p.ID, e.FlatID, "ENTRY_DRAFT_DISCARDED", in.Reason, map[string]any{"entry_id": id, "state": "DRAFT"}, map[string]any{"entry_id": id, "state": "DISCARDED"}); err != nil {
		return "", err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, hash, id); err != nil {
		return "", err
	}
	return id, tx.Commit()
}

type ReceiptJob struct {
	ID       string
	Lease    string
	Snapshot ReceiptSnapshot
}

// A single atomic claim reserves the writer; stale workers cannot publish after a new lease.
func (s *Store) ClaimReceipt(ctx context.Context, now time.Time) (ReceiptJob, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ReceiptJob{}, err
	}
	defer tx.Rollback()
	var job ReceiptJob
	job.Lease = randomToken()
	err = tx.QueryRowContext(ctx, `UPDATE receipt_jobs SET state='RUNNING',attempts=attempts+1,lease_token=?,lease_until=?
 WHERE receipt_id=(SELECT receipt_id FROM receipt_jobs WHERE (state='PENDING' AND available_at<=?) OR (state='RUNNING' AND lease_until<=?) ORDER BY available_at,receipt_id LIMIT 1) RETURNING receipt_id`, job.Lease, now.Add(time.Minute).Unix(), now.Unix(), now.Unix()).Scan(&job.ID)
	if err != nil {
		return job, err
	}
	var blob string
	if err = tx.QueryRowContext(ctx, "SELECT snapshot_json FROM receipts WHERE id=?", job.ID).Scan(&blob); err != nil {
		return job, err
	}
	if err = json.Unmarshal([]byte(blob), &job.Snapshot); err != nil {
		return job, err
	}
	return job, tx.Commit()
}
func (s *Store) FinishReceipt(ctx context.Context, job ReceiptJob, hash string, success bool, now time.Time) error {
	var result sql.Result
	var err error
	if success {
		result, err = s.DB.ExecContext(ctx, "UPDATE receipt_jobs SET state='READY',file_hash=?,lease_token='',lease_until=0 WHERE receipt_id=? AND state='RUNNING' AND lease_token=? AND lease_until>?", hash, job.ID, job.Lease, now.Unix())
	} else {
		result, err = s.DB.ExecContext(ctx, `UPDATE receipt_jobs SET state=CASE WHEN attempts>=5 THEN 'FAILED' ELSE 'PENDING' END,available_at=?,lease_token='',lease_until=0 WHERE receipt_id=? AND state='RUNNING' AND lease_token=? AND lease_until>?`, now.Add(5*time.Second).Unix(), job.ID, job.Lease, now.Unix())
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (s *Store) RetryReceipt(ctx context.Context, token, id string) error {
	tx, p, err := s.beginRecordWrite(ctx, token)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE r.id=?", id))
	if err != nil {
		return err
	}
	if e.PDFState != "FAILED" {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, "UPDATE receipt_jobs SET state='PENDING',attempts=0,available_at=? WHERE receipt_id=? AND state='FAILED'", time.Now().Unix(), id); err != nil {
		return err
	}
	if err = appendAudit(ctx, tx, p.ID, e.FlatID, "RECEIPT_RETRY", "Retry receipt document", map[string]any{}, map[string]any{"receipt_id": id}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) ReceiptEntryFor(ctx context.Context, token, id string) (Entry, error) {
	// Resolve the opaque identity, then recheck current scope in the same read snapshot.
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Entry{}, err
	}
	defer tx.Rollback()
	p, err := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if err != nil {
		return Entry{}, err
	}
	if !p.CanReadRecords {
		return Entry{}, ErrForbidden
	}
	scope, args := recordScope(p)
	args = append(args, id)
	e, err := scanEntry(tx.QueryRowContext(ctx, entrySelect+" WHERE "+scope+" AND r.id=?", args...))
	if err != nil {
		return e, err
	}
	return e, tx.Commit()
}
