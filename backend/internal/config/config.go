package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppEnv  string
	AppPort string
	BaseURL string
	LogLevel string

	DatabaseURL string

	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	GoogleScopes       []string

	TokenEncryptionKey []byte

	GmailPubSubTopic         string
	GmailWatchLabelIDs       []string
	GmailHistoryPollFallback bool

	FirebaseCredentialsFile string

	CORSAllowedOrigins []string

	RateLimitRPS   float64
	RateLimitBurst int
}

// Load reads configuration from the environment (optionally loading a .env file).
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnv("APP_PORT", "8080"),
		BaseURL: getEnv("APP_BASE_URL", "http://localhost:8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		JWTSecret: os.Getenv("JWT_SECRET"),

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  os.Getenv("GOOGLE_REDIRECT_URI"),
		GoogleScopes:       splitCSV(getEnv("GOOGLE_SCOPES", "https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.modify")),

		GmailPubSubTopic:         os.Getenv("GMAIL_PUBSUB_TOPIC"),
		GmailWatchLabelIDs:       splitCSV(getEnv("GMAIL_WATCH_LABEL_IDS", "INBOX")),
		GmailHistoryPollFallback: getEnvBool("GMAIL_HISTORY_POLL_FALLBACK", true),

		FirebaseCredentialsFile: getEnv("FIREBASE_CREDENTIALS_FILE", "./credentials/firebase-service-account.json"),

		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "*")),

		RateLimitRPS:   getEnvFloat("RATE_LIMIT_RPS", 20),
		RateLimitBurst: getEnvInt("RATE_LIMIT_BURST", 40),
	}

	var err error
	cfg.JWTAccessTTL, err = time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TTL: %w", err)
	}
	cfg.JWTRefreshTTL, err = time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_TTL: %w", err)
	}

	encKeyHex := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encKeyHex != "" {
		key, err := decodeHexKey(encKeyHex)
		if err != nil {
			return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY: %w", err)
		}
		cfg.TokenEncryptionKey = key
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate ensures required configuration is present.
func (c *Config) Validate() error {
	required := map[string]string{
		"DATABASE_URL": c.DatabaseURL,
		"JWT_SECRET":   c.JWTSecret,
	}
	for name, val := range required {
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if len(c.TokenEncryptionKey) != 32 {
		return fmt.Errorf("TOKEN_ENCRYPTION_KEY must be 32 bytes (64 hex characters)")
	}
	return nil
}

// IsDevelopment reports whether the app is running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development" || c.AppEnv == "dev"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
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

func getEnvInt(key string, fallback int) int {
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

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func splitCSV(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' '
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func decodeHexKey(hexKey string) ([]byte, error) {
	hexKey = strings.TrimSpace(hexKey)
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("expected 64 hex characters, got %d", len(hexKey))
	}
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		var b byte
		_, err := fmt.Sscanf(hexKey[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return nil, err
		}
		key[i] = b
	}
	return key, nil
}
