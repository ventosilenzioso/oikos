//go:build integration

package tunnel

import (
	"os"
	"testing"
)

func TestFrpPublicTunnel(t *testing.T) {
	for _, name := range []string{"OIKOS_FRPS_ADDR", "OIKOS_FRPS_PORT", "OIKOS_FRP_TOKEN", "OIKOS_FRPC_BIN"} {
		if os.Getenv(name) == "" {
			t.Skip("frp integration dilewati: " + name + " belum diset")
		}
	}
	if _, err := os.Stat(os.Getenv("OIKOS_FRPC_BIN")); err != nil {
		t.Skip("frp integration dilewati: binary frpc tidak tersedia")
	}
	t.Skip("frps integration harness membutuhkan deployment frps eksternal dan belum dijalankan pada host ini")
}
