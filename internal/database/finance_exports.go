package database

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var financeHashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type FinanceExportFilter struct {
	Report string `json:"report"`
	Scope  string `json:"scope"`
	HomeID string `json:"home_id"`
	FundID string `json:"fund_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}
type FinanceExportInput struct {
	FinanceExportFilter
	OperationKey string `json:"operation_key"`
	PreviewHash  string `json:"preview_hash"`
	Confirmed    bool   `json:"confirmed"`
}

// JSON paise are decimal strings too: a whole-society total must not become an
// imprecise JavaScript Number before it reaches the confirmation screen.
type FinanceExportSummary struct {
	CurrentNet        int64 `json:"current_net_paise,string"`
	CurrentReceived   int64 `json:"current_received_paise,string"`
	AvailableReceived int64 `json:"available_received_paise,string"`
	OpeningCredit     int64 `json:"opening_credit_paise,string"`
	OriginalReceived  int64 `json:"original_received_paise,string"`
	ReversedReceived  int64 `json:"reversed_received_paise,string"`
	UsableReceived    int64 `json:"usable_received_paise,string"`
	Requested         int64 `json:"requested_paise,string"`
	Active            int64 `json:"active_paise,string"`
	Allocated         int64 `json:"allocated_paise,string"`
	Outstanding       int64 `json:"outstanding_paise,string"`
	Waived            int64 `json:"waived_paise,string"`
	Voluntary         int64 `json:"voluntary_paise,string"`
	PendingReports    int   `json:"pending_reports"`
	Sources           int   `json:"sources"`
}
type FinanceExport struct {
	ID string `json:"id"`
	FinanceExportFilter
	Authority   string               `json:"authority"`
	Homes       []RecordHome         `json:"homes"`
	ScopeLabel  string               `json:"scope_label"`
	DateBasis   string               `json:"date_basis"`
	Summary     FinanceExportSummary `json:"summary"`
	ContentHash string               `json:"content_hash"`
	SHA256      string               `json:"sha256"`
	Rows        int                  `json:"rows"`
	Bytes       int                  `json:"bytes"`
	GeneratedAt string               `json:"generated_at"`
}
type FinanceExportChoices struct {
	Society  bool         `json:"society"`
	Fresh    bool         `json:"fresh"`
	Homes    []RecordHome `json:"homes"`
	OwnHomes []RecordHome `json:"own_homes"`
	Funds    []RecordHome `json:"funds"`
}
type FinanceExportPage struct {
	Items    []FinanceExport `json:"items"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}
type financeExportLimits struct {
	Rows, Bytes              int
	ActorBytes, SocietyBytes int64
}

