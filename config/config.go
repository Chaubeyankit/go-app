package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App       AppConfig
	SMTP      SMTPConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	Location  LocationConfig
}

type AppConfig struct {
	Name           string
	Env            string
	Addr           string
	URL            string
	AllowedOrigins []string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type DatabaseConfig struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type LocationConfig struct {
	APIKey string
	APIURL string
}

// fileConfig mirrors Config but with all fields as plain strings so
// json.Unmarshal can decode them without custom unmarshalling logic.
// env vars always win over file values.
type fileConfig struct {
	App struct {
		Name           string   `json:"name"`
		Env            string   `json:"env"`
		Addr           string   `json:"addr"`
		URL            string   `json:"url"`
		AllowedOrigins []string `json:"allowed_origins"`
	} `json:"app"`
	Database struct {
		DSN          string `json:"dsn"`
		MaxOpenConns string `json:"max_open_conns"`
		MaxIdleConns string `json:"max_idle_conns"`
		MaxLifetime  string `json:"max_lifetime"`
	} `json:"database"`
}

func Load() *Config {
	file := loadFile("config/config.json") // optional — missing file is fine
	return &Config{
		App: AppConfig{
			Name:           str("APP_NAME", file.App.Name, "myapp"),
			Env:            str("APP_ENV", file.App.Env, "development"),
			Addr:           str("APP_ADDR", file.App.Addr, ":8080"),
			URL:            str("APP_URL", file.App.URL, "http://localhost:8080"),
			AllowedOrigins: strSlice("APP_ALLOWED_ORIGINS", file.App.AllowedOrigins),
		},
		Database: DatabaseConfig{
			DSN:          str("DATABASE_DSN", file.Database.DSN, ""),
			MaxOpenConns: integer("DATABASE_MAX_OPEN_CONNS", file.Database.MaxOpenConns, 25),
			MaxIdleConns: integer("DATABASE_MAX_IDLE_CONNS", file.Database.MaxIdleConns, 10),
			MaxLifetime:  duration("DATABASE_MAX_LIFETIME", file.Database.MaxLifetime, 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     str("REDIS_HOST", "", "localhost:6379"),
			Password: str("REDIS_PASSWORD", "", ""),
			DB:       integer("REDIS_DB", "", 0),
		},
		JWT: JWTConfig{
			AccessSecret:  str("JWT_ACCESS_SECRET", "", ""),
			RefreshSecret: str("JWT_REFRESH_SECRET", "", ""),
			AccessTTL:     duration("JWT_ACCESS_TTL", "", 15*time.Minute),
			RefreshTTL:    duration("JWT_REFRESH_TTL", "", 7*24*time.Hour),
		},
		SMTP: SMTPConfig{
			Host:     str("SMTP_HOST", "", ""),
			Port:     integer("SMTP_PORT", "", 0),
			Username: str("SMTP_USERNAME", "", ""),
			Password: str("SMTP_PASSWORD", "", ""),
			From:     str("SMTP_FROM", "", ""),
		},
		Location: LocationConfig{
			APIKey: str("LOCATION_API_KEY", "", ""),
			APIURL: str("LOCATION_API_URL", "", ""),
		},
	}
}

// ---------------------------------------------------------------------------
// helpers — each follows the same priority: env var > file value > default
// ---------------------------------------------------------------------------

// str returns the first non-empty value in: env var → file value → fallback.
func str(envKey, fileVal, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if fileVal != "" {
		return fileVal
	}
	return fallback
}

// strSlice reads a comma-separated env var, or falls back to the file slice,
// or returns nil. Example: APP_ALLOWED_ORIGINS=http://a.com,http://b.com
func strSlice(envKey string, fileVal []string) []string {
	if v := os.Getenv(envKey); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return fileVal
}

// integer parses an int from env var or file string, falling back to def.
func integer(envKey, fileVal string, def int) int {
	if v := os.Getenv(envKey); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if fileVal != "" {
		if n, err := strconv.Atoi(fileVal); err == nil {
			return n
		}
	}
	return def
}

// duration parses a Go duration string (e.g. "15m", "7h") from env var or
// file string, falling back to def.
func duration(envKey, fileVal string, def time.Duration) time.Duration {
	if v := os.Getenv(envKey); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	if fileVal != "" {
		if d, err := time.ParseDuration(fileVal); err == nil {
			return d
		}
	}
	return def
}

// loadFile attempts to read and decode a JSON config file.
// A missing file is silently ignored — only a malformed file panics.
func loadFile(path string) fileConfig {
	var fc fileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fc // fine — use env vars and defaults
		}
		panic(fmt.Sprintf("config: read %s: %v", path, err))
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		panic(fmt.Sprintf("config: parse %s: %v", path, err))
	}
	return fc
}
