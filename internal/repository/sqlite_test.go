package repository

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"wedrink/internal/db"
	"wedrink/internal/models"
)

func setupTestSQLite(t *testing.T) (*db.SQLiteDB, func()) {
	t.Helper()
	sqliteDB, err := db.ConnectSQLite(":memory:")
	if err != nil {
		t.Fatalf("Failed to connect to in-memory SQLite: %v", err)
	}
	cleanup := func() {
		_ = sqliteDB.Close()
	}
	return sqliteDB, cleanup
}

func TestSQLiteUserRepository(t *testing.T) {
	sqliteDB, cleanup := setupTestSQLite(t)
	defer cleanup()

	ctx := context.Background()
	userRepo := NewSQLiteUserRepository(sqliteDB.DB)

	// 1. Seed default users
	if err := userRepo.SeedDefaultUsers(ctx); err != nil {
		t.Fatalf("Failed to seed default users: %v", err)
	}

	// 2. FindByUsername case-insensitive
	admin, err := userRepo.FindByUsername(ctx, "ADMIN")
	if err != nil || admin == nil {
		t.Fatalf("Expected to find admin with case-insensitivity, got: %v (err: %v)", admin, err)
	}
	if admin.Role != models.RoleSuperAdmin {
		t.Errorf("Expected role %s, got %s", models.RoleSuperAdmin, admin.Role)
	}

	// 3. Create duplicate user should fail
	err = userRepo.Create(ctx, &models.User{
		Username: "admin",
		FullName: "Duplicate Admin",
		Role:     models.RoleSuperAdmin,
	})
	if err == nil {
		t.Errorf("Expected duplicate username error, got nil")
	}

	// 4. Create new user
	cashier := &models.User{
		Username:     "cashier1",
		FullName:     "First Cashier",
		Role:         models.RoleStaff,
		PasswordHash: "hashed_pw",
	}
	if err := userRepo.Create(ctx, cashier); err != nil {
		t.Fatalf("Failed to create cashier: %v", err)
	}

	// 5. FindByID
	found, err := userRepo.FindByID(ctx, cashier.ID.Hex())
	if err != nil || found == nil {
		t.Fatalf("Expected to find cashier by ID, got: %v (err: %v)", found, err)
	}

	// 6. Update user
	cashier.FullName = "Senior Cashier"
	if err := userRepo.Update(ctx, cashier); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}
	updated, _ := userRepo.FindByUsername(ctx, "cashier1")
	if updated.FullName != "Senior Cashier" {
		t.Errorf("Expected updated full name 'Senior Cashier', got %s", updated.FullName)
	}

	// 7. Soft delete
	if err := userRepo.Delete(ctx, "cashier1", "admin"); err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}
	deleted, _ := userRepo.FindByUsername(ctx, "cashier1")
	if deleted != nil {
		t.Errorf("Expected deleted user to return nil from FindByUsername, got %v", deleted)
	}
}

