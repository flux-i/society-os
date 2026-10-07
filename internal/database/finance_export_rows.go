package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

// All four reports have the same stable header. A blank numeric cell means the
// field is inapplicable to that row type; controlled numbers are integer paise
// until the final CSV serialization. Originals are never replaced by totals.
var financeExportColumns = []string{"report", "snapshot_utc", "scope", "from", "to", "date_basis", "row_type", "home", "source_id", "entry_id", "original_entry_id", "linked_id", "receipt_number", "source_date", "recorded_utc", "title", "state", "kind", "description", "payer", "method", "reference", "original_rupees", "signed_rupees", "active_rupees", "allocated_rupees", "outstanding_rupees", "requested_rupees", "waived_rupees", "voluntary_rupees", "pending_reports", "current_net_rupees", "current_received_rupees", "available_received_rupees", "opening_credit_rupees", "selected_original_received_rupees", "selected_reversed_received_rupees", "selected_usable_received_rupees", "selected_outstanding_rupees"}

type financeExportRow struct {
	Type, Home, Source, Entry, OriginalEntry, Link, Receipt, Date, Recorded, Title, State, Kind, Description, Payer, Method, Reference string
	Original, Signed, Active, Allocated, Outstanding, Requested, Waived, Voluntary                                                     *int64
	Pending                                                                                                                            *int
}

func (row financeExportRow) cells(out FinanceExport, stamp string) []string {
	columns := []string{out.Report, stamp, out.ScopeLabel, out.From, out.To, out.DateBasis, row.Type, row.Home, row.Source, row.Entry, row.OriginalEntry, row.Link, row.Receipt, row.Date, row.Recorded, row.Title, row.State, row.Kind, row.Description, row.Payer, row.Method, row.Reference}
	for i := range columns {
		columns[i] = safeFinanceCell(columns[i])
	}
	for _, amount := range []*int64{row.Original, row.Signed, row.Active, row.Allocated, row.Outstanding, row.Requested, row.Waived, row.Voluntary} {
		value := ""
		if amount != nil {
			value = exactExportMoney(*amount)
		}
		columns = append(columns, value)
	}
	pending := ""
	if row.Pending != nil {
		pending = strconv.Itoa(*row.Pending)
	}
	columns = append(columns, pending)
	for _, amount := range []int64{out.Summary.CurrentNet, out.Summary.CurrentReceived, out.Summary.AvailableReceived, out.Summary.OpeningCredit, out.Summary.OriginalReceived, out.Summary.ReversedReceived, out.Summary.UsableReceived, out.Summary.Outstanding} {
		value := ""
		if row.Type == "SCOPE" {
			value = exactExportMoney(amount)
		}
		columns = append(columns, value)
	}
	return columns
}

const exportSourceCTE = `WITH export_homes AS (SELECT value id FROM json_each(:homes)),
 used_credit AS (SELECT source_id,SUM(amount_paise) amount FROM live_credit_uses GROUP BY source_id),
 used_charge AS (SELECT charge_id,SUM(amount_paise) amount FROM live_entry_allocations GROUP BY charge_id),
 selected_entries AS (SELECT e.* FROM entries e WHERE e.state='POSTED' AND e.flat_id IN (SELECT id FROM export_homes) AND e.entry_date BETWEEN :from AND :to),
 selected_maintenance AS (SELECT c.id,c.title,c.period_start,l.flat_id,l.entry_id,l.amount_paise FROM maintenance_cycles c JOIN maintenance_lines l ON l.cycle_id=c.id
 WHERE c.state='PUBLISHED' AND l.flat_id IN (SELECT id FROM export_homes) AND c.period_start BETWEEN :from AND :to),
 selected_funds AS (SELECT c.id,c.title,c.start_date,c.contribution_type,c.state,l.flat_id,l.requested_paise,l.waived_paise,COALESCE(l.original_entry_id,'') original_entry_id,COALESCE(l.current_entry_id,'') current_entry_id
 FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id WHERE c.state IN ('PUBLISHED','CLOSED') AND l.flat_id IN (SELECT id FROM export_homes) AND c.start_date BETWEEN :from AND :to AND (:fund='' OR c.id=:fund)),
 selected_fund_charges AS (SELECT id campaign_id,flat_id,start_date source_date,original_entry_id entry_id FROM selected_funds WHERE original_entry_id!=''
 UNION SELECT id,flat_id,start_date,current_entry_id FROM selected_funds WHERE current_entry_id!=''
 UNION SELECT w.campaign_id,w.flat_id,f.start_date,w.replacement_entry_id FROM fund_waivers w JOIN selected_funds f ON f.id=w.campaign_id AND f.flat_id=w.flat_id WHERE w.state='APPROVED' AND w.replacement_entry_id IS NOT NULL)
 `
