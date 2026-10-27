package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadBuildsConfigurationFromEnvironment(t *testing.T) {
	t.Setenv("DB_HOST", "db.internal")
	t.Setenv("DB_USER", "nexus_admin")
	t.Setenv("DB_PASSWORD", "p@ss/word")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("SQS_QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/111111111111/jobs")
	t.Setenv("VALKEY_HOST", "cache.internal")
	t.Setenv("VALKEY_CACHE_TTL", "45s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := "postgres://nexus_admin:p%40ss%2Fword@db.internal:5432/nexus?sslmode=require"; cfg.DatabaseURL != want {
		t.Errorf("database URL = %q, want %q", cfg.DatabaseURL, want)
	}
	if cfg.Address != ":8080" {
		t.Errorf("address = %q, want :8080", cfg.Address)
	}
	if cfg.Valkey.Address != "cache.internal:6379" {
		t.Errorf("Valkey address = %q, want cache.internal:6379", cfg.Valkey.Address)
	}
	if cfg.Valkey.CacheTTL != 45*time.Second {
		t.Errorf("cache TTL = %s, want 45s", cfg.Valkey.CacheTTL)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("SQS_QUEUE_URL", "")
	t.Setenv("VALKEY_TLS", "sometimes")
	t.Setenv("VALKEY_CACHE_TTL", "-1s")
	t.Setenv("VALKEY_IAM_AUTH", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("expected configuration errors")
	}
	for _, want := range []string{"DB_PASSWORD", "SQS_QUEUE_URL", "VALKEY_TLS", "VALKEY_CACHE_TTL", "VALKEY_USERNAME"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}
