package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port          string
	MongoURI      string
	MongoDBName   string
	SessionSecret string
	Env           string
	AutoSeed      bool
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

	return &Config{
		Port:          getEnv("PORT", "8080"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName:   getEnv("MONGO_DB_NAME", "wedrink"),
		SessionSecret: getEnv("SESSION_SECRET", "wedrink-secret-session-key-2026-change-me"),
		Env:           getEnv("ENV", "development"),
		AutoSeed:      autoSeed == "true" || autoSeed == "1",
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
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
