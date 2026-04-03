package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env              string
	Port             string
	LogLevel         SlogLevel
	DisableEmail     bool
	DatabaseURL      string
	JWTSecret        string
	ResendAPIKey     string
	FromEmail        string
	AdminEmail       string
	SupportEmail     string
	BaseURL          string
	ShelterAPIKeys   map[string]string
	TrustedProxyNets []*net.IPNet
	LookupRatePerMin int
	AuthRatePerMin   int
	QueryTimeout     time.Duration
	MagicLinkTTL     time.Duration
	JWTExpiry        time.Duration
	TransferExpiry   time.Duration
	MigrationsPath   string
	SeedMigrationsPath string
	DBMaxConns       int32
	DBMaxIdleConns   int32
	DBConnMaxLife    time.Duration
}

type SlogLevel string

func Load() (Config, error) {
	cfg := Config{
		Env:              getenv("APP_ENV", "development"),
		Port:             getenv("PORT", "8080"),
		LogLevel:         SlogLevel(strings.ToLower(getenv("LOG_LEVEL", "info"))),
		DisableEmail:     getenvBool("DISABLE_EMAIL", false),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		FromEmail:        os.Getenv("FROM_EMAIL"),
		AdminEmail:       os.Getenv("ADMIN_EMAIL"),
		SupportEmail:     os.Getenv("SUPPORT_EMAIL"),
		BaseURL:          strings.TrimRight(os.Getenv("BASE_URL"), "/"),
		ShelterAPIKeys:   parseKeyValueMap(os.Getenv("SHELTER_API_KEYS")),
		TrustedProxyNets: parseCIDRs(getenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32,::1/128")),
		LookupRatePerMin: getenvInt("RATE_LIMIT_LOOKUP_PER_MIN", 60),
		AuthRatePerMin:   getenvInt("RATE_LIMIT_AUTH_PER_MIN", 5),
		QueryTimeout:     5 * time.Second,
		MagicLinkTTL:     15 * time.Minute,
		JWTExpiry:        30 * 24 * time.Hour,
		TransferExpiry:   48 * time.Hour,
		MigrationsPath:   os.Getenv("MIGRATIONS_PATH"),
		SeedMigrationsPath: os.Getenv("SEED_MIGRATIONS_PATH"),
		DBMaxConns:       int32(getenvInt("DB_MAX_OPEN_CONNS", 25)),
		DBMaxIdleConns:   int32(getenvInt("DB_MAX_IDLE_CONNS", 5)),
		DBConnMaxLife:    5 * time.Minute,
	}

	missing := requiredMissing(cfg)
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if len(cfg.JWTSecret) < 32 {
		return cfg, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if !cfg.DisableEmail && cfg.ResendAPIKey == "" {
		return cfg, fmt.Errorf("missing required environment variables: RESEND_API_KEY (or set DISABLE_EMAIL=true)")
	}
	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "file://../db/migrations"
	}
	if cfg.SeedMigrationsPath == "" {
		cfg.SeedMigrationsPath = "file://../db/seeds"
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return cfg, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}

func parseKeyValueMap(raw string) map[string]string {
	result := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return result
	}
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func parseCIDRs(raw string) []*net.IPNet {
	var nets []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, network, err := net.ParseCIDR(part)
		if err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}

func requiredMissing(cfg Config) []string {
	var missing []string
	required := map[string]string{
		"DATABASE_URL": cfg.DatabaseURL,
		"JWT_SECRET":   cfg.JWTSecret,
		"FROM_EMAIL":   cfg.FromEmail,
		"ADMIN_EMAIL":  cfg.AdminEmail,
		"SUPPORT_EMAIL": cfg.SupportEmail,
		"BASE_URL":     cfg.BaseURL,
	}
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}
