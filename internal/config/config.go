package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host           string
	Port           int
	User           string
	Password       string
	Database       string
	SSL            bool
	MaxRows        int
	Timeout        time.Duration
	AllowedSchemas map[string]bool
	MaxCellChars   int
	LogLevel       string
}

func Load() (Config, error) {
	cfg := Config{
		Host:           getenv("MYSQL_MCP_HOST", "127.0.0.1"),
		Port:           getenvInt("MYSQL_MCP_PORT", 3306),
		User:           os.Getenv("MYSQL_MCP_USER"),
		Password:       os.Getenv("MYSQL_MCP_PASSWORD"),
		Database:       os.Getenv("MYSQL_MCP_DATABASE"),
		SSL:            getenvBool("MYSQL_MCP_SSL", false),
		MaxRows:        getenvInt("MYSQL_MCP_MAX_ROWS", 100),
		Timeout:        time.Duration(getenvInt("MYSQL_MCP_TIMEOUT_MS", 5000)) * time.Millisecond,
		AllowedSchemas: parseCSVSet(os.Getenv("MYSQL_MCP_ALLOWED_SCHEMAS")),
		MaxCellChars:   getenvInt("MYSQL_MCP_MAX_CELL_CHARS", 4096),
		LogLevel:       getenv("MYSQL_MCP_LOG_LEVEL", "info"),
	}

	if cfg.User == "" {
		return Config{}, errors.New("MYSQL_MCP_USER is required")
	}
	if cfg.Password == "" {
		return Config{}, errors.New("MYSQL_MCP_PASSWORD is required")
	}
	if cfg.MaxRows <= 0 {
		return Config{}, fmt.Errorf("MYSQL_MCP_MAX_ROWS must be positive")
	}
	if cfg.MaxCellChars <= 0 {
		return Config{}, fmt.Errorf("MYSQL_MCP_MAX_CELL_CHARS must be positive")
	}
	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("MYSQL_MCP_TIMEOUT_MS must be positive")
	}
	return cfg, nil
}

func (c Config) SchemaAllowed(schema string) bool {
	if len(c.AllowedSchemas) == 0 {
		return true
	}
	return c.AllowedSchemas[schema]
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func parseCSVSet(v string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = true
		}
	}
	return out
}
