CREATE TABLE household_finance_actions (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL REFERENCES users(id),
    operation_key TEXT NOT NULL,
    flat_id TEXT NOT NULL REFERENCES flats(id),
    person_id TEXT NOT NULL REFERENCES residents(id),
    request_digest TEXT NOT NULL CHECK(length(request_digest)=64),
    result_json TEXT NOT NULL CHECK(json_valid(result_json)),
    created_at INTEGER NOT NULL,
    UNIQUE(actor_id,operation_key)
) STRICT;
CREATE INDEX household_finance_home_history ON household_finance_actions(flat_id,created_at,id);
CREATE TRIGGER preserve_household_finance_update BEFORE UPDATE ON household_finance_actions
BEGIN SELECT RAISE(ABORT,'household financial access history is immutable'); END;
CREATE TRIGGER preserve_household_finance_delete BEFORE DELETE ON household_finance_actions
BEGIN SELECT RAISE(ABORT,'household financial access history is preserved'); END;
