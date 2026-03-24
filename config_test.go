package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("OLLAMA_URL")
	os.Unsetenv("OLLAMA_MODEL")
	os.Unsetenv("DB_MAX_CONNS")

	cfg := loadConfig("testdata/config.env")

	assert.Equal(t, "http://localhost:11434", cfg.OllamaURL)
	assert.Equal(t, "nomic-embed-text", cfg.OllamaModel)
	assert.Equal(t, 7438, cfg.ServerPort)
	assert.Equal(t, 10, cfg.DBMaxConns)
	assert.Equal(t, "", cfg.SupabaseURL)
}

func TestLoadConfig_FromFile(t *testing.T) {
	os.Unsetenv("SUPABASE_URL")
	os.Unsetenv("SUPABASE_KEY")

	cfg := loadConfig("testdata/config_full.env")

	assert.Equal(t, "https://test.supabase.co", cfg.SupabaseURL)
	assert.Equal(t, "secret_key", cfg.SupabaseKey)
	assert.Equal(t, 7438, cfg.ServerPort)
	assert.Equal(t, 10, cfg.DBMaxConns)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	// Missing file should not panic — just use defaults/env
	cfg := loadConfig("testdata/nonexistent.env")
	assert.Equal(t, "http://localhost:11434", cfg.OllamaURL)
	assert.Equal(t, 7438, cfg.ServerPort)
}
