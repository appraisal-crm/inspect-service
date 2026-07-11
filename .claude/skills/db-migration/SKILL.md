---
description: Creates a new SQL migration (up + down) for a service. Use when adding a table, column, index, or any schema change.
disable-model-invocation: true
argument-hint: <service-name> <description> e.g. inspect-service add_photos_table
---

# New migration: $ARGUMENTS

Arguments format: `<service-name> <description>` — e.g. `inspect-service add_photos_table`
Parse $ARGUMENTS: first word = service name, rest = description.

!`SERVICE=$(echo "$ARGUMENTS" | awk '{print $1}'); ls services/$SERVICE/migrations/ 2>/dev/null | sort | tail -5`

## Steps

1. **Determine the next version number** from the output above (last file number + 1)

2. **Create two files** (SERVICE = first word of $ARGUMENTS, DESCRIPTION = remaining words joined with `_`):
   - `services/{SERVICE}/migrations/{VERSION}_{DESCRIPTION}.up.sql`
   - `services/{SERVICE}/migrations/{VERSION}_{DESCRIPTION}.down.sql`

3. **up.sql rules:**
   - `CREATE TABLE IF NOT EXISTS`
   - UUID primary key: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
   - Timestamps: `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
   - Status enums: `CREATE TYPE ... AS ENUM` declared before the table
   - `version INTEGER NOT NULL DEFAULT 1` for optimistic locking on mutable entities
   - Indexes on every FK column and every column used in WHERE filters

4. **down.sql rules:**
   - Exact inverse of up.sql, in reverse order
   - `DROP TABLE IF EXISTS`, `DROP TYPE IF EXISTS`

5. **Reference — inspection photos table:**
```sql
-- up
CREATE TYPE photo_status AS ENUM ('pending', 'uploaded', 'rejected');

CREATE TABLE IF NOT EXISTS inspection_photos (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL,
    s3_key      TEXT NOT NULL,
    status      photo_status NOT NULL DEFAULT 'pending',
    uploaded_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_inspection_photos_task_id ON inspection_photos(task_id);

-- down
DROP TABLE IF EXISTS inspection_photos;
DROP TYPE IF EXISTS photo_status;
```

After creating the files, print the command to apply the migration.
