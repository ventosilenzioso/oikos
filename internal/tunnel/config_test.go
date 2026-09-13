package tunnel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfigEscapesAndSorts(t *testing.T) {
	data, err := GenerateConfig(FRPConfig{ServerAddr: "frps.example", ServerPort: 7000, Token: "secret"}, []PortMapping{
		{TunnelID: "b", ServerID: "srv-b", LocalPort: 25566, RemotePort: 30002, Protocol: "udp"},
		{TunnelID: "a", ServerID: "srv-a", LocalPort: 25565, RemotePort: 30001, Protocol: "tcp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "token = 'secret'") || strings.Index(got, "oikos-srv-a-a") > strings.Index(got, "oikos-srv-b-b") {
		t.Fatalf("config tidak sesuai: %s", got)
	}
}

func TestWriteConfigAtomic0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frpc.toml")
	if err := WriteConfigAtomic(path, []byte("x = 1\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestRejectInvalidProtocol(t *testing.T) {
	if _, err := GenerateConfig(FRPConfig{ServerAddr: "frps", ServerPort: 7000}, []PortMapping{{TunnelID: "x", ServerID: "s", LocalPort: 1, RemotePort: 2, Protocol: "icmp"}}); err == nil {
		t.Fatal("protocol invalid harus ditolak")
	}
}
