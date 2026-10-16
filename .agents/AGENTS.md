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
- **Top-Level Navigation & SPA Tab Registration**:
  - Whenever a new top-level route or navigation tab (e.g. `/admin/audit`) is added to the application header or mobile navigation in `web/templates/layout.html`, it MUST be registered across all 4 SPA client-routing checkpoints in `web/static/js/app.js`:
    1. `prefetchRoutes()`: Add route to the pre-fetch list for background idle caching into `pageCache`.
    2. Nav click interceptor `routes` array: Add route to enable instant 0ms tab swapping via `navigateToTabInstant`.
    3. `SWIPE_VALID_ROUTES`: Add route to enable mobile swipe gesture transitions.
    4. `resolveSwipeCurrentIndex()`: Add prefix check to map the active URL path to the tab index for directional slide animations.
  - Omitting any checkpoint causes the tab to drop out of SPA navigation, resulting in full-page browser reloads and jarring screen flashes.
- **Dashboard Date Stepping & Panel DOM Isolation Invariant**:
  - Navigation header buttons (`#dash-prev-day`, `#dash-next-day`, `#dash-today-btn`, `#dp-trigger`) MUST reside in a static, unswapped container (`#dash-header`) and remain permanently mounted in the DOM.
  - Dynamic report date updates MUST isolate swaps strictly to `#dash-data-panel`, never destroying or recreating the header buttons. Replacing interactive buttons on response destroys the node between `mousedown` and `mouseup`, which causes the browser to silently swallow consecutive rapid clicks (especially over WAN / Oracle latency).
  - Rapid double-clicks on `#dash-prev-day`, `#dash-next-day`, or `#dash-today-btn` MUST have `dblclick` default action prevented via `e.preventDefault()` to stop text selection anomalies or dropped events.
- **Shared History Store & LRU Rolling Cache Invariant**:
  - The `/reports` history page embeds loaded report payloads in structured JSON (`<script id="reports-history-data" type="application/json">` and `.appended-reports-data`), which the client ingests into `window.reportHistoryStore`.
  - When navigating dates on the Dashboard, the stepper MUST check `dashDateCache` and `window.reportHistoryStore` before issuing network requests. Matching dates and known empty dates within the history window render synchronously in 0ms without server round trips.
  - Client stores MUST implement explicit LRU rolling eviction (`setRollingCache` in `app.js`):
    - `window.reportHistoryStore`: Capped at a rolling maximum of 500 reports.
    - `dashDateCache`: Capped at a rolling maximum of 150 rendered HTML date panels.
  - Whenever reports are created, edited (overwritten), or deleted, the system MUST flush all shared caches (`pageCache`, `dashDateCache`, and `window.reportHistoryStore`) in unison via events `reportSaved`, `reportDeleted`, and `reportUpdated`.
- **Static Asset Version Bumping Invariant**:
  - Because `/static/*` assets are served with immutable long-lived caching (`Cache-Control: public, max-age=31536000, immutable`), any edit to `web/static/js/app.js` or `web/static/css/style.css` MUST immediately bump the cache-busting query parameter in `web/templates/layout.html` (e.g. `/static/js/app.js?v=YYYYMMDD_vX`).

---

## 5. Database Drivers & Storage Invariants

### Multi-Driver Support
- The application supports dual database drivers selected via `DB_DRIVER` in `.env` (`sqlite` default, `mongodb` fallback).
- Repository abstractions in `internal/repository/interfaces.go` isolate domain handlers from underlying persistence.

### SQLite Constraints & Performance Invariants
- **Engine**: Pure Go `modernc.org/sqlite` (zero CGO) for cross-compilation across Linux AMD64 and ARM64.
- **Production PRAGMAs**: All connections MUST configure:
  - `journal_mode=WAL` (Write-Ahead Logging for non-blocking concurrent reads and single-writer concurrency).
  - `busy_timeout=5000` (5-second lock retry backoff).
  - `foreign_keys=ON` (strict cascading integrity).
  - `synchronous=NORMAL` (safe durability in WAL mode without full disk sync on every transaction).
  - `cache_size=-64000` (64 MB page cache).
  - `mmap_size=268435456` (256 MB memory-mapped I/O).
  - `temp_store=MEMORY`.
- **Relational Normalized Schema**:
  - `report_expenses` is the single source of truth (`FOREIGN KEY (report_id) REFERENCES eod_reports(id) ON DELETE CASCADE`).
  - No denormalized `expenses_json` columns. Reports aggregate expenses dynamically via `json_group_array(...)`.
  - Non-negative financial invariants are strictly enforced via `CHECK (col >= 0)` constraints on `STRICT` tables.
- **Fail-Safe Backup Invariant**:
  - Automated backups use atomic SQLite `VACUUM INTO`.
  - Backups MUST be verified via `PRAGMA integrity_check` returning `"ok"` prior to pruning older archives. Corrupt backups MUST be aborted immediately and never overwrite or prune older valid archives.
- **Zero-Downtime Deployment Storage Invariant**:
  - Live SQLite database files (`data/`) and backup archives (`backups/`) MUST reside in persistent shared storage (`$APP_DIR/data`, `$APP_DIR/backups`) outside ephemeral release directories, symlinked into `$RELEASE_DIR` (`ln -sfn "$APP_DIR/data" "$RELEASE_DIR/data"`). Never store database files inside standalone release paths subject to rotation or pruning.
- **Database Path Resolution & PM2 Working Directory Invariant**:
  - In production `.env`, `SQLITE_PATH` MUST be specified as an absolute path (e.g. `/home/luyce/wedrink/data/wedrink.db`). PM2 retains the initial daemon launch `cwd` across `pm2 reload` and `pm2 restart`, which causes relative paths (`data/wedrink.db`) to resolve against stale release folders.
  - `internal/config/config.go` implements an automatic fallback: if a relative `SQLITE_PATH` or `BACKUP_DIR` is not found in the process working directory, it resolves relative to the executable application root (`filepath.Dir(filepath.Dir(exe))`).
  - All PM2 reloads MUST include `--update-env` (`pm2 reload wedrink --update-env`) to ensure updated `.env` keys are ingested by the running process.
- **Audit Trail & Diff Parity Invariants**:
  - **Action Matching**: Report overwrites emit `Action: "report.overwrite"`. Financial report audit queries (`action=reports`) across both SQLite and MongoDB MUST include `'report.submit'`, `'report.overwrite'`, `'report.update'`, and `'report.delete'`.
  - **All-Events Filter**: When `action=all` (or empty), repositories MUST return all logs without filtering by a literal `"all"`.
  - **Diff Normalization (`utils.ToCleanMap`)**: Audit states may be structs, `bson.D` slice pairs, JSON strings, or maps. State parsing and diff generation MUST use `utils.ToCleanMap` to convert payloads into standard `map[string]any`. Never run standard `json.Marshal` directly on `bson.D` without conversion, as it serializes into an array of `[{"Key":"...", "Value":...}]` objects which breaks before/after diff rendering.
  - **Synchronous Audit Logging**: `AuditService.Log()` MUST execute synchronously with a bounded timeout (5 seconds) before returning. Never offload audit writes to detached background goroutines when subsequent HTTP responses or HTMX triggers immediately re-query the audit log.
- **Bidirectional Migration Parity**:
  - `cmd/migrate_mongo_to_sqlite` and `cmd/migrate_sqlite_to_mongo` provide complete bidirectional migration with nullable column handling (`sql.NullString` for `actor_id` and `resource_id`) and idempotent upserts (`ReplaceOne(..., upsert: true)`).

### MongoDB Legacy Driver (Fallback)
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