func (s *Store) financeLimits() financeExportLimits {
	out := financeExportLimits{10000, 5 * 1024 * 1024, 100 * 1024 * 1024, 1024 * 1024 * 1024}
	// Private lower-only bounds let domain tests exercise quota rollback and races
	// without allocating a gigabyte. No runtime option can increase these limits.
	if x := s.exportLimits; x != nil {
		if x.Rows > 0 && x.Rows < out.Rows {
			out.Rows = x.Rows
		}
		if x.Bytes > 0 && x.Bytes < out.Bytes {
			out.Bytes = x.Bytes
		}
		if x.ActorBytes > 0 && x.ActorBytes < out.ActorBytes {
			out.ActorBytes = x.ActorBytes
		}
		if x.SocietyBytes > 0 && x.SocietyBytes < out.SocietyBytes {
			out.SocietyBytes = x.SocietyBytes
		}
	}
	return out
}
func validateFinanceFilter(in FinanceExportFilter) error {
	if !validDate(in.From) || !validDate(in.To) || in.From > in.To {
		return invalid("Choose a valid source-date range, with the end on or after the start.")
	}
	if in.Report != "LEDGER" && in.Report != "RECEIPTS" && in.Report != "MAINTENANCE" && in.Report != "FUNDS" {
		return invalid("Choose one of the four financial reports.")
	}
	if in.Scope != "SOCIETY" && in.Scope != "OWN" && in.Scope != "HOME" {
		return invalid("Choose the society, your financial homes or one permitted home.")
	}
	if len(in.HomeID) > 100 || len(in.FundID) > 100 || (in.Scope == "HOME") != (in.HomeID != "") || (in.Report != "FUNDS" && in.FundID != "") {
		return ErrInvalid
	}
	return nil
}
func financeDateBasis(report string) string {
	switch report {
	case "RECEIPTS":
		return "Original receipt date"
	case "MAINTENANCE":
		return "Maintenance period start"
	case "FUNDS":
		return "Fund start date"
	default:
		return "Ledger entry date"
	}
}
func (s *Store) beginFinanceExport(ctx context.Context, token string, write bool) (*sql.Tx, Principal, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: !write})
	if err != nil {
		return nil, Principal{}, err
	}
	if write {
		_, err = tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at=last_seen_at WHERE token_hash=?", TokenHash(token))
	}
	var p Principal
	if err == nil {
		p, err = fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	}
	if err == nil && !p.CanExportFinance {
		err = ErrForbidden
	}
	if err != nil {
		tx.Rollback()
		return nil, p, err
	}
	return tx, p, nil
}
func personalFinanceHomes(ctx context.Context, tx *sql.Tx, p Principal) ([]RecordHome, error) {
	personal := p
	personal.CanReadAllRecords = false
	return financialHomes(ctx, tx, personal)
}
func exportHomesJSON(homes []RecordHome) string {
	ids := make([]string, len(homes))
	for i, h := range homes {
		ids[i] = h.ID
	}
	body, _ := json.Marshal(ids)
	return string(body)
}
func resolveFinanceScope(ctx context.Context, tx *sql.Tx, p Principal, in FinanceExportFilter) (FinanceExport, error) {
	out := FinanceExport{FinanceExportFilter: in, DateBasis: financeDateBasis(in.Report)}
	var err error
	if in.Scope == "SOCIETY" || (in.Scope == "HOME" && p.CanManageRecords) {
		if !p.CanManageRecords {
			return out, ErrForbidden
		}
		if !p.Fresh {
			return out, ErrReauthRequired
		}
		out.Authority = "TREASURY"
		out.Homes, err = financialHomes(ctx, tx, p)
	} else {
		out.Authority = "PERSONAL"
		out.Homes, err = personalFinanceHomes(ctx, tx, p)
	}
	if err != nil {
		return out, err
	}
	if in.Scope == "HOME" {
		selected := []RecordHome{}
		for _, h := range out.Homes {
			if h.ID == in.HomeID {
				selected = append(selected, h)
			}
		}
		out.Homes = selected
	}
	if len(out.Homes) == 0 {
		return out, ErrForbidden
	}
	if in.Scope == "HOME" {
		out.ScopeLabel = "Home " + out.Homes[0].Label
	} else if in.Scope == "SOCIETY" {
		out.ScopeLabel = "Whole society"
	} else {
		out.ScopeLabel = "Your financial homes"
	}
	if in.FundID != "" {
		var yes bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id
   WHERE c.id=? AND c.state IN ('PUBLISHED','CLOSED') AND l.flat_id IN (SELECT value FROM json_each(?)))`, in.FundID, exportHomesJSON(out.Homes)).Scan(&yes)
		if err != nil {
			return out, err
		}
		if !yes {
			return out, sql.ErrNoRows
		}
	}
	return out, nil
}
func currentFinanceExportAccess(ctx context.Context, tx *sql.Tx, p Principal, out FinanceExport) error {
	if out.Authority == "TREASURY" {
		if !p.CanManageRecords {
			return ErrForbidden
		}
		if !p.Fresh {
			return ErrReauthRequired
		}
		return nil
	}
	homes, err := personalFinanceHomes(ctx, tx, p)
	if err != nil {
		return err
	}
	accessible := map[string]bool{}
	for _, h := range homes {
		accessible[h.ID] = true
	}
	for _, h := range out.Homes {
		if !accessible[h.ID] {
			return ErrForbidden
		}
	}
	if len(out.Homes) == 0 {
		return ErrForbidden
	}
	return nil
}
func (s *Store) FinanceExportChoicesFor(ctx context.Context, token string) (FinanceExportChoices, error) {
	out := FinanceExportChoices{Homes: []RecordHome{}, OwnHomes: []RecordHome{}, Funds: []RecordHome{}}
	tx, p, err := s.beginFinanceExport(ctx, token, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	out.Society = p.CanManageRecords
	out.Fresh = p.Fresh
	out.OwnHomes, err = personalFinanceHomes(ctx, tx, p)
	if err != nil {
		return out, err
	}
	out.Homes = out.OwnHomes
	if out.Society {
		out.Homes, err = financialHomes(ctx, tx, p)
		if err != nil {
			return out, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT c.id,c.title FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id WHERE c.state IN ('PUBLISHED','CLOSED')
  AND l.flat_id IN (SELECT value FROM json_each(?)) ORDER BY c.start_date DESC,c.id LIMIT 1001`, exportHomesJSON(out.Homes))
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var h RecordHome
		if err = rows.Scan(&h.ID, &h.Label); err != nil {
			break
		}
		out.Funds = append(out.Funds, h)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Funds) > 1000 {
		return out, invalid("More than 1,000 published funds need a separately scoped chooser before export.")
	}
	return out, tx.Commit()
}
func (s *Store) PreviewFinanceExport(ctx context.Context, token string, in FinanceExportFilter) (FinanceExport, error) {
	if err := validateFinanceFilter(in); err != nil {
		return FinanceExport{}, err
	}
	tx, p, err := s.beginFinanceExport(ctx, token, false)
	if err != nil {
		return FinanceExport{}, err
	}
	defer tx.Rollback()
	out, err := resolveFinanceScope(ctx, tx, p, in)
	if err != nil {
		return out, err
	}
	_, err = s.buildFinanceExport(ctx, tx, p, &out, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) CreateFinanceExport(ctx context.Context, token string, in FinanceExportInput) (FinanceExport, error) {
	if err := validateFinanceFilter(in.FinanceExportFilter); err != nil {
		return FinanceExport{}, err
	}
	if !in.Confirmed || !financeHashPattern.MatchString(in.PreviewHash) {
		return FinanceExport{}, invalid("Review the current preview and confirm the export scope first.")
	}
	tx, p, err := s.beginFinanceExport(ctx, token, true)
	if err != nil {
		return FinanceExport{}, err
	}
	defer tx.Rollback()
	previous, requestHash, err := replayOperation(ctx, tx, p, in.OperationKey, "FINANCE_EXPORT", in)
	if err != nil {
		return FinanceExport{}, err
	}
	if previous != "" {
		out, err := financeExportFor(ctx, tx, p, previous)
		if err != nil {
			return out, err
		}
		return out, tx.Commit()
	}
	out, err := resolveFinanceScope(ctx, tx, p, in.FinanceExportFilter)
	if err != nil {
		return out, err
	}
	body, err := s.buildFinanceExport(ctx, tx, p, &out, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return out, err
	}
	if in.PreviewHash != out.ContentHash {
		return out, fmt.Errorf("%w: these records or the selected homes changed; reload the preview before creating a snapshot", ErrConflict)
	}
	var actorBytes, totalBytes int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN actor_id=? THEN byte_length ELSE 0 END),0),COALESCE(SUM(byte_length),0) FROM finance_exports`, p.ID).Scan(&actorBytes, &totalBytes)
	if err != nil {
		return out, err
	}
	limits := s.financeLimits()
	if actorBytes+int64(len(body)) > limits.ActorBytes || totalBytes+int64(len(body)) > limits.SocietyBytes {
		return out, invalid("Stored export snapshots have reached their allowance. Ask the society officer to review the adopted retention policy.")
	}
	out.ID = randomToken()
	filterJSON, _ := json.Marshal(in.FinanceExportFilter)
	homesJSON, _ := json.Marshal(out.Homes)
	summaryJSON, _ := json.Marshal(out.Summary)
	_, err = tx.ExecContext(ctx, `INSERT INTO finance_exports VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, out.ID, p.ID, string(filterJSON), out.Authority, string(homesJSON), out.ScopeLabel, string(summaryJSON), out.ContentHash, out.SHA256, out.Rows, out.Bytes, body, out.GeneratedAt)
	if err != nil {
		return out, err
	}
	// Deliberately omit descriptions, payer names, references and CSV bytes.
	audit := struct {
		ID                 string `json:"export_id"`
		Report             string `json:"report"`
		Scope              string `json:"scope"`
		Authority          string `json:"authority"`
		From               string `json:"from"`
		To                 string `json:"to"`
		Homes, Rows, Bytes int
	}{out.ID, out.Report, out.Scope, out.Authority, out.From, out.To, len(out.Homes), out.Rows, out.Bytes}
	if err = appendAudit(ctx, tx, p.ID, "", "FINANCE_EXPORTED", "Confirmed the current financial scope and snapshot", "", audit); err != nil {
		return out, err
	}
	if err = saveOperation(ctx, tx, p, in.OperationKey, requestHash, out.ID); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func scanFinanceExport(row interface{ Scan(...any) error }) (FinanceExport, error) {
	var out FinanceExport
	var filter, homes, summary string
	err := row.Scan(&out.ID, &filter, &out.Authority, &homes, &out.ScopeLabel, &summary, &out.ContentHash, &out.SHA256, &out.Rows, &out.Bytes, &out.GeneratedAt)
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(filter), &out.FinanceExportFilter); err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(homes), &out.Homes); err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(summary), &out.Summary); err != nil {
		return out, err
	}
	out.DateBasis = financeDateBasis(out.Report)
	return out, nil
}

