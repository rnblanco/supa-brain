package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for supa-brain.
type Config struct {
	DBUrl            string // DB_URL — full PostgreSQL DSN (preferred)
	SupabaseURL      string // SUPABASE_URL — used by db:migrate to open dashboard
	SupabaseKey      string // SUPABASE_KEY — DSN fallback (backward compat)
	OllamaURL        string
	OllamaModel      string
	ServerPort       int
	MemoryProject    string
	DBMaxConns       int
	DBConnectTimeout time.Duration
	OllamaTimeout    time.Duration
}

// LoadConfig reads the env file at path and merges with environment variables.
// Environment variables take precedence over file values.
// Missing file is silently ignored.
func LoadConfig(envFile string) Config {
	_ = godotenv.Load(envFile)

	port, _ := strconv.Atoi(GetEnv("SERVER_PORT", "7438"))
	maxConns, _ := strconv.Atoi(GetEnv("DB_MAX_CONNS", "10"))
	dbTimeout, _ := time.ParseDuration(GetEnv("DB_CONNECT_TIMEOUT", "5s"))
	ollamaTimeout, _ := time.ParseDuration(GetEnv("OLLAMA_TIMEOUT", "10s"))

	return Config{
		DBUrl:            GetEnv("DB_URL", ""),
		SupabaseURL:      GetEnv("SUPABASE_URL", ""),
		SupabaseKey:      GetEnv("SUPABASE_KEY", ""),
		OllamaURL:        GetEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:      GetEnv("OLLAMA_MODEL", "nomic-embed-text"),
		ServerPort:       port,
		MemoryProject:    GetEnv("MEMORY_PROJECT", ""),
		DBMaxConns:       maxConns,
		DBConnectTimeout: dbTimeout,
		OllamaTimeout:    ollamaTimeout,
	}
}

// GetEnv retrieves an environment variable or returns a fallback value.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
