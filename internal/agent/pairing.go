package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/security"
)

// PairWithToken menjalankan pairing tahap fondasi: validasi token sekali pakai,
// buat data dir, dan tulis config.yaml awal.
// Pertukaran CSR/cert mTLS penuh (PairingRequest/Response) menyusul tahap lanjutan.
func PairWithToken(token string, cfg *config.Config, configPath string) error {
	if err := security.ValidateFormat(token); err != nil {
		return fmt.Errorf("token pairing: %w", err)
	}
	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		return fmt.Errorf("buat data dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Node.CertPath), 0750); err != nil {
		return fmt.Errorf("buat dir cert: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("tulis config: %w", err)
	}
	return nil
}
