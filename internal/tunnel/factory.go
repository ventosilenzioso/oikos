package tunnel

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

// NewFromConfig membangun manager dengan process controller nyata. Panel
// allocator dan event publisher di-inject oleh daemon agar boundary jaringan
// dan event bus tetap dapat diuji terpisah.
func NewFromConfig(cfg config.RuntimeConfig, allocator PanelAllocator, st TunnelStore, publish func(string, string)) (Manager, error) {
	if cfg.FRPBinary == "" {
		return nil, fmt.Errorf("runtime.frp_binary wajib diisi")
	}
	info, err := os.Stat(cfg.FRPBinary)
	if err != nil {
		return nil, fmt.Errorf("frpc tidak tersedia: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return nil, fmt.Errorf("frpc bukan executable: %s", cfg.FRPBinary)
	}
	if cfg.FRPConfig == "" {
		return nil, fmt.Errorf("runtime.frp_config wajib diisi")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.FRPConfig), 0750); err != nil {
		return nil, err
	}
	if allocator == nil || st == nil {
		return nil, fmt.Errorf("allocator dan store tunnel wajib diisi")
	}
	return NewFrpManager(FRPConfig{
		ServerAddr: cfg.FRPServerAddr, ServerPort: cfg.FRPServerPort, Token: cfg.FRPToken,
		ConfigPath: cfg.FRPConfig, BinaryPath: cfg.FRPBinary,
		RemotePortMin: cfg.FRPRemotePortMin, RemotePortMax: cfg.FRPRemotePortMax,
	}, allocator, st, NewOSProcessController(cfg.FRPBinary), publish), nil
}

var _ TunnelStore = (*store.DB)(nil)
