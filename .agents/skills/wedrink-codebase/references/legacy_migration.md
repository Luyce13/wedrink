# Legacy Apps Script Parity & Migration Guide

This document compares the legacy Google Apps Script / Google Sheets system (`Code.gs` and `EOD.html`) with the modern Go implementation in **Wedrink**, highlighting feature parity, improvements, and data mapping.

---

## 1. Background & Context

The original Wedrink EOD reporting was implemented inside a Google Spreadsheet using Google Apps Script:
- **`Code.gs`**: Apps Script backend serving HTML web app via `doGet()`, writing records to Google Sheets tabs (`EOD Summary - YYYY-MM` and `EOD Expenses - YYYY-MM`).
- **`EOD.html`**: Monolithic HTML/CSS/JS single-page frontend.

### Limitations of Legacy System
1. **Google Sheets Lock-outs & Race Conditions**: Simultaneous cashier submissions led to row overwrites or quota exceptions.
2. **Lack of Security / RBAC**: Cashiers had full view access to historical spreadsheet tabs and totals.
3. **No Authentication**: Anyone with the web app URL could submit or alter report dates.
4. **Manual Formatting Hacks**: Required custom menu items (`onOpen()`, `fixAllSheetsFormatting()`) to strip `.00` decimals across sheets.

---

## 2. Structural & Architectural Comparison

| Feature | Legacy Apps Script System (`Code.gs` / `EOD.html`) | Modern Go System (`wedrink`) |
| --- | --- | --- |
| **Backend Runtime** | Google Apps Script (V8 Engine) | Go (1.22+) Compiled Binary |
| **Database Storage** | Google Sheets monthly tabs (`EOD Summary - YYYY-MM`) | MongoDB Atlas (`eod_reports` collection with indexes) |
| **Authentication** | Anonymous / None | Cookie Session Auth (`bcrypt` password hashing) |
| **Access Control** | Single view for all users | Role-Based Access Control (`staff` vs `manager`) |
| **Frontend Framework** | Vanilla JS `google.script.run` RPC callbacks | HTMX 2.0 dynamic server-rendered HTML fragments |
| **Formatting** | Custom AppScript loops `fixAllSheetsFormatting()` | Go `fmtNum` template function (3-digit commas) |
| **Duplicate Protection**| Manual row scan in Sheet | Unique Index on `report_date` in MongoDB |
| **Exporting** | Native Google Sheet download | Multi-mode streaming CSV Export (`/export/csv`) |

---

## 3. Data Schema & Field Parity Mapping

### EOD Summary Record

| Legacy Google Sheets Column (`Code.gs`) | Go Struct Field (`models.EODReport`) | BSON Field | Data Type |
| --- | --- | --- | --- |
| `Date` | `ReportDate` | `report_date` | `string` (`YYYY-MM-DD`) |
| `Total Sale` | `TotalSale` | `total_sale` | `float64` |
| `Credit Card Sale` | `CreditSale` | `credit_sale` | `float64` |
| `Bank Transfer` | `BankTransfer` | `bank_transfer` | `float64` |
| `Other Payments` | `OtherPayments` | `other_payments` | `float64` |
| `Expected Cash` | `ExpectedCash` | `expected_cash` | `float64` |
| `Counter Cash` | `CounterCash` | `counter_cash` | `float64` |
| `Difference` | `Difference` | `difference` | `float64` |
| `Report ID` | `ReportID` | `report_id` | `string` (`eod_YYYY-MM-DD_timestamp`) |

---

### EOD Expense Record

| Legacy Google Sheets Column (`Code.gs`) | Go Struct Field (`models.ExpenseItem`) | BSON Field | Data Type |
| --- | --- | --- | --- |
| `Report ID` | (Parent reference) | `expenses` array | Nested Array |
| `Description` | `Description` | `description` | `string` |
| `Amount` | `Amount` | `amount` | `float64` |

---

## 4. Migration & Maintenance Notes

- The legacy files `Code.gs` and `EOD.html` are retained in the project root solely for historical reference and parity validation.
- If data migration from old Google Sheets into MongoDB is required, a simple script can parse the CSV download from Google Sheets and post JSON payloads to `ReportService.ProcessAndSaveReport` with `AllowOverwrite: true`.
