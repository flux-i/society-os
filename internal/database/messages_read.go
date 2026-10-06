package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func messageScope(p Principal) (string, []any) {
	notice, receipt := "0", "0"
	if messageStaff(p, "NOTICE") {
		notice = "source_kind='NOTICE'"
	}
	if messageStaff(p, "RECEIPT") {
		receipt = "source_kind='RECEIPT'"
	}
	return "(" + notice + " OR " + receipt + ` OR (message_batches.state IN('APPROVED','CANCELLED') AND EXISTS(SELECT 1 FROM message_recipients mr WHERE mr.batch_id=message_batches.id AND mr.snapshot_version=message_batches.snapshot_version AND mr.resident_id=? AND mr.frozen_reason='')))`, []any{p.ResidentID}
}
func decorateMessageBatch(ctx context.Context, q identityReader, p Principal, x *MessageBatch) error {
	staff := messageStaff(p, x.Source.Kind)
	clause, args := "d.batch_id=? AND d.snapshot_version=?", []any{x.ID, x.SnapshotVersion}
	if !staff {
		clause += ` AND EXISTS(SELECT 1 FROM message_recipients mr WHERE mr.delivery_id=d.id AND mr.resident_id=?)`
		args = append(args, p.ResidentID)
	}
	// Frozen destinations exist before review, but only an approved batch has
	// delivery outcomes. Preserve outcomes when an approved queue is cancelled.
	clause += ` AND ? IN('APPROVED','CANCELLED')`
	args = append(args, x.State)
	rows, e := q.QueryContext(ctx, "SELECT d.state,COUNT(*) FROM message_deliveries d WHERE "+clause+" GROUP BY d.state", args...)
	if e != nil {
		return e
	}
	x.Outcomes = map[string]int{}
	for rows.Next() {
		var state string
		var n int
		if e = rows.Scan(&state, &n); e != nil {
			rows.Close()
			return e
		}
		x.Outcomes[state] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if !staff {
		var disposition, reason string
		if e = q.QueryRowContext(ctx, `SELECT disposition,reason FROM message_recipients WHERE batch_id=? AND snapshot_version=? AND resident_id=? AND frozen_reason=''`, x.ID, x.SnapshotVersion, p.ResidentID).Scan(&disposition, &reason); e != nil {
			return e
		}
		if disposition != "ELIGIBLE" && disposition != "HANDED_OFF" {
			x.Outcomes = map[string]int{disposition: 1}
		}
		x.Counts = MessageCounts{TargetPeople: 1, SourcePeople: 1, ConsentedPeople: 1, EligiblePeople: 1, Destinations: 1, Reasons: map[string]int{}}
		src, e := messageSourceIn(ctx, q, x.Source.Kind, x.Source.ID)
		visible := false
		if e == nil {
			people, e := messagePeople(ctx, q)
			if e != nil {
				return e
			}
			for _, person := range people {
				if person.ID == p.ResidentID {
					visible = messageSourceMatches(person, src)
					break
				}
			}
		} else if e != sql.ErrNoRows {
			return e
		}
		if !visible {
			x.Counts.SourcePeople, x.Counts.EligiblePeople = 0, 0
			x.Source.ID, x.Source.HomeID, x.Source.EntryID, x.Source.Link = "", "", "", ""
			x.Envelope = ""
		}
		x.Source.Title = "Community update"
		if x.Source.Kind == "RECEIPT" {
			x.Source.Title = "Private receipt"
		}
		x.Source.Version, x.Source.Audience, x.Source.Wing = "", "", ""
		x.Target = MessageTarget{Kind: "PERSONAL", IDs: []string{}}
		x.ProposedBy, x.ReviewedBy, x.PortalOrigin, x.PreviewHash = "", "", "", ""
		return nil
	}
	x.CanRefresh = x.State == "PENDING" && x.ProposedBy == p.ID
	x.CanWithdraw = x.CanRefresh
	if x.State == "PENDING" {
		fresh, e := resolveMessage(ctx, q, MessageInput{SourceKind: x.Source.Kind, SourceID: x.Source.ID, Channel: x.Channel, Target: x.Target, PortalOrigin: x.PortalOrigin})
		if e == sql.ErrNoRows {
			x.ReviewProblem = "SOURCE_UNAVAILABLE"
		} else if e != nil {
			return e
		} else if fresh.PreviewHash != x.PreviewHash {
			x.ReviewProblem = "PREVIEW_CHANGED"
		}
		x.CanApprove = x.ProposedBy != p.ID && x.ReviewProblem == ""
		proposerCurrent, err := messageOperatorCurrent(ctx, q, x.ProposedBy, x.Source.Kind)
		if err != nil {
			return err
		}
		if !proposerCurrent {
			x.ReviewProblem = "PROPOSER_AUTHORITY_ENDED"
			x.CanApprove = false
		}
	}
	if e = q.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_deliveries WHERE batch_id=? AND snapshot_version=? AND state='FAILED' AND attempts<3", x.ID, x.SnapshotVersion).Scan(&x.RetryableDeliveries); e != nil {
		return e
	}
	x.CanDispatch = x.State == "APPROVED" && (x.Outcomes["QUEUED"] > 0 || x.RetryableDeliveries > 0)
	x.CanCancel = x.State == "APPROVED" && (x.Outcomes["QUEUED"] > 0 || x.Outcomes["FAILED"] > 0)
	return nil
}
func (s *Store) MessagesFor(ctx context.Context, token, query, state, kind string, page int) (MessagePage, error) {
	out := MessagePage{Items: []MessageBatch{}, Page: page, PageSize: 12}
	if page < 1 || page > 10000 || len(query) > 100 || (kind != "" && kind != "NOTICE" && kind != "RECEIPT") || (state != "" && state != "PENDING" && state != "APPROVED" && state != "DECLINED" && state != "WITHDRAWN" && state != "CANCELLED") {
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
	scope, args := messageScope(p)
	where := " WHERE " + scope
	if kind != "" {
		where += " AND source_kind=?"
		args = append(args, kind)
	}
	if state != "" {
		where += " AND state=?"
		args = append(args, state)
	}
	if query != "" {
		where += " AND source_kind='NOTICE' AND json_extract(source_json,'$.title') LIKE ?"
		args = append(args, "%"+query+"%")
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_batches"+where, args...).Scan(&out.Total); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, messageBatchSelect+where+" ORDER BY proposed_at DESC,id LIMIT 12 OFFSET ?", append(args, (page-1)*12)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		x, e := scanMessageBatch(rows)
		if e != nil {
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
		if e = decorateMessageBatch(ctx, tx, p, &out.Items[i]); e != nil {
			return out, e
		}
	}
	return out, tx.Commit()
}
func (s *Store) MessageFor(ctx context.Context, token, id string, recipientPage, deliveryPage, eventPage int) (MessageDetail, error) {
	out := MessageDetail{Recipients: []MessageRecipient{}, Deliveries: []MessageDelivery{}, Events: []MessageEvent{}, RecipientPage: recipientPage, DeliveryPage: deliveryPage, EventPage: eventPage, PageSize: 20}
	for _, page := range []int{recipientPage, deliveryPage, eventPage} {
		if page < 1 || page > 10000 {
			return out, ErrInvalid
		}
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
	scope, args := messageScope(p)
	x, e := scanMessageBatch(tx.QueryRowContext(ctx, messageBatchSelect+" WHERE id=? AND "+scope, append([]any{id}, args...)...))
	if e != nil {
		return out, e
	}
	out.Staff = messageStaff(p, x.Source.Kind)
	if e = decorateMessageBatch(ctx, tx, p, &x); e != nil {
		return out, e
	}
	out.MessageBatch = x
	where, values := "mr.batch_id=? AND mr.snapshot_version=?", []any{id, x.SnapshotVersion}
	if !out.Staff {
		where += " AND mr.resident_id=? AND mr.frozen_reason=''"
		values = append(values, p.ResidentID)
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_recipients mr WHERE "+where, values...).Scan(&out.RecipientTotal); e != nil {
		return out, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT mr.resident_id,mr.name,mr.contact_version,COALESCE(d.destination,''),mr.reason,COALESCE(mr.delivery_id,''),CASE WHEN mr.disposition IN('ELIGIBLE','HANDED_OFF') AND ? IN('APPROVED','CANCELLED') THEN COALESCE(d.state,mr.disposition) ELSE mr.disposition END FROM message_recipients mr LEFT JOIN message_deliveries d ON d.id=mr.delivery_id WHERE `+where+" ORDER BY mr.name,mr.resident_id LIMIT 20 OFFSET ?", append(append([]any{x.State}, values...), (recipientPage-1)*20)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var r MessageRecipient
		if e = rows.Scan(&r.ID, &r.Name, &r.ContactVersion, &r.Destination, &r.Reason, &r.DeliveryID, &r.State); e != nil {
			rows.Close()
			return out, e
		}
		out.Recipients = append(out.Recipients, publicMessageRecipient(r, p, x.Source.Kind))
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if !out.Staff {
		for i := range out.Recipients {
			out.Recipients[i].Destination, out.Recipients[i].ContactVersion = "", 0
		}
		return out, tx.Commit()
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_deliveries WHERE batch_id=? AND snapshot_version=? AND ? IN('APPROVED','CANCELLED')", id, x.SnapshotVersion, x.State).Scan(&out.DeliveryTotal); e != nil {
		return out, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT id,destination,state,attempts,provider_id,reason,accepted_at,delivered_at,read_at,updated_at FROM message_deliveries WHERE batch_id=? AND snapshot_version=? AND ? IN('APPROVED','CANCELLED') ORDER BY id LIMIT 20 OFFSET ?`, id, x.SnapshotVersion, x.State, (deliveryPage-1)*20)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var d MessageDelivery
		if e = rows.Scan(&d.ID, &d.Destination, &d.State, &d.Attempts, &d.ProviderID, &d.Reason, &d.AcceptedAt, &d.DeliveredAt, &d.ReadAt, &d.UpdatedAt); e != nil {
			rows.Close()
			return out, e
		}
		if x.Source.Kind == "RECEIPT" && !p.CanManageContacts {
			d.Destination = maskMessageDestination(d.Destination)
		}
		out.Deliveries = append(out.Deliveries, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_events WHERE batch_id=?", id).Scan(&out.EventTotal); e != nil {
		return out, e
	}
	rows, e = tx.QueryContext(ctx, `SELECT e.version,e.action,u.display_name,e.reason,e.occurred_at,e.snapshot_json FROM message_events e JOIN users u ON u.id=e.actor_id WHERE e.batch_id=? ORDER BY e.version DESC LIMIT 20 OFFSET ?`, id, (eventPage-1)*20)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var v MessageEvent
		var snapshot string
		if e = rows.Scan(&v.Version, &v.Action, &v.Actor, &v.Reason, &v.At, &snapshot); e != nil {
			rows.Close()
			return out, e
		}
		if e = json.Unmarshal([]byte(snapshot), &v.Snapshot); e != nil {
			rows.Close()
			return out, e
		}
		out.Events = append(out.Events, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	return out, tx.Commit()
}
func messageDeliveryEvent(ctx context.Context, tx *sql.Tx, id, attempt, state, reason, provider string, at int64) error {
	var nullable any
	if attempt != "" {
		nullable = attempt
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO message_delivery_events(delivery_id,attempt_id,state,reason,occurred_at,provider_id) VALUES(?,?,?,?,?,?)`, id, nullable, state, reason, at, provider)
	return e
}
func cancelMessageUnsent(ctx context.Context, tx *sql.Tx, x MessageBatch, reason string) error {
	rows, e := tx.QueryContext(ctx, "SELECT id FROM message_deliveries WHERE batch_id=? AND snapshot_version=? AND state IN('QUEUED','FAILED')", x.ID, x.SnapshotVersion)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if _, e = tx.ExecContext(ctx, "UPDATE message_deliveries SET state='CANCELLED',reason=?,updated_at=? WHERE id=?", reason, time.Now().Unix(), id); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "UPDATE message_recipients SET disposition='CANCELLED',reason=? WHERE delivery_id=? AND disposition='ELIGIBLE'", reason, id); e != nil {
			return e
		}
		if e = messageDeliveryEvent(ctx, tx, id, "", "CANCELLED", reason, "", time.Now().Unix()); e != nil {
			return e
		}
	}
	return nil
}

type MessageTargetPage struct {
	Items    []RecordHome `json:"items"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

func (s *Store) MessageTargetsFor(ctx context.Context, token, kind, sourceID, target, search string, page int) (MessageTargetPage, error) {
	out := MessageTargetPage{Items: []RecordHome{}, Page: page, PageSize: 12}
	if (target != "PEOPLE" && target != "HOMES") || page < 1 || page > 10000 || len(search) > 100 {
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
	src, e := messageSourceIn(ctx, tx, kind, sourceID)
	if e != nil {
		return out, e
	}
	people, e := messagePeople(ctx, tx)
	if e != nil {
		return out, e
	}
	items := []RecordHome{}
	seen := map[string]bool{}
	for _, person := range people {
		if !messageSourceMatches(person, src) {
			continue
		}
		if target == "PEOPLE" {
			if strings.Contains(strings.ToLower(person.Name), strings.ToLower(search)) {
				items = append(items, RecordHome{ID: person.ID, Label: person.Name})
			}
		} else {
			for _, home := range person.Homes {
				if src.Kind == "RECEIPT" && home.ID != src.HomeID {
					continue
				}
				if seen[home.ID] {
					continue
				}
				seen[home.ID] = true
				var label string
				if e = tx.QueryRowContext(ctx, "SELECT b.code||'-'||f.flat_number FROM flats f JOIN buildings b ON b.id=f.building_id WHERE f.id=?", home.ID).Scan(&label); e != nil {
					return out, e
				}
				if strings.Contains(strings.ToLower(label), strings.ToLower(search)) {
					items = append(items, RecordHome{ID: home.ID, Label: label})
				}
			}
		}
	}
	out.Total = len(items)
	first := (page - 1) * 12
	for i := first; i < len(items) && i < first+12; i++ {
		out.Items = append(out.Items, items[i])
	}
	return out, tx.Commit()
}
