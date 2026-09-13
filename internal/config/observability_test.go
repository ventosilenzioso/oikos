package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestObservabilityDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Observability.BindAddr != "127.0.0.1:9191" {
		t.Fatal(cfg.Observability.BindAddr)
	}
	if cfg.Observability.EventRetentionDays != 30 {
		t.Fatal(cfg.Observability.EventRetentionDays)
	}
	if cfg.Observability.MetricsInterval.Duration() != 15*time.Second {
		t.Fatal(cfg.Observability.MetricsInterval)
	}
}

func TestObservabilityPartialYamlPreservesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("observability:\n  bind_addr: 127.0.0.1:9292\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Observability.BindAddr != "127.0.0.1:9292" || cfg.Observability.MetricsPath != "/metrics" {
		t.Fatalf("cfg=%+v", cfg.Observability)
	}
}