const exportHomeLabel = `(SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=e.flat_id)`
const exportLineLabel = `(SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=l.flat_id)`
const exportActiveEntry = `CASE WHEN v.entry_id IS NULL THEN e.amount_paise ELSE 0 END`
const exportEntryUsed = `CASE WHEN v.entry_id IS NOT NULL THEN 0 WHEN e.kind IN ('RECEIVED','OPENING_CREDIT') THEN COALESCE(uc.amount,0) ELSE COALESCE(ua.amount,0) END`
const exportEntryRemaining = `CASE WHEN v.entry_id IS NULL THEN e.amount_paise-CASE WHEN e.kind IN ('RECEIVED','OPENING_CREDIT') THEN COALESCE(uc.amount,0) ELSE COALESCE(ua.amount,0) END ELSE 0 END`

func financePrimarySQL(report string) string {
	switch report {
	case "LEDGER", "RECEIPTS":
		rowtype := "ENTRY"
		source := "e.id"
		extra := ""
		if report == "RECEIPTS" {
			rowtype = "RECEIPT"
			source = "r.id"
			extra = " WHERE r.id IS NOT NULL"
		}
		return `SELECT json_object('Type','` + rowtype + `','Home',` + exportHomeLabel + `,'Source',` + source + `,'Entry',e.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,
   'Title',e.description,'State',CASE WHEN v.entry_id IS NULL THEN 'POSTED' ELSE 'REVERSED' END,'Kind',e.kind,'Description',e.description,'Payer',e.payer,'Method',e.method,'Reference',e.reference,
   'Original',e.amount_paise,'Signed',CASE WHEN e.kind IN ('CHARGE','OPENING_DEBIT') THEN e.amount_paise ELSE -e.amount_paise END,
   'Active',` + exportActiveEntry + `,'Allocated',` + exportEntryUsed + `,'Outstanding',` + exportEntryRemaining + `) row_json
   FROM selected_entries e LEFT JOIN receipts r ON r.entry_id=e.id LEFT JOIN entry_reversals v ON v.entry_id=e.id LEFT JOIN used_credit uc ON uc.source_id=e.id LEFT JOIN used_charge ua ON ua.charge_id=e.id` + extra
	case "MAINTENANCE":
		return `SELECT json_object('Type','MAINTENANCE','Home',` + exportLineLabel + `,'Source',l.id,'Entry',l.entry_id,'OriginalEntry',l.entry_id,'Date',l.period_start,'Title',l.title,
   'State',CASE WHEN v.entry_id IS NULL THEN 'POSTED' ELSE 'REVERSED' END,'Kind','CHARGE','Original',l.amount_paise,'Requested',l.amount_paise,
   'Active',CASE WHEN v.entry_id IS NULL THEN l.amount_paise ELSE 0 END,'Allocated',CASE WHEN v.entry_id IS NULL THEN COALESCE(a.amount,0) ELSE 0 END,
   'Outstanding',CASE WHEN v.entry_id IS NULL THEN l.amount_paise-COALESCE(a.amount,0) ELSE 0 END) row_json FROM selected_maintenance l
   LEFT JOIN entry_reversals v ON v.entry_id=l.entry_id LEFT JOIN used_charge a ON a.charge_id=l.entry_id`
	default:
		return `SELECT json_object('Type','FUND','Home',` + exportLineLabel + `,'Source',l.id,'Entry',l.current_entry_id,'OriginalEntry',l.original_entry_id,'Date',l.start_date,'Title',l.title,
   'State',l.state,'Kind',l.contribution_type,'Original',l.requested_paise,'Requested',l.requested_paise,'Waived',l.waived_paise,
   'Active',CASE WHEN e.state='POSTED' AND v.entry_id IS NULL THEN e.amount_paise ELSE 0 END,
   'Allocated',CASE WHEN e.state='POSTED' AND v.entry_id IS NULL THEN COALESCE(a.amount,0) ELSE 0 END,
   'Outstanding',CASE WHEN e.state='POSTED' AND v.entry_id IS NULL THEN e.amount_paise-COALESCE(a.amount,0) ELSE 0 END,
   'Voluntary',COALESCE((SELECT SUM(amount_paise) FROM live_fund_contributions z WHERE z.campaign_id=l.id AND z.flat_id=l.flat_id),0),
   'Pending',(SELECT COUNT(*) FROM fund_reports z WHERE z.campaign_id=l.id AND z.flat_id=l.flat_id AND z.state IN ('PENDING','NEEDS_INFO') AND (:all=1 OR z.author_id=:actor))) row_json
   FROM selected_funds l LEFT JOIN entries e ON e.id=l.current_entry_id LEFT JOIN entry_reversals v ON v.entry_id=e.id LEFT JOIN used_charge a ON a.charge_id=e.id`
	}
}

