package tunnel

import (
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/config"
)

func TestFactoryRejectsMissingFrpBinary(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.FRPBinary = filepath.Join(t.TempDir(), "missing-frpc")
	if _, err := NewFromConfig(cfg.Runtime, nil, nil, nil); err == nil {
		t.Fatal("binary hilang harus ditolak")
	}
}
