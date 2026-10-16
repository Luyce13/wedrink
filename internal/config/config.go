// Package config manages application configuration by loading settings from environment
// variables and discovering candidate .env files across working and executable directories.
// It enforces OS environment variable precedence, strips enclosing quotes, and provides
// intelligent executable-relative fallback resolution for relative database and backup paths.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds all runtime configuration settings for the Wedrink server and background services.
type Config struct {
	// Port is the HTTP server listening port (default: "8080").
	Port string

	// DBDriver specifies the active database driver: "sqlite" (default & recommended) or "mongodb" (fallback).
	DBDriver string

	// SQLitePath is the filesystem path to the SQLite database file (e.g. "data/wedrink.db" or absolute path).
	SQLitePath string

	// MongoURI is the MongoDB connection string (e.g. "mongodb+srv://...").
	MongoURI string

	// MongoDBName is the MongoDB database name (default: "wedrink").
	MongoDBName string

	// SessionSecret is the HMAC secret key used for signing session cookies.
	SessionSecret string

	// Env is the application runtime environment: "development" or "production".
	Env string

	// AutoSeed flags whether default demo accounts should be created on initial run.
	AutoSeed bool

	// BackupEnabled flags whether the automated SQLite backup scheduler goroutine should run.
	BackupEnabled bool

	// BackupInterval is the duration between automated backup runs (e.g. 24h).
	BackupInterval time.Duration

	// BackupDir is the destination directory where SQLite backup archives are stored.
	BackupDir string

	// BackupRetention is the maximum number of daily backup archives to retain before pruning.
	BackupRetention int
}

// LoadConfig initializes application configuration by discovering and parsing candidate
// .env files across multiple search directories (current working directory and executable
// parent directories). It maps configuration variables into a Config instance and applies
// intelligent fallback resolution for relative database and backup directory paths.
func LoadConfig() *Config {
	candidates := []string{".env", "../.env", "../../.env"}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, ".env"),
			filepath.Join(exeDir, "..", ".env"),
			filepath.Join(exeDir, "..", "..", ".env"),
		)
	}

	for _, p := range candidates {
		loadDotEnv(p)
	}

	autoSeed := getEnv("AUTO_SEED", "false")
	backupEnabled := getEnv("BACKUP_ENABLED", "true") == "true"
	backupIntervalStr := getEnv("BACKUP_INTERVAL", "24h")
	backupInterval, err := time.ParseDuration(backupIntervalStr)
	if err != nil {
		backupInterval = 24 * time.Hour
	}

	sqlitePath := getEnv("SQLITE_PATH", "data/wedrink.db")
	backupDir := getEnv("BACKUP_DIR", "data/backups")

	// If relative paths are provided and do not exist in current working directory,
	// resolve them relative to the executable's application root (bin/.. -> app root).
	if exe, err := os.Executable(); err == nil {
		appRoot := filepath.Dir(filepath.Dir(exe))
		if !filepath.IsAbs(sqlitePath) {
			if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
				candidate := filepath.Join(appRoot, sqlitePath)
				if _, err := os.Stat(candidate); err == nil {
					sqlitePath = candidate
				}
			}
		}
		if !filepath.IsAbs(backupDir) {
			if _, err := os.Stat(backupDir); os.IsNotExist(err) {
				candidate := filepath.Join(appRoot, backupDir)
				if _, err := os.Stat(candidate); err == nil {
					backupDir = candidate
				}
			}
		}
	}

	return &Config{
		Port:            getEnv("PORT", "8080"),
		DBDriver:        strings.ToLower(getEnv("DB_DRIVER", "mongodb")),
		SQLitePath:      sqlitePath,
		MongoURI:        getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName:     getEnv("MONGO_DB_NAME", "wedrink"),
		SessionSecret:   getEnv("SESSION_SECRET", "wedrink-secret-session-key-2026-change-me"),
		Env:             getEnv("ENV", "development"),
		AutoSeed:        autoSeed == "true" || autoSeed == "1",
		BackupEnabled:   backupEnabled,
		BackupInterval:  backupInterval,
		BackupDir:       backupDir,
		BackupRetention: 14,
	}
}

// loadDotEnv reads key=value pairs from the specified filepath and populates them
// into the process environment using os.Setenv if they are not already set.
// It skips empty lines and comments (starting with '#'), strips enclosing quotes
// (' or ") from values, and silently ignores non-existent files.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return // .env is optional
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip outer quotes if present
			val = strings.Trim(val, `"'`)
			// Only set if not already set in OS environment (preserves explicit CLI/PM2 environment)
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return
	}
}

// getEnv retrieves the environment variable corresponding to key. If the variable
// is unset or empty, it returns the provided fallback value.
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

