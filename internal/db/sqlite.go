package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	DB *sql.DB
}

func ConnectSQLite(dbPath string) (*SQLiteDB, error) {
	if dbPath == "" {
		dbPath = "data/wedrink.db"
	}

	// Ensure directory exists if not an in-memory database
	if dbPath != ":memory:" && !filepath.IsAbs(dbPath) {
		dir := filepath.Dir(dbPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for sqlite db: %w", err)
			}
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Apply production-grade performance PRAGMAs (WAL mode, cache, temp store, mmap)
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = -64000;",   // 64MB memory page cache
		"PRAGMA temp_store = MEMORY;",   // Store temp tables and indexes in RAM
		"PRAGMA mmap_size = 268435456;", // Memory-map up to 256MB for read performance
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			slog.Warn("Failed to execute sqlite PRAGMA", "pragma", p, "error", err)
		}
	}

	// Pool tuning: WAL mode allows concurrent readers
	maxConns := max(4, runtime.NumCPU()*2)
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	db.SetConnMaxLifetime(1 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	sqliteDB := &SQLiteDB{DB: db}
	if err := sqliteDB.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite migration failed: %w", err)
	}

	absPath, _ := filepath.Abs(dbPath)
	var reportCount int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM eod_reports WHERE is_deleted = 0;").Scan(&reportCount)
	slog.Info("Successfully connected to SQLite database", "path", dbPath, "abs_path", absPath, "report_count", reportCount, "maxConns", maxConns)

	return sqliteDB, nil
}

func (s *SQLiteDB) Close() error {
	if s != nil && s.DB != nil {
		// Run passive checkpoint before close
		_, _ = s.DB.Exec("PRAGMA wal_checkpoint(PASSIVE);")
		return s.DB.Close()
	}
	return nil
}

func (s *SQLiteDB) migrate() error {
	schema := `
	-- 1. Users Table with Case-Insensitive Unique Active Username
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		full_name TEXT NOT NULL,
		role TEXT NOT NULL CHECK (role IN ('super_admin', 'staff')),
		is_deleted INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0, 1)),
		deleted_at TEXT,
		deleted_by TEXT,
		created_at TEXT NOT NULL
	) STRICT;

	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_active_username 
	ON users(username COLLATE NOCASE) WHERE is_deleted = 0;

	-- 2. EOD Reports Table with Strict Financial Non-Negative Invariants
	CREATE TABLE IF NOT EXISTS eod_reports (
		id TEXT PRIMARY KEY,
		report_id TEXT NOT NULL,
		report_date TEXT NOT NULL,
		total_sale REAL NOT NULL CHECK (total_sale >= 0),
		credit_sale REAL NOT NULL CHECK (credit_sale >= 0),
		bank_transfer REAL NOT NULL CHECK (bank_transfer >= 0),
		other_payments REAL NOT NULL CHECK (other_payments >= 0),
		expected_cash REAL NOT NULL,
		counter_cash REAL NOT NULL CHECK (counter_cash >= 0),
		difference REAL NOT NULL,
		submitted_by TEXT NOT NULL,
		submitted_by_role TEXT NOT NULL,
		notes TEXT NOT NULL DEFAULT '',
		is_deleted INTEGER NOT NULL DEFAULT 0 CHECK (is_deleted IN (0, 1)),
		deleted_at TEXT,
		deleted_by TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	) STRICT;

	CREATE UNIQUE INDEX IF NOT EXISTS idx_reports_active_date 
	ON eod_reports(report_date) WHERE is_deleted = 0;

	CREATE INDEX IF NOT EXISTS idx_reports_report_id ON eod_reports(report_id);
	CREATE INDEX IF NOT EXISTS idx_reports_date_desc ON eod_reports(report_date DESC);
	CREATE INDEX IF NOT EXISTS idx_reports_total_sale ON eod_reports(total_sale, report_date);
	CREATE INDEX IF NOT EXISTS idx_reports_credit_sale ON eod_reports(credit_sale, report_date);
	CREATE INDEX IF NOT EXISTS idx_reports_bank_transfer ON eod_reports(bank_transfer, report_date);
	CREATE INDEX IF NOT EXISTS idx_reports_other_payments ON eod_reports(other_payments, report_date);
	CREATE INDEX IF NOT EXISTS idx_reports_difference ON eod_reports(difference, report_date);

	-- 3. Itemized Expenses Relational Table (Cascades on Report Deletion)
	CREATE TABLE IF NOT EXISTS report_expenses (
		id TEXT PRIMARY KEY,
		report_id TEXT NOT NULL,
		description TEXT NOT NULL,
		amount REAL NOT NULL CHECK (amount >= 0),
		sort_order INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (report_id) REFERENCES eod_reports(id) ON DELETE CASCADE
	) STRICT;

	CREATE INDEX IF NOT EXISTS idx_expenses_report_id ON report_expenses(report_id);
	CREATE INDEX IF NOT EXISTS idx_expenses_desc ON report_expenses(description COLLATE NOCASE);

	-- 4. Notifications Table (Store remarks alerts)
	CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		report_id TEXT NOT NULL,
		report_date TEXT NOT NULL,
		submitted_by TEXT NOT NULL,
		notes TEXT NOT NULL,
		is_read INTEGER NOT NULL DEFAULT 0 CHECK (is_read IN (0, 1)),
		created_at TEXT NOT NULL
	) STRICT;

	CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_report_id ON notifications(report_id);
	CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(is_read, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_notifications_id_desc ON notifications(id DESC);

	-- 5. Audit Logs Table (System mutations & security events)
	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		actor_id TEXT,
		actor TEXT NOT NULL,
		role TEXT NOT NULL,
		action TEXT NOT NULL,
		resource_id TEXT,
		ip_address TEXT NOT NULL,
		user_agent TEXT NOT NULL,
		old_state TEXT,
		new_state TEXT
	) STRICT;

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor);
	CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_logs(resource_id);
	CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
	CREATE INDEX IF NOT EXISTS idx_audit_id_desc ON audit_logs(id DESC);
	`

	_, err := s.DB.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema migration: %w", err)
	}

	// Drop legacy expenses_json column if present in existing databases
	_, _ = s.DB.Exec("ALTER TABLE eod_reports DROP COLUMN expenses_json;")

	return nil
}
