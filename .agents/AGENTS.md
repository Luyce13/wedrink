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
- Currency display formatting MUST format with comma separation after every 3 digits (e.g., `125,000` for 125000) as rendered by template func `fmtNum` in `main.go` / `utils.FormatNumber` and `formatWithCommas` in `app.js`.
- **Non-Negative Monetary Invariant**: Monetary inputs (`TotalSale`, `CreditSale`, `BankTransfer`, `CounterCash`, and expense `Amount`) MUST NEVER accept negative values. This invariant MUST be enforced across all 5 layers of the stack:
  1. **HTML Layer:** Input elements MUST specify `min="0"`.
  2. **JS Masking Layer:** `formatWithCommas` and `getRawValue` in `app.js` MUST strip minus signs (`replace(/[,-]/g, '')`).
  3. **Domain Layer:** `models/errors.go` defines `ErrNegativeAmount`.
  4. **Service Layer:** `report_service.go` checks parsed amounts and returns `ErrNegativeAmount` if `< 0`.
  5. **Repository Layer:** `report_repo.go` invokes `validateNonNegativeReport` before any MongoDB `Create`, `Update`, or `UpsertByDate` operation.

---

## 2. Architecture & Layering Rules

- **Clean Architecture Hierarchy**:
  - `cmd/server/main.go`: Entry point, DI container, HTTP routing setup.
  - `internal/handlers`: HTTP handlers (parses input, validates HTTP status, delegates to services, renders HTML via `render.Renderer`). Handlers MUST use shared helpers in `internal/handlers/htmx_helpers.go` (`renderPage`, `renderPartial`, `renderHTMXError`, `isHTMX`) rather than raw inline HTML strings or ignored template errors.
  - `internal/services`: Pure domain logic, financial math, workflow rules (no direct HTTP or DB calls).
  - `internal/repository`: MongoDB query operations using `go.mongodb.org/mongo-driver/v2`. All data mutation services MUST accept repository interface abstractions (`ReportRepo`, `UserRepo`, `NotificationRepo`) to enable mock unit testing.
  - `internal/db`: MongoDB client connection, ping checks, index creation.
  - `internal/models`: Domain models, sentinel errors (`errors.go`), and BSON struct tags (`EODReport`, `ExpenseItem`, `User`, `MonthlySummary`).
- **Dependencies**: Never call MongoDB directly from handlers or services; always go through `repository`.
- **Framework-Free**: Do NOT add third-party Go web frameworks (e.g., Gin, Fiber, Echo) unless explicitly requested by the user. Use Go standard library `net/http` router (`http.NewServeMux`).

---

## 3. Authentication & RBAC

- **Role Definitions**:
  - `models.RoleStaff` (`"staff"`): Can access `/`, `/submit`, `/reports/preview`, `/reports/expense-row`. Cannot view historical list (`/reports`), edit/delete, or export.
  - `models.RoleSuperAdmin` (`"manager"` / `"super_admin"`): Full permission across all endpoints, including report editing/overwriting, deletion, and CSV export.
- **Session Middleware**: Session state is passed via HTTP cookie `wedrink_session` (`username|role|fullname`).
- **HTMX Authorization Failures**: If an unauthenticated or unauthorized request comes via HTMX (`HX-Request: true`), respond with HTTP 401/403 and set header `HX-Redirect: /login` or render HTMX error fragment (do not render a full HTML redirect page).
- **Password-Protected Sensitive Actions**: Deleting a report row or user account MUST require current user password verification (`adminPassword`) via a glassmorphic confirmation modal prior to execution in `internal/handlers`.

---

## 4. Frontend & HTMX Interaction Rules

- **HTMX Dynamic Components**:
  - `/reports/preview` (POST): Returns `web/templates/components/calculation_preview.html`.
  - `/reports/expense-row` (GET): Returns `web/templates/components/expense_row.html`.
  - `/reports/detail` (GET): Returns `web/templates/report_modal.html`.
