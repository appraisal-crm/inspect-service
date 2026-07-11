-- Lookup table for inspection statuses (FK target instead of a bare TEXT).
-- Two states only: an inspection is scheduled, then completed.
CREATE TABLE inspection_statuses (
    id         TEXT PRIMARY KEY,
    sort_order INTEGER NOT NULL
);

INSERT INTO inspection_statuses (id, sort_order) VALUES
    ('scheduled', 1),
    ('completed', 2);

-- One inspection per request (request_id UNIQUE). inspector_id is nullable:
-- the row is created unassigned and an appraiser assigns the inspector later.
-- property_data is free-form object characteristics (rooms, area, condition, ...).
CREATE TABLE inspections (
    id            UUID PRIMARY KEY,
    request_id    UUID NOT NULL UNIQUE,
    inspector_id  UUID,
    status        TEXT NOT NULL DEFAULT 'scheduled' REFERENCES inspection_statuses(id),
    notes         TEXT,
    property_data JSONB,
    scheduled_at  TIMESTAMPTZ NOT NULL,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_inspections_inspector_id ON inspections (inspector_id);
CREATE INDEX idx_inspections_status ON inspections (status);

-- Photo binaries live in S3; we store only the object key. ON DELETE CASCADE so
-- photos disappear with their inspection.
CREATE TABLE inspection_photos (
    id            UUID PRIMARY KEY,
    inspection_id UUID NOT NULL REFERENCES inspections(id) ON DELETE CASCADE,
    s3_key        TEXT NOT NULL,
    uploaded_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_inspection_photos_inspection_id ON inspection_photos (inspection_id);
