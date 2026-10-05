package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
)

func fundScopeArgs(p Principal, home string) []any {
	return []any{sql.Named("all", p.CanReadAllRecords), sql.Named("actor", p.ID), sql.Named("resident", p.ResidentID), sql.Named("day", today()), sql.Named("home", home)}
}

const fundHomeScope = `(:all=1 OR EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=l.flat_id AND m.resident_id=:resident AND m.can_view_finances=1 AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day)))`
const fundLinesCTE = `WITH scoped_base AS (
 SELECT l.campaign_id,l.flat_id,b.code||'-'||f.flat_number home,l.requested_paise,l.waived_paise,l.version,
 COALESCE(l.current_entry_id,'') entry_id,COALESCE(l.original_entry_id,'') original_entry_id,c.state campaign_state,c.contribution_type,c.due_date,
 CASE WHEN e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=e.id) THEN e.amount_paise ELSE 0 END active_paise,
 COALESCE((SELECT SUM(a.amount_paise) FROM live_entry_allocations a WHERE a.charge_id=l.current_entry_id),0) allocated_paise,
 COALESCE((SELECT SUM(v.amount_paise) FROM live_fund_contributions v WHERE v.campaign_id=l.campaign_id AND v.flat_id=l.flat_id),0) voluntary_paise,
 (SELECT COUNT(*) FROM fund_reports r WHERE r.campaign_id=l.campaign_id AND r.flat_id=l.flat_id AND r.state IN ('PENDING','NEEDS_INFO') AND (:all=1 OR r.author_id=:actor)) pending_reports
 FROM fund_participants l JOIN fund_campaigns c ON c.id=l.campaign_id JOIN flats f ON f.id=l.flat_id JOIN buildings b ON b.id=f.building_id LEFT JOIN entries e ON e.id=l.current_entry_id
 WHERE (:all=1 OR c.state IN ('PUBLISHED','CLOSED')) AND ` + fundHomeScope + ` AND (:home='' OR l.flat_id=:home)),
 scoped_lines AS (SELECT *,MAX(active_paise-allocated_paise,0) outstanding_paise FROM scoped_base),
 fund_summary AS (SELECT campaign_id,COUNT(*) participants,SUM(requested_paise) requested_paise,SUM(active_paise) active_paise,SUM(allocated_paise) allocated_paise,SUM(outstanding_paise) outstanding_paise,
 SUM(CASE WHEN due_date<:day THEN outstanding_paise ELSE 0 END) overdue_paise,SUM(waived_paise) waived_paise,SUM(voluntary_paise) voluntary_paise,SUM(pending_reports) pending_reports,
 SUM((active_paise>0 AND outstanding_paise=0) OR voluntary_paise>0) paid_homes,SUM(allocated_paise>0 AND outstanding_paise>0) partial_homes,
 SUM(active_paise>0 AND allocated_paise=0) unpaid_homes,SUM(requested_paise>0 AND waived_paise=requested_paise) exempt_homes FROM scoped_lines GROUP BY campaign_id) `
const fundSelect = `SELECT c.id,c.title,c.purpose,c.contribution_type,c.start_date,c.due_date,c.target_paise,c.source_reference,c.note,c.state,c.version,c.author_id,u.display_name,c.created_at,COALESCE(v.display_name,''),COALESCE(c.decided_at,0),c.decision_reason,
 s.participants,s.requested_paise,s.active_paise,s.allocated_paise,s.outstanding_paise,s.overdue_paise,s.waived_paise,s.voluntary_paise,s.pending_reports,s.paid_homes,s.partial_homes,s.unpaid_homes,s.exempt_homes FROM fund_campaigns c JOIN fund_summary s ON s.campaign_id=c.id JOIN users u ON u.id=c.author_id LEFT JOIN users v ON v.id=c.decided_by `

