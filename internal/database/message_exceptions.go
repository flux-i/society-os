package database

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type MessageException struct {
	MessageID          string `json:"message_id"`
	DeliveryID         string `json:"delivery_id"`
	SourceKind         string `json:"source_kind"`
	Title              string `json:"title"`
	Destination        string `json:"destination"`
	State              string `json:"state"`
	Channel            string `json:"channel"`
	Attempts           int    `json:"attempts"`
	UpdatedAt          int64  `json:"updated_at"`
	RetryAt            int64  `json:"retry_at"`
	DeliveryPage       int    `json:"delivery_page"`
	CanRetry           bool   `json:"can_retry"`
	CanReconcile       bool   `json:"can_reconcile"`
	EligibilityProblem string `json:"eligibility_problem"`
	CurrentKey         string `json:"current_key"`
}
type MessageExceptionPage struct {
	Items    []MessageException `json:"items"`
	Counts   map[string]int     `json:"counts"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func (s *Store) MessageExceptionsFor(ctx context.Context, token, state string, page int) (MessageExceptionPage, error) {
	out := MessageExceptionPage{Items: []MessageException{}, Counts: map[string]int{}, Page: page, PageSize: 12}
	if (page < 1 || page > 10000) || (state != "" && state != "UNKNOWN" && state != "FAILED" && state != "CLAIMED") {
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
	if !messageStaff(p, "NOTICE") && !messageStaff(p, "RECEIPT") {
		return out, ErrForbidden
	}
	base := ` FROM message_batches b JOIN message_deliveries d ON d.batch_id=b.id AND d.snapshot_version=b.snapshot_version
 WHERE b.state IN('APPROVED','CANCELLED') AND d.state IN('UNKNOWN','FAILED','CLAIMED') AND
 ((b.source_kind IN('NOTICE','MEETING_REMINDER') AND ?) OR (b.source_kind IN('RECEIPT','STATEMENT','MAINTENANCE_REMINDER','FUND_REMINDER') AND ?))`
	args := []any{messageStaff(p, "NOTICE"), messageStaff(p, "RECEIPT")}
	rows, e := tx.QueryContext(ctx, "SELECT d.state,COUNT(*)"+base+" GROUP BY d.state", args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var key string
		var n int
		if e = rows.Scan(&key, &n); e != nil {
			rows.Close()
			return out, e
		}
		out.Counts[key] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	where := base
	if state != "" {
		where += " AND d.state=?"
		args = append(args, state)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*)"+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	out.Page = clampMaintenancePage(page, out.Total, out.PageSize)
	rows, e = tx.QueryContext(ctx, `SELECT b.id,d.id,b.source_kind,json_extract(b.source_json,'$.title'),d.destination,d.state,b.channel,d.attempts,d.updated_at,
 COALESCE((SELECT h.retry_at FROM whatsapp_handoffs h JOIN message_attempts a ON a.id=h.attempt_id WHERE a.delivery_id=d.id AND a.attempt_number=d.attempts),0),
 1+(SELECT COUNT(*) FROM message_deliveries earlier WHERE earlier.batch_id=b.id AND earlier.snapshot_version=b.snapshot_version AND earlier.id<d.id)/20`+where+` ORDER BY CASE d.state WHEN 'UNKNOWN' THEN 0 WHEN 'CLAIMED' THEN 1 ELSE 2 END,d.updated_at,d.id LIMIT 12 OFFSET ?`, append(args, (out.Page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var x MessageException
		if e = rows.Scan(&x.MessageID, &x.DeliveryID, &x.SourceKind, &x.Title, &x.Destination, &x.State, &x.Channel, &x.Attempts, &x.UpdatedAt, &x.RetryAt, &x.DeliveryPage); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for i := range out.Items {
		x := &out.Items[i]
		if financeMessageKind(x.SourceKind) && !p.CanManageContacts {
			x.Destination = maskMessageDestination(x.Destination)
		}
		batch, e := messageBatchIn(ctx, tx, x.MessageID)
		if e != nil {
			return out, e
		}
		x.CanReconcile = x.State == "UNKNOWN"
		x.EligibilityProblem, e = messageSourceProblem(ctx, tx, batch)
		if e != nil {
			return out, e
		}
		if x.State == "FAILED" && x.EligibilityProblem == "" {
			people, e := messagePeople(ctx, tx)
			if e != nil {
				return out, e
			}
			current := map[string]messagePerson{}
			for _, person := range people {
				current[person.ID] = person
			}
			rows, e := tx.QueryContext(ctx, "SELECT resident_id,contact_version FROM message_recipients WHERE delivery_id=? AND disposition IN('ELIGIBLE','HANDED_OFF')", x.DeliveryID)
			if e != nil {
				return out, e
			}
			frozen := []MessageRecipient{}
			for rows.Next() {
				var r MessageRecipient
				if e = rows.Scan(&r.ID, &r.ContactVersion); e != nil {
					rows.Close()
					return out, e
				}
				frozen = append(frozen, r)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, e
			}
			eligible := false
			for _, r := range frozen {
				person, exists := current[r.ID]
				if !exists {
					continue
				}
				reason, _ := messageRecipientReason(person, batch.Source, batch.Channel, batch.Purpose)
				if reason == "" && person.Contact.Version != r.ContactVersion {
					reason = "CONTACT_CHANGED"
				}
				if reason == "" && batch.Source.Reminder != nil {
					reason, e = reminderDispatchReason(ctx, tx, batch, person)
					if e != nil {
						return out, e
					}
				}
				if reason == "" {
					eligible = true
				}
			}
			if !eligible {
				x.EligibilityProblem = "NO_ELIGIBLE_RECIPIENT"
			}
		}
		x.CanRetry = x.State == "FAILED" && batch.State == "APPROVED" && x.Attempts < 3 && x.RetryAt <= time.Now().Unix() && x.EligibilityProblem == ""
		key := ""
		if reminderKind(batch.Source.Kind) {
			key, e = reminderReadKey(ctx, tx, p, batch)
			if e != nil {
				return out, e
			}
		}
		x.CurrentKey, e = reminderHash(strings.Join([]string{key, x.State, x.EligibilityProblem}, "|"))
		if e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
