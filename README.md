# Wedrink - End of Day (EOD) Report System

A high-performance store daily reconciliation and reporting web application built with **Go**, **HTMX**, and **MongoDB (Atlas ready)**, replacing legacy Google Apps Script & Google Sheets.

---

## Features

- **Dashboard & Real-Time Physical Reconciliation**:
  - Gross Sales, Credit Card Sales, Bank Transfers, and Itemized Other Payments.
  - Expected Cash vs. Physical Counter Cash calculations.
  - Visual discrepancy status badges (`Balanced Match`, `Cash Shortage Warning`, `Cash Surplus`).
  - Interactive Date Selector with Quick Day Shortcuts (Today, Yesterday, Next Day).

- **EOD Report Submission (HTMX Driven)**:
  - Real-time live net cash & discrepancy calculation preview before saving.
  - Dynamic itemized expense row addition and removal.
  - Automatic duplicate submission prevention per date.
  - Store Manager overwrite authorization toggle.

- **Role-Based Access Control (RBAC)**:
  - **`staff` (Cashier)**: Can submit daily reports, view today's dashboard, and preview calculations. Cannot edit past locked reports or export data.
  - **`manager` (Store Manager)**: Full permissions (view all reports, edit/overwrite reports, delete reports, and perform CSV data exports).

- **Data Export & Reporting**:
  - Export all reports to CSV format.
  - Export filtered month or date range to CSV.
  - Selective CSV Export (check specific report rows to export).

- **MongoDB Atlas Integration**:
  - Direct connection to MongoDB Atlas or local MongoDB via connection string (`MONGO_URI`).
  - Auto-indexing on `report_date` (unique constraint) and `username`.
  - Automatic seeding of default demo users on initial run.

---

## Technology Stack

- **Backend**: Go (Standard Library `net/http` router + `log/slog` structured logging)
- **Database**: MongoDB (`go.mongodb.org/mongo-driver/v2`)
- **Frontend**: HTMX 2.0 + Vanilla CSS Glassmorphism + Responsive Design System
- **Authentication**: Session Cookies + `golang.org/x/crypto/bcrypt` password hashing

---

## Quick Start

### 1. Environment Setup
Copy `.env.example` to `.env` or set environment variables:

```bash
export PORT=8080
export MONGO_URI="mongodb+srv://<username>:<password>@cluster0.mongodb.net/wedrink?retryWrites=true&w=majority"
export MONGO_DB_NAME="wedrink"
export SESSION_SECRET="wedrink-secret-session-key-2026"
```

### 2. Build & Run
```bash
# Build the binary
go build -o bin/wedrink ./cmd/server

# Seed sample monthly data (Optional)
go run ./cmd/seed

# Run the server
./bin/wedrink
```

Open your browser at [http://localhost:8080](http://localhost:8080).

---

## Demo Accounts

The application automatically seeds initial demo accounts when first launched:

| Role | Username | Password | Permissions |
| --- | --- | --- | --- |
| **Store Staff** | `staff` | `staffpassword` | Submit daily EOD reports, view dashboard |
| **Store Manager** | `manager` | `managerpassword` | Full Access (Edit, Delete, Export CSV, Overwrite) |

---

## AI Agent Knowledge & Architecture Skills

This repository includes pre-built workspace agent rules and skills under `.agents/` so that AI coding assistants can navigate and work on the codebase without extensive onboarding:

- **Agent Rules**: [`.agents/AGENTS.md`](file:///home/luyce/Documents/Personal/wedrink/.agents/AGENTS.md)
- **Core Agent Skill**: [`.agents/skills/wedrink-codebase/SKILL.md`](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/SKILL.md)
- **Detailed References**:
  - [Architecture & Routing Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/architecture.md)
  - [Financial Reconciliation Math](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/eod_reconciliation_math.md)
  - [Data Models & MongoDB Guide](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/data_models_and_mongo.md)
  - [HTMX & Frontend Dynamic Components](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/htmx_and_frontend.md)
  - [Legacy Apps Script Parity](file:///home/luyce/Documents/Personal/wedrink/.agents/skills/wedrink-codebase/references/legacy_migration.md)

---

## API & HTMX Endpoints

- `GET /login` / `POST /login`: Authentication
- `GET /`: Interactive Dashboard
- `GET /submit`: EOD Report Submission Form
- `POST /reports/preview`: Live Calculation Preview
- `GET /reports/expense-row`: HTMX Dynamic Expense Row Component
- `POST /reports`: Submit & Process EOD Report
- `GET /reports`: Historical Reports List (Month & Date Range Filtered)
- `GET /reports/detail`: Itemized Report Detail Modal
- `DELETE /reports/delete`: Delete Report (Manager Only)
- `GET /export/csv`: Export Reports to CSV (`?type=all`, `?month=YYYY-MM`, or `?ids=id1,id2`)
- `GET /health`: Health Check Endpoint

