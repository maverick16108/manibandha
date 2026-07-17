package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config зеркалит app/core/config.py — те же переменные окружения, чтобы Go-бэкенд
// был drop-in заменой Python-версии (та же БД, тот же SECRET_KEY, те же пути).
type Config struct {
	AppName        string
	APIPrefix      string
	SecretKey      string
	Algorithm      string
	TokenExpireMin int
	DatabaseURL    string
	UploadDir      string
	CorsOrigins    []string
	SMSCLogin      string
	SMSCPassword   string
	SMSCEnabled    bool
	SMSCodeTTLSec  int
	Port           string
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(strings.ToLower(v)); err == nil {
			return b
		}
	}
	return def
}

// Load читает .env (путь можно переопределить ENV_FILE) и переменные окружения.
func Load() *Config {
	_ = godotenv.Load(env("ENV_FILE", ".env"))

	c := &Config{
		AppName:        env("APP_NAME", "Manibandha"),
		APIPrefix:      env("API_PREFIX", "/api"),
		SecretKey:      env("SECRET_KEY", "change-me"),
		Algorithm:      env("ALGORITHM", "HS256"),
		TokenExpireMin: envInt("ACCESS_TOKEN_EXPIRE_MINUTES", 43200),
		DatabaseURL:    env("DATABASE_URL", "postgresql+psycopg2://manibandha:manibandha@localhost:5432/manibandha"),
		UploadDir:      env("UPLOAD_DIR", "uploads"),
		SMSCLogin:      env("SMSC_LOGIN", ""),
		SMSCPassword:   env("SMSC_PASSWORD", ""),
		SMSCEnabled:    envBool("SMSC_ENABLED", false),
		SMSCodeTTLSec:  envInt("SMS_CODE_TTL_SECONDS", 300),
		Port:           env("PORT", "8010"),
	}
	for _, o := range strings.Split(env("BACKEND_CORS_ORIGINS", "http://localhost:5173"), ",") {
		if s := strings.TrimSpace(o); s != "" {
			c.CorsOrigins = append(c.CorsOrigins, s)
		}
	}
	return c
}
