package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	Port               string
	AllowedOrigins     []string
	GoogleAppCredsJSON string
	GCSBucket          string
	GCPProjectID       string
	AnthropicAPIKey    string
	AnthropicModel     string
	JWTSecret          string
}

func Load() (*Config, error) {
	required := []string{
		"DATABASE_URL",
		"ALLOWED_ORIGINS",
		"GOOGLE_APPLICATION_CREDENTIALS_JSON",
		"GCS_BUCKET",
		"GCP_PROJECT_ID",
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_MODEL",
		"JWT_SECRET",
	}
	var missing []string
	for _, k := range required {
		if os.Getenv(k) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Port:               firstNonEmpty(os.Getenv("PORT"), "8080"),
		AllowedOrigins:     splitAndTrim(os.Getenv("ALLOWED_ORIGINS"), ","),
		GoogleAppCredsJSON: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON"),
		GCSBucket:          os.Getenv("GCS_BUCKET"),
		GCPProjectID:       os.Getenv("GCP_PROJECT_ID"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:     os.Getenv("ANTHROPIC_MODEL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
	}
	if len(cfg.AllowedOrigins) == 0 {
		return nil, errors.New("ALLOWED_ORIGINS must have at least one origin")
	}
	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
