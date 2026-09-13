package config

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"
)

func TestObservabilityConfigRoundTripKeepsDurationStrings(t *testing.T) {
	cfg := Default()
	path := filepath.Join(t.TempDir(), "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Observability.MetricsInterval != cfg.Observability.MetricsInterval {
		t.Fatalf("metrics interval=%v", loaded.Observability.MetricsInterval)
	}
}
