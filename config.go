package main

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for supa-brain.
type Config struct {
	SupabaseURL      string
	SupabaseKey      string
	OllamaURL        string
	OllamaModel      string
	ServerPort       int
	MemoryProject    string
	DBMaxConns       int
	DBConnectTimeout time.Duration
	OllamaTimeout    time.Duration
}

// loadConfig reads the env file at path and merges with environment variables.
// Environment variables take precedence over file values.
// Missing file is silently ignored.
func loadConfig(envFile string) Config {
	_ = godotenv.Load(envFile)

	port, _ := strconv.Atoi(getEnv("SERVER_PORT", "7438"))
	maxConns, _ := strconv.Atoi(getEnv("DB_MAX_CONNS", "10"))
	dbTimeout, _ := time.ParseDuration(getEnv("DB_CONNECT_TIMEOUT", "5s"))
	ollamaTimeout, _ := time.ParseDuration(getEnv("OLLAMA_TIMEOUT", "10s"))

	return Config{
		SupabaseURL:      getEnv("SUPABASE_URL", ""),
		SupabaseKey:      getEnv("SUPABASE_KEY", ""),
		OllamaURL:        getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:      getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		ServerPort:       port,
		MemoryProject:    getEnv("MEMORY_PROJECT", ""),
		DBMaxConns:       maxConns,
		DBConnectTimeout: dbTimeout,
		OllamaTimeout:    ollamaTimeout,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
