package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ExpenseItem struct {
	ID          string  `json:"id" bson:"id"`
	Description string  `json:"description" bson:"description"`
	Amount      float64 `json:"amount" bson:"amount"`
}

type EODReport struct {
	ID              bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ReportID        string        `json:"report_id" bson:"report_id"`
	ReportDate      string        `json:"report_date" bson:"report_date"` // YYYY-MM-DD format
	TotalSale       float64       `json:"total_sale" bson:"total_sale"`
	CreditSale      float64       `json:"credit_sale" bson:"credit_sale"`
	BankTransfer    float64       `json:"bank_transfer" bson:"bank_transfer"`
	OtherPayments   float64       `json:"other_payments" bson:"other_payments"` // Sum of expenses
	ExpectedCash    float64       `json:"expected_cash" bson:"expected_cash"`   // TotalSale - CreditSale - BankTransfer - OtherPayments
	CounterCash     float64       `json:"counter_cash" bson:"counter_cash"`     // Actual physical cash counted
	Difference      float64       `json:"difference" bson:"difference"`         // CounterCash - ExpectedCash
	Expenses        []ExpenseItem `json:"expenses" bson:"expenses"`
	SubmittedBy     string        `json:"submitted_by" bson:"submitted_by"`
	SubmittedByRole string        `json:"submitted_by_role" bson:"submitted_by_role"`
	Notes           string        `json:"notes" bson:"notes"`
	IsDeleted       bool          `json:"is_deleted" bson:"is_deleted"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
	DeletedBy       string        `json:"deleted_by,omitempty" bson:"deleted_by,omitempty"`
	CreatedAt       time.Time     `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at" bson:"updated_at"`
}

type MonthlySummary struct {
	YearMonth       string  `json:"year_month"`
	TotalSale       float64 `json:"total_sale"`
	TotalCredit     float64 `json:"total_credit"`
	TotalBank       float64 `json:"total_bank"`
	TotalExpenses   float64 `json:"total_expenses"`
	ExpectedCash    float64 `json:"expected_cash"`
	CounterCash     float64 `json:"counter_cash"`
	TotalDifference float64 `json:"total_difference"`
	ReportCount     int     `json:"report_count"`
}
