package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

type FRPConfig struct {
	ServerAddr    string
	ServerPort    int
	Token         string
	ConfigPath    string
	BinaryPath    string
	RemotePortMin int
	RemotePortMax int
}

type frpFile struct {
	ServerAddr string     `toml:"serverAddr"`
	ServerPort int        `toml:"serverPort"`
	Auth       frpAuth    `toml:"auth"`
	Proxies    []frpProxy `toml:"proxies"`
}

type frpAuth struct {
	Method string `toml:"method"`
	Token  string `toml:"token"`
}

type frpProxy struct {
	Name       string `toml:"name"`
	Type       string `toml:"type"`
	LocalIP    string `toml:"localIP"`
	LocalPort  int    `toml:"localPort"`
	RemotePort int    `toml:"remotePort"`
}

func GenerateConfig(cfg FRPConfig, mappings []PortMapping) ([]byte, error) {
	if cfg.ServerAddr == "" || cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		return nil, fmt.Errorf("server frp tidak valid")
	}
	proxies := make([]frpProxy, 0, len(mappings))
	sorted := append([]PortMapping(nil), mappings...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].TunnelID < sorted[j].TunnelID })
	for _, m := range sorted {
		if m.TunnelID == "" || m.ServerID == "" {
			return nil, fmt.Errorf("id tunnel/server wajib diisi")
		}
		if m.Protocol != "tcp" && m.Protocol != "udp" {
			return nil, fmt.Errorf("protocol %q tidak didukung", m.Protocol)
		}
		if m.LocalPort < 1 || m.LocalPort > 65535 || m.RemotePort < 1 || m.RemotePort > 65535 {
			return nil, fmt.Errorf("port tunnel %s tidak valid", m.TunnelID)
		}
		proxies = append(proxies, frpProxy{
			Name: "oikos-" + m.ServerID + "-" + m.TunnelID, Type: m.Protocol,
			LocalIP: "127.0.0.1", LocalPort: m.LocalPort, RemotePort: m.RemotePort,
		})
	}
	return toml.Marshal(frpFile{
		ServerAddr: cfg.ServerAddr, ServerPort: cfg.ServerPort,
		Auth: frpAuth{Method: "token", Token: cfg.Token}, Proxies: proxies,
	})
}

func WriteConfigAtomic(path string, data []byte) error {
	if path == "" {
		return fmt.Errorf("path config kosong")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("buat direktori config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".frpc-*.tmp")
	if err != nil {
		return fmt.Errorf("buat temporary config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("permission config: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("tulis config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("tutup config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}
