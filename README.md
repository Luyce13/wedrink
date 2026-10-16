# Wedrink - End of Day (EOD) Report System

A high-performance daily store financial reconciliation and reporting system built with **Go (1.22+)**, **HTMX 2.0**, and dual-driver database architecture (**SQLite WAL engine** by default, with **MongoDB Atlas** fallback). Replaces legacy Google Apps Script & Google Sheets with sub-millisecond responsiveness, atomic fail-safe backups, and zero-downtime deployment.

---

## 🚀 Key Highlights & Architecture

- **Dual-Driver Persistence Layer**:
  - **SQLite (Default & Recommended)**: High-speed local database powered by pure Go `modernc.org/sqlite` (zero CGO). Optimized for Linux AMD64 and ARM64 (OCI Ampere A1).
  - **MongoDB Atlas (Fallback)**: Native driver (`go.mongodb.org/mongo-driver/v2`) isolated behind clean repository interfaces for seamless rollback or cloud database requirements.
- **54x – 1,816x Performance Multiplier**:
  - In-process SQLite executes full financial transactions in **462 µs** (vs. 839,000 µs on cloud Atlas over WAN).
  - Complex monthly aggregates and cursor pagination complete in **317 µs**.
- **Automated Backup & Fail-Safe Disaster Recovery**:
  - Non-blocking online snapshots using SQLite `VACUUM INTO`.
  - Automatic `PRAGMA integrity_check` validation before pruning older archives.
  - Background cron scheduler + standalone CLI tool (`cmd/backup`).
- **Normalized Relational Schema**:
  - `report_expenses` is the single source of truth (SSOT) with `ON DELETE CASCADE`.
  - Elimination of denormalized JSON columns; reports dynamically aggregate expenses via SQLite `json_group_array`.
  - Financial non-negative invariants enforced across all 5 stack layers with `CHECK (col >= 0)` constraints.
- **Modern HTMX Frontend**:
  - 0ms client-side SPA tab transitions via prefetching and `navigateToTabInstant`.
  - Live net cash calculation previews and dynamic itemized expense rows without full page reloads.

---

## ⚡ Performance Benchmarks

Benchmark comparison between **In-Process SQLite (WAL Mode)** and **MongoDB Atlas (Cloud WAN)**:

| Operation | In-Process SQLite (WAL) | MongoDB Atlas (Cloud WAN) | Speedup Factor |
| :--- | :--- | :--- | :--- |
| **User Authentication / Fetch** | **15.2 µs** | **27.6 ms** | **1,816x faster** |
| **Monthly Financial Summary Aggregation** | **317.0 µs** | **18.7 ms** | **59x faster** |
| **Historical Reports List Query (Paginated)** | **341.0 µs** | **18.4 ms** | **54x faster** |
| **Full Report Write & Relational Expenses** | **462.0 µs** | **839.0 ms** | **1,816x faster** |

---

## 🛠️ Technology Stack

- **Backend Language**: Go 1.22+ (Standard library `net/http` router, `log/slog` structured logging)
- **Primary Database**: SQLite 3 via `modernc.org/sqlite` (Pure Go, Zero CGO, WAL mode)
- **Fallback Database**: MongoDB v2 (`go.mongodb.org/mongo-driver/v2`)
- **Frontend & Dynamic UI**: HTMX 2.0, Vanilla CSS Glassmorphism, Responsive Mobile Navigation
- **Security & Cryptography**: Session Cookies (`wedrink_session`), `golang.org/x/crypto/bcrypt` password hashing
- **Task Runner / Process Manager**: PM2, Background Backup Scheduler Goroutine

---

## 📦 Codebase Structure

