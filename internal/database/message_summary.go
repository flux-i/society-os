package database

import (
	"context"
	"database/sql"
	"time"
)

type MessageSummary struct {
	Batches    map[string]int `json:"batches"`
	Outcomes   map[string]int `json:"outcomes"`
	Simulation bool           `json:"simulation"`
}

func (s *Store) MessageSummaryFor(ctx context.Context, token string) (MessageSummary, error) {
	out := MessageSummary{Batches: map[string]int{}, Outcomes: map[string]int{}, Simulation: true}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	p, e := fullPrincipal(ctx, tx, TokenHash(token), time.Now())
	if e != nil {
		return out, e
	}
	out, e = messageSummaryIn(ctx, tx, p)
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func messageSummaryIn(ctx context.Context, q identityReader, p Principal) (MessageSummary, error) {
	out := MessageSummary{Batches: map[string]int{}, Outcomes: map[string]int{}, Simulation: true}
	scope, args := messageScope(p)
	rows, e := q.QueryContext(ctx, "SELECT state,COUNT(*) FROM message_batches WHERE "+scope+" GROUP BY state", args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var state string
		var n int
		if e = rows.Scan(&state, &n); e != nil {
			rows.Close()
			return out, e
		}
		out.Batches[state] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	// Staff counts are unique envelopes; a resident's stopped/suppressed decision
	// overrides a shared envelope delivered for somebody else.
	rows, e = q.QueryContext(ctx, `SELECT CASE WHEN ((source_kind IN('NOTICE','MEETING_REMINDER') AND ?) OR (source_kind IN('RECEIPT','STATEMENT','MAINTENANCE_REMINDER','FUND_REMINDER') AND ?)) THEN d.state ELSE CASE WHEN mr.disposition IN('ELIGIBLE','HANDED_OFF') THEN d.state ELSE mr.disposition END END,COUNT(*)
 FROM message_batches JOIN message_deliveries d ON d.batch_id=message_batches.id AND d.snapshot_version=message_batches.snapshot_version
 LEFT JOIN message_recipients mr ON mr.delivery_id=d.id AND mr.resident_id=? WHERE message_batches.state IN('APPROVED','CANCELLED') AND `+scope+` GROUP BY 1`, append([]any{messageStaff(p, "NOTICE"), messageStaff(p, "RECEIPT"), p.ResidentID}, args...)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var state string
		var n int
		if e = rows.Scan(&state, &n); e != nil {
			rows.Close()
			return out, e
		}
		out.Outcomes[state] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, nil
}

func overviewMessages(ctx context.Context, tx *sql.Tx, p Principal, out *Overview) error {
	summary, e := messageSummaryIn(ctx, tx, p)
	if e != nil {
		return e
	}
	out.Counts["awaiting_review"] = int64(summary.Batches["PENDING"])
	for _, state := range []string{"QUEUED", "CLAIMED", "UNKNOWN", "FAILED", "ACCEPTED", "DELIVERED", "READ", "OPTED_OUT", "SKIPPED", "CANCELLED"} {
		out.Counts[state] = int64(summary.Outcomes[state])
	}
	scope, args := messageScope(p)
	where := ` WHERE ` + scope + ` AND (state='PENDING' OR (state='APPROVED' AND EXISTS(SELECT 1 FROM message_deliveries d WHERE d.batch_id=message_batches.id AND d.snapshot_version=message_batches.snapshot_version AND d.state IN('QUEUED','UNKNOWN','FAILED') AND (((source_kind IN('NOTICE','MEETING_REMINDER') AND ?) OR (source_kind IN('RECEIPT','STATEMENT','MAINTENANCE_REMINDER','FUND_REMINDER') AND ?)) OR EXISTS(SELECT 1 FROM message_recipients mr WHERE mr.delivery_id=d.id AND mr.resident_id=? AND mr.disposition IN('ELIGIBLE','HANDED_OFF'))))))`
	values := append(args, messageStaff(p, "NOTICE"), messageStaff(p, "RECEIPT"), p.ResidentID)
	if e = overviewCounts(ctx, tx, out, []string{"attention_items"}, `SELECT COUNT(*) FROM message_batches`+where, values...); e != nil {
		return e
	}
	return overviewItems(ctx, tx, out, `SELECT id,CASE source_kind WHEN 'NOTICE' THEN 'Community message' WHEN 'STATEMENT' THEN 'Financial statement message' WHEN 'MAINTENANCE_REMINDER' THEN 'Private maintenance reminder' WHEN 'FUND_REMINDER' THEN 'Private fund reminder' WHEN 'MEETING_REMINDER' THEN 'Private meeting reminder' ELSE 'Private receipt message' END,'','MESSAGE',state,'','',updated_at,0
 FROM message_batches`+where+` ORDER BY CASE state WHEN 'PENDING' THEN 0 ELSE 1 END,updated_at,id LIMIT 4`, values...)
}
