# Wedrink Repository Agent Rules & Technical Invariants

When working on the **Wedrink EOD Report System** codebase, all AI agents MUST adhere to the following architectural, financial, and coding constraints.

---

## 1. Domain & Financial Invariants

### Financial Calculations
- All monetary calculations MUST use the exact formulas established in `internal/services/report_service.go`:
  - $\text{OtherPayments} = \sum (\text{Expense Amounts})$
  - $\text{ExpectedCash} = \text{TotalSale} - \text{CreditSale} - \text{BankTransfer} - \text{OtherPayments}$
  - $\text{Difference} = \text{CounterCash} - \text{ExpectedCash}$
- All monetary float values MUST be rounded using `math.Round` prior to calculation and database persistence to prevent floating point drift.
- Currency display formatting MUST use Indian Numbering System (`en-IN` style, e.g., `1,25,000` for 125000) as rendered by template func `fmtNum` in `main.go`.

---

## 2. Architecture & Layering Rules

- **Clean Architecture Hierarchy**:
  - `cmd/server/main.go`: Entry point, DI container, HTTP routing setup.
  - `internal/handlers`: HTTP handlers (parses input, validates HTTP status, delegates to services, renders HTML via `render.Renderer`).
  - `internal/services`: Pure domain logic, financial math, workflow rules (no direct HTTP or DB calls).
  - `internal/repository`: MongoDB query operations using `go.mongodb.org/mongo-driver/v2`.
  - `internal/db`: MongoDB client connection, ping checks, index creation.
  - `internal/models`: Domain models and BSON struct tags (`EODReport`, `ExpenseItem`, `User`, `MonthlySummary`).
- **Dependencies**: Never call MongoDB directly from handlers or services; always go through `repository`.
- **Framework-Free**: Do NOT add third-party Go web frameworks (e.g., Gin, Fiber, Echo) unless explicitly requested by the user. Use Go standard library `net/http` router (`http.NewServeMux`).

---

## 3. Authentication & RBAC

- **Role Definitions**:
  - `models.RoleStaff` (`"staff"`): Can access `/`, `/submit`, `/reports/preview`, `/reports/expense-row`. Cannot view historical list (`/reports`), edit/delete, or export.
  - `models.RoleSuperAdmin` (`"manager"` / `"super_admin"`): Full permission across all endpoints, including report editing/overwriting, deletion, and CSV export.
- **Session Middleware**: Session state is passed via HTTP cookie `wedrink_session` (`username|role|fullname`).
- **HTMX Authorization Failures**: If an unauthenticated or unauthorized request comes via HTMX (`HX-Request: true`), respond with HTTP 401/403 and set header `HX-Redirect: /login` or render HTMX error fragment (do not render a full HTML redirect page).

---

## 4. Frontend & HTMX Interaction Rules

- **HTMX Dynamic Components**:
  - `/reports/preview` (POST): Returns `web/templates/components/calculation_preview.html`.
  - `/reports/expense-row` (GET): Returns `web/templates/components/expense_row.html`.
  - `/reports/detail` (GET): Returns `web/templates/report_modal.html`.
- **Static Assets**: Dynamic static asset routes MUST include `Cache-Control: no-cache, no-store, must-revalidate` headers in Go (to bypass Cloudflare static caching during dev/deploys).

---

## 5. MongoDB & BSON Constraints

- **Driver Version**: Always use `go.mongodb.org/mongo-driver/v2` (not v1).
- **Primary Key Mapping**: `EODReport.ID` uses `bson.ObjectID` with `bson:"_id,omitempty"`.
- **Unique Indexes**:
  - `eod_reports`: Unique index on `report_date`.
  - `users`: Unique index on `username`.
- **Report Overwrite**: Duplicate `report_date` submissions require `AllowOverwrite: true` set by a Manager; otherwise, return duplicate report error.

---

## 6. Code Style & Logging

- **Logging**: Use `log/slog` structured logging throughout `internal/`.
- **Testing / Seeding**: Default demo accounts (`staff` / `staffpassword`, `manager` / `managerpassword`) are auto-seeded on initial run.
