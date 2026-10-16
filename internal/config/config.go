package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Port            string
	DBDriver        string // "sqlite" or "mongodb" (default: "sqlite")
	SQLitePath      string // e.g. "data/wedrink.db"
	MongoURI        string
	MongoDBName     string
	SessionSecret   string
	Env             string
	AutoSeed        bool
	BackupEnabled   bool
	BackupInterval  time.Duration
	BackupDir       string
	BackupRetention int
}

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
			// Only set if not already set in OS environment
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
