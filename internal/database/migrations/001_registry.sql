CREATE TABLE app_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

CREATE TABLE buildings (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
) STRICT;

CREATE TABLE flats (
    id TEXT PRIMARY KEY,
    building_id TEXT NOT NULL REFERENCES buildings(id),
    flat_number TEXT NOT NULL,
    floor INTEGER NOT NULL CHECK (floor >= 0),
    status TEXT NOT NULL CHECK (status IN ('OWNER_OCCUPIED', 'RENTED', 'VACANT')),
    UNIQUE (building_id, flat_number)
) STRICT;

CREATE TABLE residents (
    id TEXT PRIMARY KEY,
    full_name TEXT NOT NULL CHECK (length(trim(full_name)) > 0)
) STRICT;

CREATE TABLE flat_memberships (
    id TEXT PRIMARY KEY,
    flat_id TEXT NOT NULL REFERENCES flats(id),
    resident_id TEXT NOT NULL REFERENCES residents(id),
    relationship TEXT NOT NULL CHECK (relationship IN ('OWNER', 'TENANT', 'FAMILY', 'AUTHORIZED_OCCUPANT')),
    start_date TEXT NOT NULL,
    end_date TEXT,
    is_primary_contact INTEGER NOT NULL CHECK (is_primary_contact IN (0, 1)),
    can_view_finances INTEGER NOT NULL CHECK (can_view_finances IN (0, 1)),
    CHECK (end_date IS NULL OR end_date > start_date),
    UNIQUE (flat_id, resident_id, relationship, start_date)
) STRICT;

CREATE UNIQUE INDEX one_current_primary_contact ON flat_memberships(flat_id)
    WHERE is_primary_contact = 1 AND end_date IS NULL;
CREATE INDEX memberships_resident ON flat_memberships(resident_id, flat_id);
CREATE INDEX flats_building_status ON flats(building_id, status, floor, flat_number);