```
wedrink/
├── cmd/
│   ├── backup/                 # Standalone CLI for creating, listing, verifying & restoring backups
│   ├── benchmark/              # Comparative performance benchmarks (SQLite vs MongoDB)
│   ├── migrate_mongo_to_sqlite/# Automated one-way migration tool (Atlas -> SQLite)
│   ├── seed/                   # Sample report generator for testing
│   └── server/                 # HTTP server entry point & DI container (main.go)
├── internal/
│   ├── backup/                 # Online atomic backup engine & background scheduler
│   ├── config/                 # Environment config loader (.env)
│   ├── db/                     # Database initializers (SQLite with PRAGMAs & MongoDB)
│   ├── handlers/               # HTTP handlers & HTMX fragment renderers
│   ├── middleware/             # Session auth, RBAC, and rate limiting
│   ├── models/                 # Domain models, sentinel errors, and JSON/BSON tags
│   ├── render/                 # HTML template renderer & FuncMap helpers
│   ├── repository/             # Repository implementations (SQLite & MongoDB)
│   ├── services/               # Pure business logic & financial formulas
│   └── utils/                  # Formatting, IP helpers, and structured logger
├── web/
│   ├── static/                 # CSS styling (style.css) and client SPA logic (app.js)
│   └── templates/              # HTML layout and reusable HTMX component fragments
└── .github/
    ├── dependabot.yml          # Dependabot configuration with PR grouping
    └── workflows/deploy.yml    # GitHub Actions zero-downtime deployment & release pipeline
```

---

## 🏁 Quick Start

### 1. Configuration (`.env`)

Create a `.env` file from `.env.example`:

```bash
# General
PORT=8080
ENV=development
SESSION_SECRET=wedrink-secret-session-key-2026

# Database Selection: "sqlite" (default) or "mongodb"
DB_DRIVER=sqlite

# SQLite Configuration
SQLITE_PATH=data/wedrink.db

# Backup Engine Configuration
BACKUP_ENABLED=true
BACKUP_INTERVAL=24h
BACKUP_DIR=backups
BACKUP_RETENTION=7

# MongoDB Fallback Configuration (only needed if DB_DRIVER=mongodb)
MONGO_URI=mongodb+srv://<user>:<password>@cluster0.mongodb.net/wedrink?retryWrites=true&w=majority
MONGO_DB_NAME=wedrink
```

### 2. Build & Run

```bash
# Build the server and backup CLI
go build -o bin/wedrink ./cmd/server
go build -o bin/wedrink-backup ./cmd/backup

# Run the server
./bin/wedrink
```

