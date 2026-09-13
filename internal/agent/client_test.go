package agent

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/oikos/oikos/internal/security"
)

func TestComputeBackoff(t *testing.T) {
	cases := map[int]time.Duration{0: time.Second, 1: 2 * time.Second, 2: 4 * time.Second, 5: 30 * time.Second, 99: 30 * time.Second}
	for attempt, want := range cases {
		if got := ComputeBackoff(attempt); got != want {
			t.Fatalf("attempt %d = %v, mau %v", attempt, got, want)
		}
	}
}

func TestDialPanelTimeout(t *testing.T) {
	dir := t.TempDir()
	caPEM, _, err := security.GenerateCA("ca")
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	priv, err := security.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "k")
	if err := security.SaveKey(keyPath, priv); err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "node-uji"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "c")
	if err := security.SaveCert(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := DialPanel(ctx, "127.0.0.1:1", certPath, keyPath, caPath); err == nil {
		t.Fatal("dial ke port mati harus error")
	}
}

func TestDialWithConfigBufconn(t *testing.T) {
	panel := newMockPanel(t)
	certPEM, key := panel.issue("node-uji")
	keyPEM, err := security.MarshalKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(panel.caPEM)
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := dialWithConfig(ctx, "passthrough:///bufnet", tlsCfg, grpc.WithContextDialer(panel.tlsDialer()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
}
