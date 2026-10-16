---
name: wedrink-codebase
description: Comprehensive domain knowledge, financial reconciliation rules, architecture, SQLite WAL & MongoDB schemas, HTMX dynamic endpoints, and backup operations for the Wedrink EOD system.
---

# Wedrink EOD System - Core Skill & Knowledge Base

This skill provides AI agents with full domain knowledge, system architecture details, financial reconciliation rules, database schemas, and HTMX interaction patterns for the **Wedrink End of Day (EOD) Report System**.

---

## 1. Executive Summary & Purpose

The **Wedrink EOD Report System** is a high-performance store daily financial reconciliation web application built in **Go (1.22+)**, **HTMX 2.0**, and a **dual-driver database architecture** (**SQLite 3 WAL engine** by default, with **MongoDB Atlas v2** as fallback).

It replaces a legacy Google Apps Script & Google Sheets setup (`Code.gs` and `EOD.html`), providing:
- High-performance, concurrent daily reconciliation with **54x–1,816x speedup** via in-process SQLite.
- Online, non-blocking atomic backups (`VACUUM INTO`) with automated `PRAGMA integrity_check` validation and retention pruning.
- Role-based security (`staff` cashiers vs. `manager` store managers).
- Dynamic HTMX-driven form calculation previews, itemized expense rows, and 0ms SPA tab swapping.
- Automated fail-safe zero-downtime deployments with shared persistent storage for SQLite.
- Multi-mode CSV data exports (Summary, Itemized Expenses, and Combined views).

---

## 2. Codebase Structure & Navigation

| Component Path | Description | Key Reference |
| --- | --- | --- |
| [`cmd/server/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/server/main.go) | Main server entry point, DI container, driver selection, background backup runner, and HTTP router. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`cmd/backup/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/backup/main.go) | Standalone CLI tool for creating, listing, verifying, and restoring SQLite backups. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`cmd/benchmark/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/benchmark/main.go) | Comparative benchmark suite testing SQLite vs MongoDB read/write/aggregation latencies. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`cmd/migrate_mongo_to_sqlite/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/migrate_mongo_to_sqlite/main.go) | Automated migration tool (Mongo Atlas -> SQLite). | [Database Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`cmd/migrate_sqlite_to_mongo/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/migrate_sqlite_to_mongo/main.go) | Automated reverse migration tool (SQLite -> Mongo Atlas). | [Database Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/backup/`](file:///home/luyce/Documents/Personal/wedrink/internal/backup/backup.go) | SQLite online atomic backup engine (`VACUUM INTO`), integrity verification, and cron scheduler. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`internal/db/`](file:///home/luyce/Documents/Personal/wedrink/internal/db/sqlite.go) | Database connection initializers (`sqlite.go` with production PRAGMAs & `mongodb.go`). | [Database Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/repository/`](file:///home/luyce/Documents/Personal/wedrink/internal/repository) | Repository abstractions (`interfaces.go`), SQLite implementations (`sqlite_*.go`), and Mongo implementations. | [Database Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/services/`](file:///home/luyce/Documents/Personal/wedrink/internal/services) | Pure business logic (`report_service.go`, `auth_service.go`, `audit_service.go`). | [Reconciliation Math](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md) |
| [`internal/handlers/`](file:///home/luyce/Documents/Personal/wedrink/internal/handlers) | HTTP handler layer (`auth_handler`, `dashboard_handler`, `report_handler`, `export_handler`, `audit_handler`). | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`web/templates/`](file:///home/luyce/Documents/Personal/wedrink/web/templates) | HTML templates and HTMX component fragments (`components/`). | [HTMX & Frontend Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md) |
| [`web/static/`](file:///home/luyce/Documents/Personal/wedrink/web/static) | Glassmorphism CSS (`css/style.css`) and client SPA logic (`js/app.js`). | [HTMX & Frontend Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md) |

---

## 3. Financial Reconciliation Core Logic

The core task of this application is reconciling register sales against counter cash:

$$\text{OtherPayments} = \sum_{i=1}^{N} \text{Expense Amount}_i$$

$$\text{Expected Cash} = \text{Total Sale} - \text{Credit Card Sale} - \text{Bank Transfer} - \text{Other Payments}$$

$$\text{Discrepancy (Difference)} = \text{Counter Cash} - \text{Expected Cash}$$

- **Status Badges**:
  - `Difference == 0`: **Balanced Match** (Green badge)
  - `Difference < 0`: **Cash Shortage Warning** (Red badge)
  - `Difference > 0`: **Cash Surplus** (Amber badge)

For full edge cases and formula implementation, see [eod_reconciliation_math.md](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md).

---

## 4. Role-Based Security Matrix (RBAC)

| User Role | Username | Permissions | Accessible Endpoints |
| --- | --- | --- | --- |
| **`staff`** | `staff` | Submit daily EOD reports, view current day dashboard, live preview calculation. Cannot edit past reports, view historical lists, delete, or export. | `GET /login`, `POST /login`, `POST /logout`, `GET /`, `GET /submit`, `POST /reports`, `POST /reports/preview`, `GET /reports/expense-row` |
| **`manager`** / **`super_admin`** | `manager` | Full system access. Edit/overwrite existing date reports, delete reports, view historical list with date range filters, export CSVs, access audit logs. | All endpoints including `GET /reports`, `GET /reports/detail`, `GET /reports/edit`, `DELETE /reports/delete`, `GET /export/csv`, `GET /admin/audit` |

---

## 5. Environment & Quick Start

```bash
# 1. Configuration (.env)
PORT=8080
DB_DRIVER=sqlite
SQLITE_PATH=data/wedrink.db
BACKUP_ENABLED=true
BACKUP_INTERVAL=24h
BACKUP_DIR=backups
BACKUP_RETENTION=7
SESSION_SECRET=wedrink-secret-session-key-2026
ENV=development

# 2. Build & Run Server
go build -o bin/wedrink ./cmd/server
go build -o bin/wedrink-backup ./cmd/backup
./bin/wedrink
```

---

## 6. Deep-Dive Reference Guides

- [Architecture & Routing Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md)
- [Financial Reconciliation & Math Specification](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md)
- [Data Models, Indexes & Database Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md)
- [HTMX & Frontend Dynamic Component Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md)
- [Legacy Apps Script Parity & Migration Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/legacy_migration.md)