Open your browser at [http://localhost:8080](http://localhost:8080).

### 3. Demo Accounts

The application automatically seeds demo accounts when launched on a fresh database:

| Role | Username | Password | Permissions |
| :--- | :--- | :--- | :--- |
| **Store Staff** | `staff` | `staffpassword` | Submit daily EOD reports, view today's dashboard, preview calculations |
| **Store Manager** | `manager` | `managerpassword` | Full permissions (all historical reports, overwrite, delete, CSV export) |

---

## 🗄️ SQLite Production Engine & PRAGMAs

The SQLite engine is tuned for concurrency, non-blocking reads, and durability:

```sql
PRAGMA journal_mode = WAL;          -- Non-blocking concurrent reads and writes
PRAGMA busy_timeout = 5000;         -- 5-second lock backoff
PRAGMA foreign_keys = ON;           -- Strict cascading delete integrity
PRAGMA synchronous = NORMAL;        -- Safe durability without disk sync on every write
PRAGMA cache_size = -64000;         -- 64 MB in-memory page cache
PRAGMA mmap_size = 268435456;       -- 256 MB memory-mapped I/O
PRAGMA temp_store = MEMORY;         -- In-memory temporary tables and indexes
```

### Schema & Invariant Rules
- **Relational Integrity**: Itemized expenses are stored in `report_expenses` referencing `eod_reports(id)` with `ON DELETE CASCADE`.
- **STRICT Tables**: Enforces strict column types on SQLite 3.37+.
- **Check Constraints**: Enforces `TotalSale >= 0`, `CreditSale >= 0`, `BankTransfer >= 0`, `CounterCash >= 0`, and `Amount >= 0`.
- **Partial Unique Index**: Ensures only one active report exists per date (`UNIQUE(report_date) WHERE is_deleted = 0`).

---

## 💾 Backup & Disaster Recovery CLI

The repository includes a dedicated CLI utility (`cmd/backup`) for manual and automated operations.

```bash
# Create a verified online backup (runs PRAGMA integrity_check and prunes old archives)
go run ./cmd/backup create

# List all existing backup archives with report counts and sizes
go run ./cmd/backup list

# Verify integrity across all backups
go run ./cmd/backup verify

# Restore live database from a backup (creates a safety .bak copy before restoring)
go run ./cmd/backup restore --file="wedrink_backup_20261017_002123.db"
```

### Automated In-App Cron
When `BACKUP_ENABLED=true`, a background scheduler goroutine in `internal/backup/scheduler.go` runs every `BACKUP_INTERVAL` (e.g. 24 hours), creates atomic snapshots via `VACUUM INTO`, validates integrity, and automatically trims older archives beyond `BACKUP_RETENTION`.

---

## 🔄 Bidirectional Database Migration Tools

The system provides dedicated migration utilities for transitioning between databases in either direction with complete data integrity:

### 1. MongoDB Atlas ➔ SQLite (`cmd/migrate_mongo_to_sqlite`)
Used when moving from MongoDB Atlas to SQLite:
```bash
# Via Go directly:
go run ./cmd/migrate_mongo_to_sqlite

# Or using the pre-compiled binary on the server:
./bin/migrate_mongo_to_sqlite
```
- Transfers all users, active credentials, and roles.
- Transfers all EOD reports and normalizes embedded expenses into `report_expenses` (SSOT).
- Transfers store notifications and audit logs with microsecond timestamp fidelity.

### 2. SQLite ➔ MongoDB Atlas (`cmd/migrate_sqlite_to_mongo`)
Used when rolling back or syncing from SQLite back to MongoDB Atlas:
```bash
# Via Go directly:
go run ./cmd/migrate_sqlite_to_mongo

# Or using the pre-compiled binary on the server:
./bin/migrate_sqlite_to_mongo
```
- Reads users, reports, itemized expenses, notifications, and audit logs from SQLite.
- Performs idempotent upserts (`ReplaceOne` with `upsert: true`) into MongoDB collections.
- Enables seamless, instantaneous fallback to MongoDB without any data loss.

---

## 🚢 CI/CD & Zero-Downtime Deployment

The deployment pipeline is fully automated via GitHub Actions ([`.github/workflows/deploy.yml`](file:///home/luyce/Documents/Personal/wedrink/.github/workflows/deploy.yml)):

1. **Parallel CI Validation**: Runs `go vet`, `go test ./...`, and compiles both `bin/wedrink` and `bin/wedrink-backup` in parallel.
2. **Release Artifact Packaging**: Generates a versioned `release.tar.gz` and publishes a GitHub Release.
3. **Atomic Zero-Downtime Deploy**:
   - Deploys release into `~/wedrink/releases/<commit_sha>/`.
   - **Persistent Shared Storage Invariant**: Symlinks persistent directories `~/wedrink/data` and `~/wedrink/backups` into the active release folder:
     ```bash
     mkdir -p "$APP_DIR/data" "$APP_DIR/backups"
     ln -sfn "$APP_DIR/data" "$RELEASE_DIR/data"
     ln -sfn "$APP_DIR/backups" "$RELEASE_DIR/backups"
     ```
   - Swaps symlink `~/wedrink/current` atomically.
   - Reloads process with PM2 (`pm2 reload wedrink`).
   - Automatically prunes older release directories without touching the database or backup files.

---

## 📐 Financial Invariants & Domain Math

All calculations follow the established domain rules in `internal/services/report_service.go`:

$$\text{OtherPayments} = \sum (\text{Expense Amounts})$$

$$\text{ExpectedCash} = \text{TotalSale} - \text{CreditSale} - \text{BankTransfer} - \text{OtherPayments}$$

$$\text{Difference} = \text{CounterCash} - \text{ExpectedCash}$$

- **Status Badges**:
  - $\text{Difference} = 0$: **Balanced Match** (Green)
  - $\text{Difference} < 0$: **Cash Shortage Warning** (Red)
  - $\text{Difference} > 0$: **Cash Surplus** (Amber)
- **Rounding Rule**: All monetary values are rounded using `math.Round` before calculation and persistence to eliminate floating-point drift.