// A selected source retains later corrections. Dates below are the selected
// source dates; Recorded carries the actual later action time without reasons
// or officer identities. EXISTS avoids duplicate allocations when both linked
// entries occur in the selected ledger range.
func financeLinkPredicate(report, entry string) string {
	switch report {
	case "LEDGER":
		return `EXISTS(SELECT 1 FROM selected_entries z WHERE z.id=` + entry + `)`
	case "RECEIPTS":
		return `EXISTS(SELECT 1 FROM selected_entries z JOIN receipts rr ON rr.entry_id=z.id WHERE z.id=` + entry + `)`
	case "MAINTENANCE":
		return `EXISTS(SELECT 1 FROM selected_maintenance z WHERE z.entry_id=` + entry + `)`
	default:
		return `EXISTS(SELECT 1 FROM selected_fund_charges z WHERE z.entry_id=` + entry + `)`
	}
}
func financeLinksSQL(report string) []string {
	selected := financeLinkPredicate(report, "e.id")
	correction := `SELECT json_object('Type','ENTRY_REVERSAL','Home',` + exportHomeLabel + `,'Source',e.id,'Entry',e.id,'Link',e.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,
  'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',v.created_at,'unixepoch'),'State','REVERSED','Kind',e.kind,'Original',e.amount_paise,'Signed',CASE WHEN e.kind IN ('CHARGE','OPENING_DEBIT') THEN -e.amount_paise ELSE e.amount_paise END) row_json
  FROM entry_reversals v JOIN entries e ON e.id=v.entry_id LEFT JOIN receipts r ON r.entry_id=e.id WHERE ` + selected
	out := []string{correction}
	if report == "RECEIPTS" {
		return out
	}
	linked := financeLinkPredicate(report, "a.charge_id")
	if report == "LEDGER" {
		linked = "(" + linked + " OR " + financeLinkPredicate(report, "a.source_id") + ")"
	}
	base := ` FROM entry_allocations a JOIN entries e ON e.id=a.source_id LEFT JOIN allocation_reversals v ON v.allocation_id=a.id LEFT JOIN receipts r ON r.entry_id=e.id
  WHERE e.flat_id IN (SELECT id FROM export_homes) AND ` + linked
	out = append(out, `SELECT json_object('Type','ALLOCATION','Home',`+exportHomeLabel+`,'Source',a.source_id,'Entry',a.charge_id,'Link',a.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,
  'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',a.created_at,'unixepoch'),'State',CASE WHEN v.allocation_id IS NOT NULL THEN 'CORRECTED' WHEN NOT EXISTS(SELECT 1 FROM live_entry_allocations x WHERE x.id=a.id) THEN 'SOURCE_REVERSED' ELSE 'ACTIVE' END,
  'Kind',e.kind,'Original',a.amount_paise,'Active',CASE WHEN EXISTS(SELECT 1 FROM live_entry_allocations x WHERE x.id=a.id) THEN a.amount_paise ELSE 0 END) row_json`+base)
	out = append(out, `SELECT json_object('Type','ALLOCATION_REVERSAL','Home',`+exportHomeLabel+`,'Source',a.source_id,'Entry',a.charge_id,'Link',a.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,
  'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',v.created_at,'unixepoch'),'State','CORRECTED','Original',a.amount_paise,'Signed',-a.amount_paise) row_json`+base+` AND v.allocation_id IS NOT NULL`)
	if report == "MAINTENANCE" {
		return out
	}
	contributionScope := `EXISTS(SELECT 1 FROM selected_entries z WHERE z.id=c.source_id)`
	if report == "FUNDS" {
		contributionScope = `EXISTS(SELECT 1 FROM selected_funds z WHERE z.id=c.campaign_id AND z.flat_id=c.flat_id)`
	}
	contribBase := ` FROM fund_contributions c JOIN entries e ON e.id=c.source_id JOIN fund_campaigns f ON f.id=c.campaign_id LEFT JOIN fund_contribution_reversals v ON v.contribution_id=c.id
  LEFT JOIN receipts r ON r.entry_id=e.id WHERE e.flat_id IN (SELECT id FROM export_homes) AND f.state IN ('PUBLISHED','CLOSED') AND ` + contributionScope
	out = append(out, `SELECT json_object('Type','CONTRIBUTION','Home',`+exportHomeLabel+`,'Source',c.campaign_id,'Entry',c.source_id,'Link',c.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,'Title',f.title,
  'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',c.created_at,'unixepoch'),'State',CASE WHEN v.contribution_id IS NOT NULL THEN 'CORRECTED' WHEN NOT EXISTS(SELECT 1 FROM live_fund_contributions x WHERE x.id=c.id) THEN 'SOURCE_REVERSED' ELSE 'ACTIVE' END,
  'Original',c.amount_paise,'Active',CASE WHEN EXISTS(SELECT 1 FROM live_fund_contributions x WHERE x.id=c.id) THEN c.amount_paise ELSE 0 END) row_json`+contribBase)
	out = append(out, `SELECT json_object('Type','CONTRIBUTION_REVERSAL','Home',`+exportHomeLabel+`,'Source',c.campaign_id,'Entry',c.source_id,'Link',c.id,'Receipt',COALESCE(r.number,''),'Date',e.entry_date,
  'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',v.created_at,'unixepoch'),'State','CORRECTED','Original',c.amount_paise,'Signed',-c.amount_paise) row_json`+contribBase+` AND v.contribution_id IS NOT NULL`)
	if report == "FUNDS" {
		out = append(out, `SELECT json_object('Type','EXEMPTION','Home',`+exportLineLabel+`,'Source',l.id,'Entry',COALESCE(w.replacement_entry_id,''),'OriginalEntry',l.original_entry_id,'Link',w.id,'Date',l.start_date,'Title',l.title,
   'Recorded',strftime('%Y-%m-%dT%H:%M:%SZ',w.reviewed_at,'unixepoch'),'State','APPROVED','Original',w.amount_paise,'Signed',-w.amount_paise) row_json
   FROM fund_waivers w JOIN selected_funds l ON l.id=w.campaign_id AND l.flat_id=w.flat_id WHERE w.state='APPROVED'`)
	}
	return out
}
func (s *Store) buildFinanceExport(ctx context.Context, tx *sql.Tx, p Principal, out *FinanceExport, stamp string) ([]byte, error) {
	args := []any{sql.Named("homes", exportHomesJSON(out.Homes)), sql.Named("from", out.From), sql.Named("to", out.To), sql.Named("fund", out.FundID), sql.Named("all", out.Authority == "TREASURY"), sql.Named("actor", p.ID)}
	// Household net excludes money explicitly assigned to voluntary purposes.
	// Actual received cash and available received credit remain separate columns.
	err := tx.QueryRowContext(ctx, exportSourceCTE+`SELECT
  COALESCE(SUM(CASE WHEN e.kind IN ('CHARGE','OPENING_DEBIT') THEN e.amount_paise ELSE -e.amount_paise END),0)+COALESCE((SELECT SUM(v.amount_paise) FROM live_fund_contributions v WHERE v.flat_id IN (SELECT id FROM export_homes)),0),
  COALESCE(SUM(CASE WHEN e.kind='RECEIVED' THEN e.amount_paise ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN e.kind='RECEIVED' THEN e.amount_paise-COALESCE(u.amount,0) ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN e.kind='OPENING_CREDIT' THEN e.amount_paise ELSE 0 END),0)
  FROM entries e LEFT JOIN used_credit u ON u.source_id=e.id WHERE e.state='POSTED' AND e.flat_id IN (SELECT id FROM export_homes) AND NOT EXISTS(SELECT 1 FROM entry_reversals v WHERE v.entry_id=e.id)`, args...).Scan(&out.Summary.CurrentNet, &out.Summary.CurrentReceived, &out.Summary.AvailableReceived, &out.Summary.OpeningCredit)
	if err != nil {
		return nil, err
	}
	primary := financePrimarySQL(out.Report)
	err = tx.QueryRowContext(ctx, exportSourceCTE+`SELECT COUNT(*),
  COALESCE(SUM(CASE WHEN json_extract(row_json,'$.Kind')='RECEIVED' THEN json_extract(row_json,'$.Original') ELSE 0 END),0),
  COALESCE(SUM(CASE WHEN json_extract(row_json,'$.Kind')='RECEIVED' AND json_extract(row_json,'$.State')='REVERSED' THEN json_extract(row_json,'$.Original') ELSE 0 END),0),
		COALESCE(SUM(COALESCE(json_extract(row_json,'$.Requested'),CASE WHEN json_extract(row_json,'$.Kind') IN ('CHARGE','OPENING_DEBIT') THEN json_extract(row_json,'$.Original') ELSE 0 END)),0),
		COALESCE(SUM(CASE WHEN json_extract(row_json,'$.Type') IN ('MAINTENANCE','FUND') OR json_extract(row_json,'$.Kind') IN ('CHARGE','OPENING_DEBIT') THEN json_extract(row_json,'$.Active') ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN json_extract(row_json,'$.Type') IN ('MAINTENANCE','FUND') OR json_extract(row_json,'$.Kind') IN ('CHARGE','OPENING_DEBIT') THEN json_extract(row_json,'$.Allocated') ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN json_extract(row_json,'$.Type') IN ('MAINTENANCE','FUND') OR json_extract(row_json,'$.Kind') IN ('CHARGE','OPENING_DEBIT') THEN json_extract(row_json,'$.Outstanding') ELSE 0 END),0),
  COALESCE(SUM(json_extract(row_json,'$.Waived')),0),COALESCE(SUM(json_extract(row_json,'$.Voluntary')),0),COALESCE(SUM(json_extract(row_json,'$.Pending')),0) FROM (`+primary+`)`, args...).Scan(&out.Summary.Sources, &out.Summary.OriginalReceived, &out.Summary.ReversedReceived, &out.Summary.Requested, &out.Summary.Active, &out.Summary.Allocated, &out.Summary.Outstanding, &out.Summary.Waived, &out.Summary.Voluntary, &out.Summary.PendingReports)
	if err != nil {
		return nil, err
	}
	out.Summary.UsableReceived = out.Summary.OriginalReceived - out.Summary.ReversedReceived
	limits := s.financeLimits()
	body := &boundedExportBuffer{limit: limits.Bytes}
	writer := csv.NewWriter(body)
	hash := sha256.New()
	canonical := csv.NewWriter(hash)
	identity, _ := json.Marshal(struct {
		Filter    FinanceExportFilter
		Authority string
		Homes     []RecordHome
	}{out.FinanceExportFilter, out.Authority, out.Homes})
	hash.Write(identity)
	hash.Write([]byte("\n"))
	for _, w := range []*csv.Writer{writer, canonical} {
		if err = w.Write(financeExportColumns); err != nil {
			return nil, err
		}
	}
	add := func(row financeExportRow) error {
		if out.Rows >= limits.Rows {
			return invalid("This export exceeds 10,000 rows. Choose a smaller scope or date range.")
		}
		out.Rows++
		if err := writer.Write(row.cells(*out, stamp)); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
		return canonical.Write(row.cells(*out, ""))
	}
	scope := financeExportRow{Type: "SCOPE", Title: "Current operational snapshot; selected source dates with current corrections", State: "CURRENT", Requested: &out.Summary.Requested, Active: &out.Summary.Active, Allocated: &out.Summary.Allocated, Outstanding: &out.Summary.Outstanding, Waived: &out.Summary.Waived, Voluntary: &out.Summary.Voluntary, Pending: &out.Summary.PendingReports}
	if err = add(scope); err != nil {
		return nil, err
	}
	query := primary
	for _, q := range financeLinksSQL(out.Report) {
		query += " UNION ALL " + q
	}
	rows, err := tx.QueryContext(ctx, exportSourceCTE+`SELECT row_json FROM (`+query+`) ORDER BY json_extract(row_json,'$.Date'),json_extract(row_json,'$.Source'),json_extract(row_json,'$.Type'),json_extract(row_json,'$.Link'),json_extract(row_json,'$.Entry') LIMIT :limit`, append(args, sql.Named("limit", limits.Rows))...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if len(data) > limits.Bytes {
			return nil, invalid("An original row exceeds the export byte allowance. Choose a smaller scope.")
		}
		var row financeExportRow
		if err = json.Unmarshal([]byte(data), &row); err != nil {
			return nil, err
		}
		if err = add(row); err != nil {
			return nil, err
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	canonical.Flush()
	if err = canonical.Error(); err != nil {
		return nil, err
	}
	out.ContentHash = hex.EncodeToString(hash.Sum(nil))
	digest := sha256.Sum256(body.Bytes())
	out.SHA256 = hex.EncodeToString(digest[:])
	out.Bytes = body.Len()
	out.GeneratedAt = stamp
	return body.Bytes(), nil
}
