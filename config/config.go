package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	AppEnv string
	Port   string

	DatabaseURL string
	RedisURL    string

	JWTAccessSecret  string
	JWTRefreshSecret string

	IuranAmount   int
	StartMonth    int
	StartYear     int
	SetDefaultPin string

	// BendaharaNameKeyword is used to validate OCR'd payment proof text
	// against the treasurer's name. Configurable instead of hardcoded.
	BendaharaNameKeyword string

	UploadDir string

	// --- Mail watcher (auto-confirm via SeaBank email notification) ---
	MailEnabled      bool
	MailIMAPHost     string // e.g. "imap.gmail.com:993"
	MailUsername     string
	MailAppPassword  string
	MailSenderFilter string // only process emails from this sender address
	MailPollInterval int    // seconds; fallback safety-net poll alongside IDLE

	// --- Dues reminder job ---
	ReminderEnabled bool
	ReminderDay     int // day of month (Asia/Jakarta) to send the reminder
	ReminderHour    int // hour of day (0-23, Asia/Jakarta) to send the reminder
	ReminderMinute  int // minute of hour (0-59, Asia/Jakarta) to send the reminder

	// --- Web Push (VAPID) ---
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string

	AutoMigrate bool
}

// Load reads environment variables (optionally from a .env file) and returns
// a validated Config. It fails fast (returns an error) if any required
// secret is missing, instead of silently falling back to an insecure
// default the way the original Node.js implementation did for JWT secrets.
func Load() (*Config, error) {
	// It's fine if .env doesn't exist (e.g. in production where env vars
	// are injected by the platform) - so we ignore the error here.
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		Port:                 getEnv("PORT", getEnv("API_PORT", "3001")),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTAccessSecret:      os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:     os.Getenv("JWT_REFRESH_SECRET"),
		IuranAmount:          getEnvInt("IURAN_AMOUNT", 20000),
		StartMonth:           getEnvInt("START_MONTH", 6),
		StartYear:            getEnvInt("START_YEAR", 2025),
		SetDefaultPin:        os.Getenv("SET_DEFAULT_PIN"),
		BendaharaNameKeyword: getEnv("BENDAHARA_NAME_KEYWORD", ""),
		UploadDir:            getEnv("UPLOAD_DIR", "./uploads"),

		MailEnabled:      getEnvBool("MAIL_WATCHER_ENABLED", false),
		MailIMAPHost:     getEnv("MAIL_IMAP_HOST", "imap.gmail.com:993"),
		MailUsername:     os.Getenv("MAIL_USERNAME"),
		MailAppPassword:  os.Getenv("MAIL_APP_PASSWORD"),
		MailSenderFilter: getEnv("MAIL_SENDER_FILTER", "notification@seabank.co.id"),
		MailPollInterval: getEnvInt("MAIL_POLL_INTERVAL_SECONDS", 120),

		ReminderEnabled: getEnvBool("REMINDER_ENABLED", false),
		ReminderDay:     getEnvInt("REMINDER_DAY", 5),
		ReminderHour:    getEnvInt("REMINDER_HOUR", 9),
		ReminderMinute:  getEnvInt("REMINDER_MINUTE", 0),

		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    getEnv("VAPID_SUBJECT", "mailto:admin@example.com"),

		AutoMigrate: getEnvBool("AUTO_MIGRATE", false),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var missing []string

	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	// Unlike the original Node.js code (which silently fell back to the
	// hardcoded strings "access_secret"/"refresh_secret" when these env
	// vars were missing), we treat missing JWT secrets as a fatal
	// configuration error. Shipping with a guessable default secret is a
	// real vulnerability, so we'd rather refuse to start.
	if c.JWTAccessSecret == "" {
		missing = append(missing, "JWT_ACCESS_SECRET")
	}
	if c.JWTRefreshSecret == "" {
		missing = append(missing, "JWT_REFRESH_SECRET")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	if c.MailEnabled {
		var mailMissing []string
		if c.MailUsername == "" {
			mailMissing = append(mailMissing, "MAIL_USERNAME")
		}
		if c.MailAppPassword == "" {
			mailMissing = append(mailMissing, "MAIL_APP_PASSWORD")
		}
		if len(mailMissing) > 0 {
			return fmt.Errorf("MAIL_WATCHER_ENABLED=true but missing: %v", mailMissing)
		}
	}

	if c.ReminderDay < 1 || c.ReminderDay > 28 {
		return fmt.Errorf("REMINDER_DAY must be between 1 and 28 (got %d) - stick to 28 or below so it exists in every month", c.ReminderDay)
	}
	if c.ReminderHour < 0 || c.ReminderHour > 23 {
		return fmt.Errorf("REMINDER_HOUR must be between 0 and 23 (got %d)", c.ReminderHour)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
