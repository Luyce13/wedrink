# Financial Reconciliation & Math Specification

This guide explains the core business logic, formulas, currency formatting rules, and discrepancy classification rules used in the **Wedrink EOD Report System**.

---

## 1. Core Financial Formulas

All sales reconciliation calculations are centralized in `internal/services/report_service.go` inside the `ProcessAndSaveReport` function.

### Inputs (Store POS & Physical Counter Data)
- **$\text{TotalSale}$**: Gross daily total sales reported by POS register.
- **$\text{CreditSale}$**: Total card / digital payment processor transactions.
- **$\text{BankTransfer}$**: Total direct bank transfers or UPI payments.
- **$\text{CounterCash}$**: Physical cash counted in the store register drawer at the end of the day.
- **$\text{Expenses}$**: List of itemized store expense payouts made directly out of drawer cash.

---

### Derived Metrics

1. **Other Payments ($\text{totalExpenses}$)**:
   $$\text{OtherPayments} = \sum_{i=1}^{N} \text{Expense Amount}_i$$
   *Note: Only expenses with non-empty descriptions and amounts $> 0$ are included.*

2. **Expected Cash**:
   $$\text{ExpectedCash} = \text{TotalSale} - \text{CreditSale} - \text{BankTransfer} - \text{OtherPayments}$$

3. **Discrepancy / Cash Difference**:
   $$\text{Difference} = \text{CounterCash} - \text{ExpectedCash}$$

---

## 2. Floating-Point Precision & Rounding Rules

To eliminate floating-point representation anomalies inherent in standard 64-bit float math (e.g. `0.1 + 0.2 = 0.30000000000000004`), the system applies explicit rounding rules:

1. **Input Parsing (`parseAmount`)**:
   Commas are automatically stripped before standard string parsing (`strconv.ParseFloat`). Empty input strings default to `0.0`.

2. **Expense Item Normalization**:
   Expense amounts are converted to positive absolute values and rounded to nearest integer integer/unit float using `math.Round(math.Abs(amt))`.

3. **Metric Rounding**:
   Before calculation of expected cash and discrepancy, all input numbers are rounded:
   ```go
   totalSale = math.Round(totalSale)
   creditSale = math.Round(creditSale)
   bankTransfer = math.Round(bankTransfer)
   counterCash = math.Round(counterCash)
   ```

---

## 3. Discrepancy Classification & Visual Badging

Discrepancies indicate register imbalance and trigger visual alerts across templates (`dashboard_content.html`, `report_table.html`, `report_modal.html`):

| Condition | Status Classification | UI Visual Representation | Meaning |
| --- | --- | --- | --- |
| $\text{Difference} == 0$ | **Balanced Match** | Green Badge (`bg-emerald-500/10 text-emerald-400 border-emerald-500/30`) | Counter cash exactly matches expected net cash. |
| $\text{Difference} < 0$ | **Cash Shortage Warning** | Red Badge (`bg-rose-500/10 text-rose-400 border-rose-500/30`) | Physical counter cash is LESS than expected. Money is missing. |
| $\text{Difference} > 0$ | **Cash Surplus** | Amber / Yellow Badge (`bg-amber-500/10 text-amber-400 border-amber-500/30`) | Physical counter cash exceeds expected net cash. |

---

## 4. Currency Formatting (`fmtNum` Template Helper)

Currency values in HTML templates are rendered using the `fmtNum` custom function registered in `main.go`.

### Formatting Rule
- Formats floating point values into comma-separated groups according to the **Indian Numbering System** (`en-IN` grouping: thousands group of 3, followed by groupings of 2 digits).

### Code Logic:
```go
// fmtNum formats float64 into en-IN integer representation (e.g. 125000 -> "1,25,000")
"fmtNum": func(val float64) string {
    n := int64(math.Round(math.Abs(val)))
    s := strconv.FormatInt(n, 10)
    // Splits last 3 digits, then pairs preceding digits by 2s from right to left.
    // ...
}
```

### Examples:
- `125000` $\rightarrow$ `"1,25,000"`
- `5000` $\rightarrow$ `"5,000"`
- `500` $\rightarrow$ `"500"`
- `-1500` $\rightarrow$ `"-1,500"`

---

## 5. Duplicate Submission & Overwrite Security Logic

1. **Unique Date Constraint**:
   MongoDB enforces a unique index on `report_date`.
2. **Staff Behavior**:
   If a staff cashier attempts to submit a report for a date that already exists in `eod_reports`, `ReportService.ProcessAndSaveReport` returns an error:
   `"a report for date YYYY-MM-DD already exists. Contact a Manager to edit."`
3. **Manager Overwrite**:
   Managers can submit with `AllowOverwrite: true`. In this case, `ReportService` fetches the existing document ID, updates the fields while preserving `CreatedAt`, and performs a MongoDB `Update` (upsert behavior).
