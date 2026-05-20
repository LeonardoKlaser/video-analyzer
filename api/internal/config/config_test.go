package config

import (
	"testing"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ALLOWED_ORIGINS", "http://a,http://b")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", "Zm9v")
	t.Setenv("GCS_BUCKET", "buck")
	t.Setenv("GCP_PROJECT_ID", "proj")
	t.Setenv("ANTHROPIC_API_KEY", "sk-x")
	t.Setenv("ANTHROPIC_MODEL", "claude-sonnet-4-6")
}

func TestLoad_AllRequiredSet(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "9000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "9000" {
		t.Errorf("Port: got %q want 9000", cfg.Port)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins: got %v want 2", cfg.AllowedOrigins)
	}
	if cfg.GCSBucket != "buck" {
		t.Errorf("GCSBucket: got %q", cfg.GCSBucket)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS_JSON", "")
	t.Setenv("GCS_BUCKET", "")
	t.Setenv("GCP_PROJECT_ID", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_MODEL", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when required env vars missing")
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	setRequired(t)
	t.Setenv("PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default Port: got %q want 8080", cfg.Port)
	}
}
