---
name: wedrink-codebase
description: Comprehensive domain knowledge, financial reconciliation rules, architecture, MongoDB schemas, HTMX dynamic endpoints, and legacy Apps Script parity for the Wedrink EOD system.
---

# Wedrink EOD System - Core Skill & Knowledge Base

This skill provides AI agents with full domain knowledge, system architecture details, financial reconciliation rules, database schemas, and HTMX interaction patterns for the **Wedrink End of Day (EOD) Report System**.

---

## 1. Executive Summary & Purpose

The **Wedrink EOD Report System** is a store daily financial reconciliation web application built in **Go (1.22+)**, **HTMX 2.0**, and **MongoDB Atlas** (using `go.mongodb.org/mongo-driver/v2`).

It replaces a legacy Google Apps Script & Google Sheets setup (`Code.gs` and `EOD.html`), providing:
- High-performance, concurrent daily reconciliation.
- Role-based security (`staff` cashiers vs. `manager` store managers).
- Dynamic HTMX-driven form calculation previews and itemized expense rows.
- Automated MongoDB indexing on `report_date` and `username`.
- Multi-mode CSV data exports (Summary, Itemized Expenses, and Combined views).

---

## 2. Codebase Structure & Navigation

| Component Path | Description | Key Reference |
| --- | --- | --- |
| [`cmd/server/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/server/main.go) | Main server entry point, DI container, HTTP router, and template `FuncMap` initialization. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`cmd/seed/main.go`](file:///home/luyce/Documents/Personal/wedrink/cmd/seed/main.go) | Dedicated CLI binary for populating sample EOD report data across months. | [Database & Seed Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/config/`](file:///home/luyce/Documents/Personal/wedrink/internal/config/config.go) | Custom `.env` file loader and fallback configuration reader. | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`internal/db/`](file:///home/luyce/Documents/Personal/wedrink/internal/db/mongodb.go) | MongoDB Atlas connection initialization, ping check, and background index setup. | [Database & Seed Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/handlers/`](file:///home/luyce/Documents/Personal/wedrink/internal/handlers) | HTTP handler layer (`auth_handler`, `dashboard_handler`, `report_handler`, `export_handler`). | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`internal/middleware/`](file:///home/luyce/Documents/Personal/wedrink/internal/middleware) | Session authentication cookie decoder (`auth.go`) and request logging middleware (`logger.go`). | [Architecture Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md) |
| [`internal/models/`](file:///home/luyce/Documents/Personal/wedrink/internal/models) | Struct definitions for `EODReport`, `ExpenseItem`, `User`, and `MonthlySummary`. | [Database & Seed Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/render/`](file:///home/luyce/Documents/Personal/wedrink/internal/render/render.go) | Go HTML template loader and component layout renderer. | [HTMX & Frontend Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md) |
| [`internal/repository/`](file:///home/luyce/Documents/Personal/wedrink/internal/repository) | MongoDB data access layer for reports (`report_repo.go`) and users (`user_repo.go`). | [Database & Seed Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md) |
| [`internal/services/`](file:///home/luyce/Documents/Personal/wedrink/internal/services) | Pure business logic (`report_service.go` and `auth_service.go`). | [Reconciliation Math](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md) |
| [`web/templates/`](file:///home/luyce/Documents/Personal/wedrink/web/templates) | HTML templates and HTMX component fragments (`components/`). | [HTMX & Frontend Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md) |
| [`web/static/`](file:///home/luyce/Documents/Personal/wedrink/web/static) | Custom Glassmorphism CSS (`css/style.css`) and client JavaScript (`js/app.js`). | [HTMX & Frontend Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md) |
| [`Code.gs` / `EOD.html`](file:///home/luyce/Documents/Personal/wedrink/Code.gs) | Legacy Google Apps Script backend and single-page frontend. | [Legacy Migration Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/legacy_migration.md) |

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
| **`manager`** / **`super_admin`** | `manager` | Full system access. Edit/overwrite existing date reports, delete reports, view historical list with date range filters, export CSVs. | All endpoints including `GET /reports`, `GET /reports/detail`, `GET /reports/edit`, `DELETE /reports/delete`, `GET /export/csv` |

For session serialization and middleware protection, see [architecture.md](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md).

---

## 5. Environment & Quick Start

```bash
# 1. Configuration (.env)
PORT=8080
MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=wedrink
SESSION_SECRET=wedrink-secret-session-key-2026
ENV=development

# 2. Build & Run Server
go build -o bin/wedrink ./cmd/server
./bin/wedrink

# 3. Seed Sample Data (Optional)
go run ./cmd/seed
```

---

## 6. Deep-Dive Reference Guides

For comprehensive technical specifications, open the following reference documents:
- [Architecture & Routing Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md)
- [Financial Reconciliation & Math Specification](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md)
- [Data Models, Indexes & MongoDB Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md)
- [HTMX & Frontend Dynamic Component Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md)
- [Legacy Apps Script Parity & Migration Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/legacy_migration.md)
