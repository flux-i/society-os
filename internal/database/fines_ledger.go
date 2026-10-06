package database

import (
	"context"
	"database/sql"
	"strings"
)

// Legacy preservation fixtures also read their populated database before its
// upgrade. Read the table identity in the same snapshot, without treating a
// failed query as an empty financial result.
func fineSchemaAvailable(ctx context.Context, q queryer) (bool, error) {
	var available bool
	err := q.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='fines')").Scan(&available)
	return available, err
}

// Fine links decorate only an already authorised, bounded entry result. Keep
// the shared immutable entry reader independent of later-purpose tables.
func enrichFineEntries(ctx context.Context, tx *sql.Tx, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	available, err := fineSchemaAvailable(ctx, tx)
	if err != nil || !available {
		return err
	}
	marks := make([]string, len(entries))
	args := make([]any, len(entries))
	positions := map[string]int{}
	for i, entry := range entries {
		marks[i], args[i], positions[entry.ID] = "?", entry.ID, i
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id,f.id FROM entries e JOIN fines f
 ON e.id IN(f.original_entry_id,f.current_entry_id)
 OR EXISTS(SELECT 1 FROM fine_waivers w WHERE w.fine_id=f.id AND w.state='APPROVED' AND w.replacement_entry_id=e.id)
 WHERE e.id IN (`+strings.Join(marks, ",")+")", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var entry, fine string
		if err = rows.Scan(&entry, &fine); err != nil {
			return err
		}
		entries[positions[entry]].FineID = fine
	}
	return rows.Err()
}
