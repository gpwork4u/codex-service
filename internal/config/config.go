package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr string
	DataDir    string
	LogLevel   string
	LocalAuth  string
}

func Load() *Config {
	return &Config{
		ListenAddr: getEnv("CODEX_LISTEN_ADDR", ":8787"),
		DataDir:    getEnv("CODEX_DATA_DIR", defaultDataDir()),
		LogLevel:   getEnv("CODEX_LOG_LEVEL", "info"),
		LocalAuth:  os.Getenv("CODEX_LOCAL_AUTH"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex-service"
	}
	return filepath.Join(home, ".codex-service")
}
