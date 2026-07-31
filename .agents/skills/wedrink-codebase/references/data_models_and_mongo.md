# Data Models, Indexes & MongoDB Integration Guide

This document specifies the MongoDB database structure, document schemas, indexing strategies, repository query operations, and automated demo seeding logic for **Wedrink**.

---

## 1. Database Driver & Connection Setup

- **Driver**: Modern official Go driver `go.mongodb.org/mongo-driver/v2`.
- **Connection Logic (`internal/db/mongodb.go`)**:
  - Connects using URI configured in `MONGO_URI` (supports local `mongodb://` and Atlas `mongodb+srv://`).
  - Sets connection timeout to 15 seconds.
  - Performs ping verification (`client.Ping`) on initialization.
  - Spawns background goroutine to guarantee unique index creation asynchronously.

---

## 2. Collections & Document Schemas

The database contains two collections: `eod_reports` and `users`.

### A. Collection `eod_reports`

Defined in `internal/models/report.go`:

```go
type ExpenseItem struct {
    ID          string  `json:"id" bson:"id"`
    Description string  `json:"description" bson:"description"`
    Amount      float64 `json:"amount" bson:"amount"`
}

type EODReport struct {
    ID              bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    ReportID        string        `json:"report_id" bson:"report_id"`
    ReportDate      string        `json:"report_date" bson:"report_date"` // Format: YYYY-MM-DD
    TotalSale       float64       `json:"total_sale" bson:"total_sale"`
    CreditSale      float64       `json:"credit_sale" bson:"credit_sale"`
    BankTransfer    float64       `json:"bank_transfer" bson:"bank_transfer"`
    OtherPayments   float64       `json:"other_payments" bson:"other_payments"`
    ExpectedCash    float64       `json:"expected_cash" bson:"expected_cash"`
    CounterCash     float64       `json:"counter_cash" bson:"counter_cash"`
    Difference      float64       `json:"difference" bson:"difference"`
    Expenses        []ExpenseItem `json:"expenses" bson:"expenses"`
    SubmittedBy     string        `json:"submitted_by" bson:"submitted_by"`
    SubmittedByRole string        `json:"submitted_by_role" bson:"submitted_by_role"`
    Notes           string        `json:"notes" bson:"notes"`
    CreatedAt       time.Time     `json:"created_at" bson:"created_at"`
    UpdatedAt       time.Time     `json:"updated_at" bson:"updated_at"`
}
```

#### Indexing Rules (`eod_reports`):
- `idx_unique_report_date`: Unique index on `report_date` (ascending `1`). Prevents duplicate submissions for the same calendar date.

---

### B. Collection `users`

Defined in `internal/models/user.go`:

```go
type Role string

const (
    RoleStaff      Role = "staff"
    RoleSuperAdmin Role = "manager"
)

type User struct {
    ID           bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
    Username     string        `json:"username" bson:"username"`
    PasswordHash string        `json:"-" bson:"password_hash"`
    FullName     string        `json:"full_name" bson:"full_name"`
    Role         Role          `json:"role" bson:"role"`
    CreatedAt    time.Time     `json:"created_at" bson:"created_at"`
}
```

#### Indexing Rules (`users`):
- `idx_unique_username`: Unique index on `username` (ascending `1`).

---

## 3. Repository Query Patterns (`internal/repository/`)

### `ReportRepository` (`report_repo.go`)

- **`FindByDate(ctx, dateStr)`**: Matches `report_date: dateStr`.
- **`FindByMonth(ctx, yearMonth)`**: Regex match `report_date: "^2026-07"`. Sorted by `report_date` ascending.
- **`FindByDateRange(ctx, startDate, endDate)`**: Queries `report_date` with `$gte` and `$lte`.
- **`CalculateMonthlySummary(ctx, yearMonth)`**: Uses **MongoDB Aggregation Pipeline**:
  - `$match`: Filters by regex `^yearMonth`.
  - `$group`: Sums `total_sale`, `credit_sale`, `bank_transfer`, `other_payments`, `expected_cash`, `counter_cash`, `difference`, and counts `report_count`.

---

## 4. Automatic User & Sample Data Seeding

### Default Demo Users (`UserRepository.SeedDefaultUsers`)
Ran automatically on every server boot (`main.go`):
- Checks if `staff` or `manager` exist.
- If missing, hashes passwords using `golang.org/x/crypto/bcrypt` (`bcrypt.DefaultCost`) and inserts:
  - `staff` / `staffpassword` (`RoleStaff`, Name: "Store Staff")
  - `manager` / `managerpassword` (`RoleSuperAdmin`, Name: "Store Manager")

### Sample Data CLI (`cmd/seed/main.go`)
Executed manually via `go run ./cmd/seed`:
- Generates 30 days of realistic daily EOD report data for July 2026.
- Populates realistic sales numbers, credit card split, bank transfer split, itemized expenses (Ice Delivery, Milk Supplies, Cleaning Supplies), and counter cash variances.