const financeExportSelect = `SELECT id,filter_json,authority,homes_json,scope_label,summary_json,content_hash,sha256,row_count,byte_length,generated_at FROM finance_exports `

func financeExportFor(ctx context.Context, tx *sql.Tx, p Principal, id string) (FinanceExport, error) {
	if id == "" || len(id) > 100 {
		return FinanceExport{}, ErrInvalid
	}
	out, err := scanFinanceExport(tx.QueryRowContext(ctx, financeExportSelect+"WHERE id=? AND actor_id=?", id, p.ID))
	if err != nil {
		return out, err
	}
	return out, currentFinanceExportAccess(ctx, tx, p, out)
}
func (s *Store) FinanceExportFor(ctx context.Context, token, id string) (FinanceExport, error) {
	tx, p, err := s.beginFinanceExport(ctx, token, false)
	if err != nil {
		return FinanceExport{}, err
	}
	defer tx.Rollback()
	out, err := financeExportFor(ctx, tx, p, id)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) DownloadFinanceExport(ctx context.Context, token, id string) (FinanceExport, []byte, error) {
	tx, p, err := s.beginFinanceExport(ctx, token, false)
	if err != nil {
		return FinanceExport{}, nil, err
	}
	defer tx.Rollback()
	out, err := financeExportFor(ctx, tx, p, id)
	if err != nil {
		return out, nil, err
	}
	var body []byte
	err = tx.QueryRowContext(ctx, "SELECT csv_bytes FROM finance_exports WHERE id=? AND actor_id=?", id, p.ID).Scan(&body)
	if err != nil {
		return out, nil, err
	}
	digest := sha256.Sum256(body)
	if len(body) != out.Bytes || hex.EncodeToString(digest[:]) != out.SHA256 {
		return out, nil, errors.New("stored finance export checksum mismatch")
	}
	return out, body, tx.Commit()
}
func (s *Store) FinanceExportsFor(ctx context.Context, token string, page int) (FinanceExportPage, error) {
	out := FinanceExportPage{Items: []FinanceExport{}, PageSize: 12}
	if !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, err := s.beginFinanceExport(ctx, token, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	// Count and pagination use current scope in SQL, including every frozen home.
	where := `WHERE actor_id=? AND ((authority='TREASURY' AND ?=1 AND ?=1) OR (authority='PERSONAL' AND NOT EXISTS(
  SELECT 1 FROM json_each(homes_json) h WHERE NOT EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=json_extract(h.value,'$.id') AND m.resident_id=? AND m.can_view_finances=1 AND m.start_date<=? AND (m.end_date IS NULL OR m.end_date>?)))))`
	args := []any{p.ID, p.CanManageRecords, p.Fresh, p.ResidentID, today(), today()}
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM finance_exports "+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	rows, err := tx.QueryContext(ctx, financeExportSelect+where+" ORDER BY generated_at DESC,id LIMIT 12 OFFSET ?", append(args, (out.Page-1)*12)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		x, e := scanFinanceExport(rows)
		if e != nil {
			err = e
			break
		}
		out.Items = append(out.Items, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

func exactExportMoney(paise int64) string {
	negative := ""
	if paise < 0 {
		negative = "-"
		paise = -paise
	}
	return negative + strconv.FormatInt(paise/100, 10) + fmt.Sprintf(".%02d", paise%100)
}
func safeFinanceCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf) })
	if trimmed != "" && strings.ContainsRune("=+-@＝＋－＠", []rune(trimmed)[0]) {
		return "[text] " + value
	}
	return value
}

type boundedExportBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedExportBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, invalid("This export exceeds 5 MiB. Choose a smaller scope or date range.")
	}
	return b.Buffer.Write(p)
}