- **Static Assets**: Dynamic static asset routes MUST include `Cache-Control: no-cache, no-store, must-revalidate` headers in Go (to bypass Cloudflare static caching during dev/deploys).
- **HTMX Trigger Synchronization**: When triggering a client-side HTMX event via `HX-Trigger` headers (e.g. `reportSaved`), all database mutations associated with that event MUST be executed synchronously prior to returning the HTTP response. Never offload DB writes to un-awaited background goroutines if an HTMX trigger immediately re-fetches that state, as this causes a race condition resulting in "1 version behind" UI renders.
- **Form Lock State & Edit Mode**: Asynchronous status checks (e.g. `GET /reports/check-date`) must accept `isEditMode` context so existing reports opened explicitly for editing return an unlocked state (`data-status="editing"`) and remain editable.
- **Filter Bar State**: Apply and Reset filter buttons on `/reports` MUST default to `disabled` on page load and dynamically enable via JS (`updateReportsFilterState()`) only when filter/sort inputs (`startDate`, `endDate`, `sortBy`, `sortOrder`) are modified by the user.
- **Management Page Actions & Error Banners**: Deletion or management form submissions MUST redirect cleanly via `?error=...` or `?success=...` (or `HX-Redirect`). This displays top-page alert banners without breaking table layouts or modal states.
- **SPA `pageCache` & Alert Sanitization**:
  - The client SPA switcher (`pageCache` in `app.js`) MUST NEVER cache HTML strings containing `.alert-banner` elements (`sanitizeHTMLForCache`).
  - Query parameters `?success=` and `?error=` MUST be stripped via `history.replaceState` immediately on page load (`cleanAlertQueryParams`).
  - `htmx:beforeHistorySave` MUST remove alert banners from HTMX history DOM snapshots.

---

## 5. MongoDB & BSON Constraints

- **Driver Version**: Always use `go.mongodb.org/mongo-driver/v2` (not v1).
- **Primary Key Mapping**: `EODReport.ID` uses `bson.ObjectID` with `bson:"_id,omitempty"`.
- **Unique Indexes**:
  - `eod_reports`: Unique index on `report_date`.
  - `users`: Unique index on `username`.
  - `notifications`: Unique index on `report_id`.
- **Report Overwrite**: Duplicate `report_date` submissions require `AllowOverwrite: true` set by a Manager; otherwise, return duplicate report error.
- **ObjectID String Decoding**: Always set `ObjectIDAsHexString: true` in `options.BSONOptions` on `mongo.Connect` when mapping BSON ObjectID fields into Go `string` struct fields to prevent BSON decoding type errors (`decoding an object ID into a string is not supported by default`).
- **Notification Upserting**: Notifications for store remarks MUST be bound to the report's `_id` (`EODReport.ID.Hex()`). Editing a report MUST upsert the existing notification record (updating notes text, timestamp, and resetting `is_read = false`) instead of inserting duplicate rows. Clearing notes text (`notes == ""`) MUST delete the notification.

---

## 6. Code Style & Logging

- **Logging**: Use `log/slog` structured logging throughout `internal/`.
- **Testing / Seeding**: Default demo accounts (`staff` / `staffpassword`, `manager` / `managerpassword`) are auto-seeded on initial run when `AUTO_SEED=true`.

---

## 7. Developer Signature & Branding Rule

- **Developer Signature**: All web application user interfaces MUST feature Tanveer's interactive signature badge in the footer.
- **Badge Design & Behavior**:
  - Default state: Circled T Monogram `Ⓣ` styled identically to the classic copyright symbol `©` (`<svg viewBox="0 0 24 24"><circle/><path d="M8 8.5h8M12 8.5v7"/></svg>`).
  - Hover / Interactive state: Smoothly expands on hover to display: `Designed & Built by Tanveer Abbas`.
  - Light & Dark mode support: Adaptive styling using theme tokens (Cyan glow in dark mode, Brand Red glow in light mode).

---

## 8. UI, Theme & Component Layout Rules

- **2-State Theme Toggle**: Theme toggle strictly switches between Dark ↔ Light modes (`localStorage.wedrink_theme`). OS-level theme changes (`prefers-color-scheme`) MUST automatically clear `localStorage` and re-sync to the OS preference.
- **Light Theme Color Contrast**: When overriding dark component styles in `html.theme-light`, container background utilities (such as `.bg-[#0f1c35]`) MUST be mapped to light backgrounds (`#ffffff`) alongside text utility overrides (`.text-sky-400` -> `#0284c7`). Bright neon dark-mode colors must always map to deep, high-contrast shades in light mode to maintain WCAG legibility.
- **Dynamic Viewport Datepicker Alignment**: Datepicker popover panels MUST use real-time viewport boundary detection (`autoAlignPanel(panel, trigger)` in `app.js`). When `rect.left + panelWidth > window.innerWidth - 12`, the panel automatically aligns to `right: 0; left: auto; transform-origin: top right;` to prevent off-screen clipping.
- **Responsive Mobile Popovers**: Header dropdown popovers MUST use responsive positioning (`fixed left-4 right-4 top-14 sm:absolute sm:right-0 sm:top-full sm:w-80`) to prevent clipping off the left/right edges of mobile viewports (< 640px).
- **Dynamic Textarea Auto-Expansion**: Multiline textareas (e.g. Store Remarks) MUST dynamically auto-expand `scrollHeight` with a 160px (`max-h-[10rem]`) height limit, enabling vertical scrollbars thereafter.
