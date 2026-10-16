# Data Models, Schemas & Persistence Guide

This document specifies the database schemas, indexing strategies, repository query operations, and data models for both the primary **SQLite relational engine** and the fallback **MongoDB Atlas** driver in **Wedrink**.

---

## 1. Storage Overview & Multi-Driver Support

The system abstracts all database mutations behind Go interfaces in `internal/repository/interfaces.go`:
- `UserRepo`: User authentication, profile updates, and soft deletes.
- `ReportRepo`: EOD report creation, updating, cursor pagination, filtering, and monthly aggregations.
- `NotificationRepo`: Store remark notifications and unread badges.
- `AuditRepo`: Activity tracking and change audit logs.

The active driver is selected via `DB_DRIVER` in `.env`:
- `DB_DRIVER=sqlite` (**Default & Recommended**): In-process SQLite via `modernc.org/sqlite` (Pure Go, Zero CGO, WAL mode).
- `DB_DRIVER=mongodb` (**Fallback**): MongoDB Atlas driver via `go.mongodb.org/mongo-driver/v2`.

---

## 2. SQLite Relational Schema (`internal/db/sqlite.go`)

SQLite uses strict normalized tables with foreign keys and financial constraints:

### A. Table `eod_reports`
```sql
CREATE TABLE IF NOT EXISTS eod_reports (
    id                TEXT PRIMARY KEY,
    report_id         TEXT NOT NULL,
    report_date       TEXT NOT NULL,
    total_sale        REAL NOT NULL DEFAULT 0 CHECK (total_sale >= 0),
    credit_sale       REAL NOT NULL DEFAULT 0 CHECK (credit_sale >= 0),
    bank_transfer     REAL NOT NULL DEFAULT 0 CHECK (bank_transfer >= 0),
    other_payments    REAL NOT NULL DEFAULT 0 CHECK (other_payments >= 0),
    expected_cash     REAL NOT NULL DEFAULT 0,
    counter_cash      REAL NOT NULL DEFAULT 0 CHECK (counter_cash >= 0),
    difference        REAL NOT NULL DEFAULT 0,
    submitted_by      TEXT NOT NULL,
    submitted_by_role TEXT NOT NULL,
    notes             TEXT NOT NULL DEFAULT '',
    is_deleted        INTEGER NOT NULL DEFAULT 0,
    deleted_at        TEXT,
    deleted_by        TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
) STRICT;

-- Partial unique index ensures only one non-deleted active report per calendar date:
CREATE UNIQUE INDEX IF NOT EXISTS idx_eod_reports_date_active 
ON eod_reports (report_date) WHERE is_deleted = 0;
```

### B. Table `report_expenses` (Single Source of Truth for Expenses)
```sql
CREATE TABLE IF NOT EXISTS report_expenses (
    id          TEXT PRIMARY KEY,
    report_id   TEXT NOT NULL,
    description TEXT NOT NULL,
    amount      REAL NOT NULL DEFAULT 0 CHECK (amount >= 0),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (report_id) REFERENCES eod_reports(id) ON DELETE CASCADE
) STRICT;

CREATE INDEX IF NOT EXISTS idx_report_expenses_report_id ON report_expenses(report_id);
```

#### Dynamic Aggregation via `json_group_array`:
To eliminate data anomalies and storage bloat from denormalized JSON columns, SQLite dynamically aggregates itemized expenses in queries:
```sql
SELECT 
    r.id, r.report_id, r.report_date, r.total_sale, r.credit_sale, r.bank_transfer,
    r.other_payments, r.expected_cash, r.counter_cash, r.difference, r.submitted_by,
    r.submitted_by_role, r.notes, r.created_at, r.updated_at,
    COALESCE(
        json_group_array(
            json_object('id', e.id, 'description', e.description, 'amount', e.amount)
        ) FILTER (WHERE e.id IS NOT NULL),
        '[]'
    ) AS expenses_json
FROM eod_reports r
LEFT JOIN report_expenses e ON r.id = e.report_id
WHERE r.is_deleted = 0
GROUP BY r.id;
```

### C. Table `users`
```sql
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    role          TEXT NOT NULL,
    is_active     INTEGER NOT NULL DEFAULT 1,
    is_deleted    INTEGER NOT NULL DEFAULT 0,
    deleted_at    TEXT,
    deleted_by    TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
) STRICT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_active 
ON users (username) WHERE is_deleted = 0;
```

### D. Table `notifications`
```sql
CREATE TABLE IF NOT EXISTS notifications (
    id          TEXT PRIMARY KEY,
    report_id   TEXT NOT NULL UNIQUE,
    report_date TEXT NOT NULL,
    notes       TEXT NOT NULL,
    is_read     INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
) STRICT;
```

### E. Table `audit_logs`
```sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id            TEXT PRIMARY KEY,
    entity_type   TEXT NOT NULL,
    entity_id     TEXT NOT NULL,
    action        TEXT NOT NULL,
    performed_by  TEXT NOT NULL,
    changes       TEXT NOT NULL,
    ip_address    TEXT NOT NULL DEFAULT '',
    user_agent    TEXT NOT NULL DEFAULT '',
    timestamp     TEXT NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
```

---

## 3. SQLite Production PRAGMAs

On startup, `internal/db/sqlite.go` enforces:
- `PRAGMA journal_mode = WAL;` (Concurrent readers and single-writer concurrency)
- `PRAGMA busy_timeout = 5000;` (5-second lock backoff)
- `PRAGMA foreign_keys = ON;` (Strict cascading deletes)
- `PRAGMA synchronous = NORMAL;` (WAL mode crash-safe durability)
- `PRAGMA cache_size = -64000;` (64 MB page cache)
- `PRAGMA mmap_size = 268435456;` (256 MB memory-mapped I/O)
- `PRAGMA temp_store = MEMORY;` (In-memory scratch tables)

---

## 4. Fallback MongoDB Document Schemas

When `DB_DRIVER=mongodb`, the collections `eod_reports`, `users`, `notifications`, and `audit_logs` store BSON documents:
- Primary keys map to `bson.ObjectID` via `bson:"_id,omitempty"`.
- `ObjectIDAsHexString: true` is configured in `options.BSONOptions` on connect.
- Indexes:
  - `eod_reports`: Unique index on `report_date`.
  - `users`: Unique index on `username`.
  - `notifications`: Unique index on `report_id`.

---

## 5. Bidirectional Migration Utilities

- **MongoDB ➔ SQLite (`cmd/migrate_mongo_to_sqlite`)**: Reads MongoDB collections, normalizes embedded itemized expenses into SQLite's `report_expenses` table, and verifies row integrity.
- **SQLite ➔ MongoDB (`cmd/migrate_sqlite_to_mongo`)**: Reads SQLite normalized records and executes idempotent upserts (`ReplaceOne` with `upsert: true`) against MongoDB collections for seamless rollback.