func scanFund(row interface{ Scan(...any) error }, p Principal) (FundCampaign, error) {
	var c FundCampaign
	err := row.Scan(&c.ID, &c.Title, &c.Purpose, &c.ContributionType, &c.StartDate, &c.DueDate, &c.TargetPaise, &c.SourceReference, &c.Note, &c.State, &c.Version, &c.AuthorID, &c.Author, &c.CreatedAt, &c.DecidedBy, &c.DecidedAt, &c.DecisionReason, &c.Participants, &c.RequestedPaise, &c.ActivePaise, &c.AllocatedPaise, &c.OutstandingPaise, &c.OverduePaise, &c.WaivedPaise, &c.VoluntaryPaise, &c.PendingReports, &c.PaidHomes, &c.PartialHomes, &c.UnpaidHomes, &c.ExemptHomes)
	if !p.CanReadAllRecords {
		c.SourceReference, c.Note, c.AuthorID, c.Author, c.DecidedBy, c.DecisionReason = "", "", "", "", "", ""
	}
	return c, err
}
func validFundFilters(query, home, state string) bool {
	return validText(query, 0, 100) && len(home) <= 100 && (state == "" || state == "PENDING" || state == "PUBLISHED" || state == "CLOSED" || state == "DECLINED" || state == "WITHDRAWN")
}
func (s *Store) FundCampaignsFor(ctx context.Context, token, query, home, state string, page int) (FundPage, error) {
	out := FundPage{Items: []FundCampaign{}, Homes: []RecordHome{}, PageSize: 12, Page: page}
	if !validFundFilters(query, home, state) || !boundedMaintenancePage(page) {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	out.Homes, err = financialHomes(ctx, tx, p)
	if err != nil {
		return out, err
	}
	if home != "" {
		if _, err = permittedFinancialHome(ctx, tx, p, home); err != nil {
			return out, err
		}
	}
	args := append(fundScopeArgs(p, home), sql.Named("query", "%"+query+"%"), sql.Named("state", state))
	where := ` WHERE (:state='' OR c.state=:state) AND (c.title LIKE :query OR c.purpose LIKE :query) `
	if err = tx.QueryRowContext(ctx, fundLinesCTE+"SELECT COUNT(*) FROM fund_campaigns c JOIN fund_summary s ON s.campaign_id=c.id"+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	err = tx.QueryRowContext(ctx, fundLinesCTE+`SELECT COALESCE(SUM(s.requested_paise),0),COALESCE(SUM(s.active_paise),0),COALESCE(SUM(s.allocated_paise),0),COALESCE(SUM(s.outstanding_paise),0),COALESCE(SUM(s.overdue_paise),0),COALESCE(SUM(s.waived_paise),0),COALESCE(SUM(s.voluntary_paise),0),COALESCE(SUM(s.pending_reports),0),COALESCE(SUM(s.paid_homes),0),COALESCE(SUM(s.partial_homes),0),COALESCE(SUM(s.unpaid_homes),0),COALESCE(SUM(s.exempt_homes),0) FROM fund_campaigns c JOIN fund_summary s ON s.campaign_id=c.id`+where, args...).Scan(&out.Totals.RequestedPaise, &out.Totals.ActivePaise, &out.Totals.AllocatedPaise, &out.Totals.OutstandingPaise, &out.Totals.OverduePaise, &out.Totals.WaivedPaise, &out.Totals.VoluntaryPaise, &out.Totals.PendingReports, &out.Totals.PaidHomes, &out.Totals.PartialHomes, &out.Totals.UnpaidHomes, &out.Totals.ExemptHomes)
	if err != nil {
		return out, err
	}
	args = append(args, sql.Named("limit", out.PageSize), sql.Named("offset", (out.Page-1)*out.PageSize))
	rows, err := tx.QueryContext(ctx, fundLinesCTE+fundSelect+where+"ORDER BY c.created_at DESC,c.id LIMIT :limit OFFSET :offset", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		c, e := scanFund(rows, p)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func fundEvents(ctx context.Context, tx *sql.Tx, kind, id string, page int) ([]FundEvent, int, int, error) {
	items := []FundEvent{}
	var total int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM fund_events WHERE subject_type=? AND subject_id=?", kind, id).Scan(&total); err != nil {
		return items, 0, 0, err
	}
	page = clampMaintenancePage(page, total, 20)
	rows, err := tx.QueryContext(ctx, "SELECT e.action,u.display_name,e.reason,e.version,e.occurred_at FROM fund_events e JOIN users u ON u.id=e.actor_id WHERE e.subject_type=? AND e.subject_id=? ORDER BY e.id DESC LIMIT 20 OFFSET ?", kind, id, (page-1)*20)
	if err != nil {
		return items, 0, 0, err
	}
	for rows.Next() {
		var e FundEvent
		if err = rows.Scan(&e.Action, &e.Actor, &e.Reason, &e.Version, &e.At); err != nil {
			rows.Close()
			return items, 0, 0, err
		}
		items = append(items, e)
	}
	err = rows.Err()
	rows.Close()
	return items, total, page, err
}
func (s *Store) FundCampaignFor(ctx context.Context, token, id string, linePage, eventPage int) (FundDetail, error) {
	out := FundDetail{Lines: []FundParticipant{}, PageSize: 20}
	if id == "" || len(id) > 100 || !boundedMaintenancePage(linePage) || !boundedMaintenancePage(eventPage) {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	args := append(fundScopeArgs(p, ""), sql.Named("id", id))
	out.FundCampaign, err = scanFund(tx.QueryRowContext(ctx, fundLinesCTE+fundSelect+"WHERE c.id=:id", args...), p)
	if err != nil {
		return out, err
	}
	out.LinePage = clampMaintenancePage(linePage, out.Participants, out.PageSize)
	args = append(args, sql.Named("limit", out.PageSize), sql.Named("offset", (out.LinePage-1)*out.PageSize))
	rows, err := tx.QueryContext(ctx, fundLinesCTE+`SELECT flat_id,home,requested_paise,active_paise,allocated_paise,outstanding_paise,waived_paise,voluntary_paise,pending_reports,entry_id,original_entry_id,version FROM scoped_lines WHERE campaign_id=:id ORDER BY home LIMIT :limit OFFSET :offset`, args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var l FundParticipant
		if err = rows.Scan(&l.FlatID, &l.Home, &l.RequestedPaise, &l.ActivePaise, &l.AllocatedPaise, &l.OutstandingPaise, &l.WaivedPaise, &l.VoluntaryPaise, &l.PendingReports, &l.EntryID, &l.OriginalEntryID, &l.Version); err != nil {
			rows.Close()
			return out, err
		}
		l.Status = "UNPAID"
		switch {
		case out.State != "PUBLISHED" && out.State != "CLOSED":
			l.Status = "PROPOSED"
		case out.ContributionType == "VOLUNTARY":
			if l.VoluntaryPaise > 0 {
				l.Status = "CONFIRMED"
			} else {
				l.Status = "NO_CONTRIBUTION"
			}
		case l.WaivedPaise == l.RequestedPaise:
			l.Status = "EXEMPT"
		case l.ActivePaise == 0:
			l.Status = "CORRECTED"
		case l.OutstandingPaise == 0:
			l.Status = "PAID"
		case l.AllocatedPaise > 0:
			l.Status = "PART_PAID"
		}
		out.Lines = append(out.Lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if p.CanReadAllRecords {
		out.Events, out.EventTotal, out.EventPage, err = fundEvents(ctx, tx, "CAMPAIGN", id, eventPage)
		if err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}

const reportSelect = `SELECT r.id,r.campaign_id,c.title,r.flat_id,b.code||'-'||f.flat_number,r.author_id,u.display_name,r.amount_paise,r.payment_date,r.payer,r.method,r.reference,r.comment,COALESCE(r.evidence_id,''),r.state,r.version,r.created_at,r.updated_at,COALESCE(r.entry_id,''),COALESCE(r.allocation_id,''),COALESCE(r.duplicate_report_id,''),COALESCE(v.display_name,''),COALESCE(r.reviewed_at,0),r.decision_reason,COALESCE(rc.id,''),COALESCE(rc.number,''),
 CASE WHEN EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=r.entry_id) OR (r.allocation_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM live_entry_allocations a WHERE a.id=r.allocation_id)) OR EXISTS(SELECT 1 FROM fund_contributions fc JOIN fund_contribution_reversals fr ON fr.contribution_id=fc.id WHERE fc.campaign_id=r.campaign_id AND fc.flat_id=r.flat_id AND fc.source_id=r.entry_id) THEN 'CORRECTED' ELSE r.state END
 FROM fund_reports r JOIN fund_campaigns c ON c.id=r.campaign_id JOIN flats f ON f.id=r.flat_id JOIN buildings b ON b.id=f.building_id JOIN users u ON u.id=r.author_id LEFT JOIN users v ON v.id=r.reviewer_id LEFT JOIN receipts rc ON rc.entry_id=r.entry_id `
const reportScope = `(:all=1 OR (r.author_id=:actor AND EXISTS(SELECT 1 FROM flat_memberships m WHERE m.flat_id=r.flat_id AND m.resident_id=:resident AND m.can_view_finances=1 AND m.start_date<=:day AND (m.end_date IS NULL OR m.end_date>:day))))`

func scanFundReport(row interface{ Scan(...any) error }) (FundReport, error) {
	var r FundReport
	err := row.Scan(&r.ID, &r.CampaignID, &r.Campaign, &r.FlatID, &r.Home, &r.AuthorID, &r.Author, &r.AmountPaise, &r.PaymentDate, &r.Payer, &r.Method, &r.Reference, &r.Comment, &r.EvidenceID, &r.State, &r.Version, &r.CreatedAt, &r.UpdatedAt, &r.EntryID, &r.AllocationID, &r.DuplicateReportID, &r.Reviewer, &r.ReviewedAt, &r.DecisionReason, &r.ReceiptID, &r.Receipt, &r.CurrentState)
	return r, err
}
func fundReportIn(ctx context.Context, tx *sql.Tx, p Principal, id string) (FundReport, error) {
	args := append(fundScopeArgs(p, ""), sql.Named("id", id))
	return scanFundReport(tx.QueryRowContext(ctx, reportSelect+" WHERE r.id=:id AND "+reportScope, args...))
}
func (s *Store) FundReportFor(ctx context.Context, token, id string, eventPage int) (FundReport, error) {
	if id == "" || len(id) > 100 || !boundedMaintenancePage(eventPage) {
		return FundReport{}, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return FundReport{}, err
	}
	defer tx.Rollback()
	out, err := fundReportIn(ctx, tx, p, id)
	if err != nil {
		return out, err
	}
	out.PageSize = 20
	out.Events, out.EventTotal, out.EventPage, err = fundEvents(ctx, tx, "REPORT", id, eventPage)
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) FundReportsFor(ctx context.Context, token, query, campaign, home, state string, page int) (FundReportPage, error) {
	out := FundReportPage{Items: []FundReport{}, Homes: []RecordHome{}, Counts: map[string]int64{}, Page: page, PageSize: 12}
	if !validText(query, 0, 100) || len(campaign) > 100 || len(home) > 100 || !boundedMaintenancePage(page) || (state != "" && !strings.Contains("|PENDING|NEEDS_INFO|REJECTED|WITHDRAWN|CONFIRMED|DUPLICATE|", "|"+state+"|")) {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	out.Homes, err = financialHomes(ctx, tx, p)
	if err != nil {
		return out, err
	}
	if home != "" {
		if _, err = permittedFinancialHome(ctx, tx, p, home); err != nil {
			return out, err
		}
	}
	args := append(fundScopeArgs(p, home), sql.Named("query", "%"+query+"%"), sql.Named("campaign", campaign), sql.Named("state", state))
	where := ` WHERE ` + reportScope + ` AND (:home='' OR r.flat_id=:home) AND (:campaign='' OR r.campaign_id=:campaign) AND (:state='' OR r.state=:state) AND (c.title LIKE :query OR b.code||'-'||f.flat_number LIKE :query) `
	countBase := ` FROM fund_reports r JOIN fund_campaigns c ON c.id=r.campaign_id JOIN flats f ON f.id=r.flat_id JOIN buildings b ON b.id=f.building_id `
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+countBase+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	rows, err := tx.QueryContext(ctx, "SELECT r.state,COUNT(*)"+countBase+where+"GROUP BY r.state", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var key string
		var value int64
		if err = rows.Scan(&key, &value); err != nil {
			rows.Close()
			return out, err
		}
		out.Counts[key] = value
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	args = append(args, sql.Named("limit", out.PageSize), sql.Named("offset", (out.Page-1)*out.PageSize))
	rows, err = tx.QueryContext(ctx, reportSelect+where+"ORDER BY r.created_at DESC,r.id LIMIT :limit OFFSET :offset", args...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		r, e := scanFundReport(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) FundConfirmOptionsFor(ctx context.Context, token, id string) (FundConfirmOptions, error) {
	out := FundConfirmOptions{Entries: []FundReceiptChoice{}}
	if id == "" || len(id) > 100 {
		return out, ErrInvalid
	}
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if !p.CanManageRecords {
		return out, ErrForbidden
	}
	report, err := fundReportIn(ctx, tx, p, id)
	if err != nil {
		return out, err
	}
	err = tx.QueryRowContext(ctx, `SELECT c.contribution_type,c.state,COALESCE(l.current_entry_id,''),CASE WHEN e.state='POSTED' AND NOT EXISTS(SELECT 1 FROM entry_reversals r WHERE r.entry_id=e.id) THEN e.amount_paise-COALESCE((SELECT SUM(a.amount_paise) FROM live_entry_allocations a WHERE a.charge_id=e.id),0) ELSE 0 END FROM fund_campaigns c JOIN fund_participants l ON l.campaign_id=c.id LEFT JOIN entries e ON e.id=l.current_entry_id WHERE c.id=? AND l.flat_id=?`, report.CampaignID, report.FlatID).Scan(&out.ContributionType, &out.CampaignState, &out.ChargeID, &out.OutstandingPaise)
	if err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id,e.kind,e.description,e.entry_date,e.amount_paise,COALESCE((SELECT SUM(x.amount_paise) FROM live_credit_uses x WHERE x.source_id=e.id),0),e.amount_paise-COALESCE((SELECT SUM(x.amount_paise) FROM live_credit_uses x WHERE x.source_id=e.id),0),r.number,COALESCE(v.verification_source,''),COALESCE(v.payment_identity,'') FROM entries e JOIN receipts r ON r.entry_id=e.id LEFT JOIN verified_fund_payments v ON v.entry_id=e.id WHERE e.flat_id=? AND e.kind='RECEIVED' AND e.state='POSTED' AND e.amount_paise=? AND e.entry_date=? AND e.method=? AND upper(trim(e.reference))=upper(trim(?)) AND NOT EXISTS(SELECT 1 FROM entry_reversals x WHERE x.entry_id=e.id) ORDER BY e.created_at DESC,e.id LIMIT 40`, report.FlatID, report.AmountPaise, report.PaymentDate, report.Method, report.Reference)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var e FundReceiptChoice
		if err = rows.Scan(&e.ID, &e.Kind, &e.Description, &e.Date, &e.AmountPaise, &e.AllocatedPaise, &e.RemainingPaise, &e.Receipt, &e.VerificationSource, &e.PaymentIdentity); err != nil {
			rows.Close()
			return out, err
		}
		out.Entries = append(out.Entries, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Store) FundReportEvidenceFor(ctx context.Context, token, id string) (LibraryDocument, []byte, error) {
	tx, p, err := beginMaintenanceRead(ctx, s, token)
	if err != nil {
		return LibraryDocument{}, nil, err
	}
	defer tx.Rollback()
	report, err := fundReportIn(ctx, tx, p, id)
	if err != nil {
		return LibraryDocument{}, nil, err
	}
	if report.EvidenceID == "" {
		return LibraryDocument{}, nil, sql.ErrNoRows
	}
	x, err := scanDocument(tx.QueryRowContext(ctx, librarySelect+" WHERE x.id=? AND x.flat_id=? AND x.uploaded_by=? AND x.category='PAYMENT_EVIDENCE' AND x.validation_status='AVAILABLE' AND x.review_state IN ('PENDING','APPROVED') AND g.archived=0", report.EvidenceID, report.FlatID, report.AuthorID))
	if err != nil {
		return LibraryDocument{}, nil, err
	}
	var bytes []byte
	if err = tx.QueryRowContext(ctx, "SELECT original_bytes FROM local_document_objects WHERE document_id=?", x.ID).Scan(&bytes); err != nil {
		return LibraryDocument{}, nil, err
	}
	sum := sha256.Sum256(bytes)
	if fmt.Sprintf("%x", sum) != x.SHA256 {
		return LibraryDocument{}, nil, fmt.Errorf("evidence original checksum is unavailable")
	}
	return x, bytes, tx.Commit()
}