func TestSQLiteReportRepository(t *testing.T) {
	sqliteDB, cleanup := setupTestSQLite(t)
	defer cleanup()

	ctx := context.Background()
	reportRepo := NewSQLiteReportRepository(sqliteDB.DB)

	// 1. Non-negative amount invariant test
	invalidReport := &models.EODReport{
		ReportDate: "2026-10-15",
		TotalSale:  -100,
	}
	if err := reportRepo.Create(ctx, invalidReport); err != models.ErrNegativeAmount {
		t.Errorf("Expected ErrNegativeAmount for negative TotalSale, got %v", err)
	}

	// 2. Valid report creation with expenses
	report1 := &models.EODReport{
		ReportID:      "eod_2026-10-15_123456",
		ReportDate:    "2026-10-15",
		TotalSale:     50000,
		CreditSale:    10000,
		BankTransfer:  5000,
		OtherPayments: 2000,
		ExpectedCash:  33000,
		CounterCash:   33000,
		Difference:    0,
		SubmittedBy:   "staff",
		Expenses: []models.ExpenseItem{
			{Description: "Milk", Amount: 1200},
			{Description: "Ice", Amount: 800},
		},
		Notes: "All balanced",
	}

	if err := reportRepo.Create(ctx, report1); err != nil {
		t.Fatalf("Failed to create report1: %v", err)
	}

	// 3. Duplicate report_date creation should fail
	dupReport := &models.EODReport{
		ReportDate: "2026-10-15",
		TotalSale:  60000,
	}
	if err := reportRepo.Create(ctx, dupReport); err == nil {
		t.Errorf("Expected error when submitting duplicate report for 2026-10-15, got nil")
	}

	// 4. FindByDate and FindByID
	foundDate, err := reportRepo.FindByDate(ctx, "2026-10-15")
	if err != nil || foundDate == nil {
		t.Fatalf("Failed to find report by date: %v", err)
	}
	if len(foundDate.Expenses) != 2 {
		t.Errorf("Expected 2 expenses, got %d", len(foundDate.Expenses))
	}
	if foundDate.Expenses[0].Description != "Milk" || foundDate.Expenses[0].Amount != 1200 {
		t.Errorf("Expense item mismatch: %+v", foundDate.Expenses[0])
	}

	foundID, err := reportRepo.FindByID(ctx, report1.ID.Hex())
	if err != nil || foundID == nil {
		t.Fatalf("Failed to find report by ID: %v", err)
	}

	// 5. Create second report for 2026-10-16
	report2 := &models.EODReport{
		ReportID:      "eod_2026-10-16_123457",
		ReportDate:    "2026-10-16",
		TotalSale:     80000,
		CreditSale:    20000,
		BankTransfer:  10000,
		OtherPayments: 3000,
		ExpectedCash:  47000,
		CounterCash:   46500,
		Difference:    -500, // shortage
		SubmittedBy:   "staff",
		Expenses: []models.ExpenseItem{
			{Description: "Syrup", Amount: 3000},
		},
	}
	if err := reportRepo.Create(ctx, report2); err != nil {
		t.Fatalf("Failed to create report2: %v", err)
	}

	// 6. FindByMonth & Monthly Summary
	monthlyReports, err := reportRepo.FindByMonth(ctx, "2026-10")
	if err != nil || len(monthlyReports) != 2 {
		t.Fatalf("Expected 2 reports for 2026-10, got %d (err: %v)", len(monthlyReports), err)
	}

	summary, err := reportRepo.CalculateMonthlySummary(ctx, "2026-10")
	if err != nil {
		t.Fatalf("Failed to calculate monthly summary: %v", err)
	}
	if summary.ReportCount != 2 {
		t.Errorf("Expected ReportCount 2, got %d", summary.ReportCount)
	}
	if summary.TotalSale != 130000 {
		t.Errorf("Expected TotalSale 130000, got %.0f", summary.TotalSale)
	}
	if summary.TotalDifference != -500 {
		t.Errorf("Expected TotalDifference -500, got %.0f", summary.TotalDifference)
	}

	// 7. FindWithParams (Sort by total_sale desc)
	sortedReports, err := reportRepo.FindWithParams(ctx, ReportQueryParams{
		SortBy:    "total_sale",
		SortOrder: "desc",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Failed to find reports sorted: %v", err)
	}
	if len(sortedReports) != 2 || sortedReports[0].TotalSale != 80000 {
		t.Errorf("Expected first sorted report TotalSale 80000, got %.0f", sortedReports[0].TotalSale)
	}

	// 8. Soft Delete
	if err := reportRepo.Delete(ctx, report2.ReportDate, "admin"); err != nil {
		t.Fatalf("Failed to delete report2: %v", err)
	}
	delCheck, _ := reportRepo.FindByDate(ctx, "2026-10-16")
	if delCheck != nil {
		t.Errorf("Expected deleted report to return nil, got %v", delCheck)
	}
}

func TestSQLiteNotificationRepository(t *testing.T) {
	sqliteDB, cleanup := setupTestSQLite(t)
	defer cleanup()

	ctx := context.Background()
	notifRepo := NewSQLiteNotificationRepository(sqliteDB.DB)

	notif := &models.Notification{
		ID:          bson.NewObjectID(),
		ReportID:    "rep_123",
		ReportDate:  "2026-10-15",
		SubmittedBy: "staff",
		Notes:       "Need repair on POS terminal",
		IsRead:      false,
	}

	if err := notifRepo.Create(ctx, notif); err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}

	count, err := notifRepo.GetUnreadCount(ctx)
	if err != nil || count != 1 {
		t.Fatalf("Expected unread count 1, got %d (err: %v)", count, err)
	}

	if err := notifRepo.MarkAsRead(ctx, notif.ID.Hex()); err != nil {
		t.Fatalf("Failed to mark notification as read: %v", err)
	}

	countAfter, _ := notifRepo.GetUnreadCount(ctx)
	if countAfter != 0 {
		t.Errorf("Expected unread count 0 after mark read, got %d", countAfter)
	}
}

func TestSQLiteAuditRepository(t *testing.T) {
	sqliteDB, cleanup := setupTestSQLite(t)
	defer cleanup()

	ctx := context.Background()
	auditRepo := NewSQLiteAuditRepository(sqliteDB.DB)

	log1 := &models.AuditLog{
		Timestamp:  time.Now(),
		ActorID:    "user_1",
		Actor:      "admin",
		Role:       "super_admin",
		Action:     "user.login",
		ResourceID: "user_1",
		IPAddress:  "127.0.0.1",
		UserAgent:  "Mozilla/5.0",
	}
	if err := auditRepo.Create(ctx, log1); err != nil {
		t.Fatalf("Failed to create login audit log: %v", err)
	}

	log2 := &models.AuditLog{
		Timestamp:  time.Now(),
		ActorID:    "user_1",
		Actor:      "admin",
		Role:       "super_admin",
		Action:     "report.submit",
		ResourceID: "eod_2026-10-15",
		IPAddress:  "127.0.0.1",
		UserAgent:  "Mozilla/5.0",
		NewState:   map[string]any{"total_sale": 50000},
	}
	if err := auditRepo.Create(ctx, log2); err != nil {
		t.Fatalf("Failed to create report audit log: %v", err)
	}

	stats, err := auditRepo.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get audit stats: %v", err)
	}
	if stats.TotalEvents != 2 {
		t.Errorf("Expected 2 total events, got %d", stats.TotalEvents)
	}
	if stats.SecurityEvents != 1 {
		t.Errorf("Expected 1 security event, got %d", stats.SecurityEvents)
	}
	if stats.MutationEvents != 1 {
		t.Errorf("Expected 1 mutation event, got %d", stats.MutationEvents)
	}
	if stats.UniqueActors != 1 {
		t.Errorf("Expected 1 unique actor, got %d", stats.UniqueActors)
	}
}
