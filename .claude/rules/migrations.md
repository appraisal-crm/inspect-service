---
paths:
  - "migrations/*.sql"
---

# SQL migration conventions

- File naming: `{version}_{description}.up.sql` and `{version}_{description}.down.sql`
- Version: 6-digit zero-padded number — `000001_create_requests.up.sql`
- **Never modify an already-applied migration** — create a new additive one instead
- `down.sql` must exactly invert `up.sql`
- Use `IF NOT EXISTS` / `IF EXISTS` where applicable
- Index every FK column and every column frequently used in WHERE
- Use `CREATE TYPE ... AS ENUM` for status fields — never plain strings without a constraint

```sql
-- Status enum example
CREATE TYPE request_status AS ENUM (
    'new', 'in_progress', 'inspection_scheduled',
    'inspection_completed', 'appraisal', 'report_sent', 'closed'
);

-- Standard columns every mutable table must have
id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
version    INTEGER NOT NULL DEFAULT 1  -- optimistic locking
```
