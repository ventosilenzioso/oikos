package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/security"
)

// mockPanel adalah Panel palsu in-process untuk test agent.
// Dua listener meniru topologi produksi: plaintext untuk tahap pairing
// (node belum punya cert), mTLS untuk heartbeat/stream.
type mockPanel struct {
	nodepb.UnimplementedNodeServiceServer
	t        *testing.T
	mu       sync.Mutex
	tokens   map[string]bool
	caPEM    []byte
	caPriv   ed25519.PrivateKey
	caCert   *x509.Certificate
	seenHB   []string
	commands []*nodepb.NodeCommand
	pairs    int
	plainLis *bufconn.Listener
	tlsLis   *bufconn.Listener
}

func newMockPanel(t *testing.T, tokens ...string) *mockPanel {
	t.Helper()
	caPEM, caPriv, err := security.GenerateCA("mock-panel-ca")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(caPEM)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	m := &mockPanel{
		t:        t,
		tokens:   map[string]bool{},
		caPEM:    caPEM,
		caPriv:   caPriv,
		caCert:   caCert,
		plainLis: bufconn.Listen(1 << 20),
		tlsLis:   bufconn.Listen(1 << 20),
	}
	for _, tok := range tokens {
		m.tokens[tok] = false
	}
	plain := grpc.NewServer()
	nodepb.RegisterNodeServiceServer(plain, m)
	srvCertPEM, srvKey := m.issue("panel.test")
	tlsSrv := grpc.NewServer(grpc.Creds(mustServerTLS(t, srvCertPEM, srvKey, caPEM)))
	nodepb.RegisterNodeServiceServer(tlsSrv, m)
	go func() {
		if err := plain.Serve(m.plainLis); err != nil {
			t.Log("mock panel plain stop:", err)
		}
	}()
	go func() {
		if err := tlsSrv.Serve(m.tlsLis); err != nil {
			t.Log("mock panel tls stop:", err)
		}
	}()
	t.Cleanup(func() { plain.Stop(); tlsSrv.Stop() })
	return m
}

func mustServerTLS(t *testing.T, certPEM []byte, key ed25519.PrivateKey, caPEM []byte) credentials.TransportCredentials {
	t.Helper()
	keyPEM, err := security.MarshalKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatal("parse CA gagal")
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	})
}

// issue menerbitkan cert (client/server) yang di-sign CA mock.
func (m *mockPanel) issue(cn string) (certPEM []byte, key ed25519.PrivateKey) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		m.t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, m.caCert, priv.Public(), m.caPriv)
	if err != nil {
		m.t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), priv
}

func (m *mockPanel) plainDialer() func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return m.plainLis.DialContext(ctx)
	}
}

func (m *mockPanel) tlsDialer() func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return m.tlsLis.DialContext(ctx)
	}
}

func (m *mockPanel) dial(t *testing.T, tlsCfg *tls.Config, dialer func(context.Context, string) (net.Conn, error)) nodepb.NodeServiceClient {
	t.Helper()
	opts := []grpc.DialOption{grpc.WithContextDialer(dialer)}
	if tlsCfg == nil {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}
	conn, err := grpc.NewClient("passthrough:///bufnet", opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return nodepb.NewNodeServiceClient(conn)
}

// nodeClient adalah koneksi plaintext (untuk tahap pairing).
func (m *mockPanel) nodeClient(t *testing.T) nodepb.NodeServiceClient {
	t.Helper()
	return m.dial(t, nil, m.plainDialer())
}

// mtlsClient adalah koneksi mTLS (untuk heartbeat/stream).
func (m *mockPanel) mtlsClient(t *testing.T) nodepb.NodeServiceClient {
	t.Helper()
	certPEM, key := m.issue("node-uji")
	keyPEM, err := security.MarshalKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(m.caPEM) {
		t.Fatal("parse CA gagal")
	}
	return m.dial(t, &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		ServerName:   "localhost",
		MinVersion:   tls.VersionTLS12,
	}, m.tlsDialer())
}

func (m *mockPanel) caPath(t *testing.T) string {
	t.Helper()
	p := t.TempDir() + "/ca.crt"
	if err := os.WriteFile(p, m.caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func (m *mockPanel) queue(cmd *nodepb.NodeCommand) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = append(m.commands, cmd)
}

func (m *mockPanel) heartbeats() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.seenHB...)
}

func (m *mockPanel) pairCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pairs
}

func (m *mockPanel) Pair(ctx context.Context, req *nodepb.PairingRequest) (*nodepb.PairingResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	used, ok := m.tokens[req.Token]
	if !ok {
		return nil, fmt.Errorf("token tidak dikenal")
	}
	if used {
		return nil, fmt.Errorf("token sudah dipakai")
	}
	block, _ := pem.Decode(req.CsrPem)
	if block == nil {
		return nil, fmt.Errorf("CSR tidak valid")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		return nil, fmt.Errorf("CSR tidak valid")
	}
	m.tokens[req.Token] = true
	m.pairs++
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, m.caCert, csr.PublicKey, m.caPriv)
	if err != nil {
		return nil, fmt.Errorf("sign cert: %w", err)
	}
	return &nodepb.PairingResponse{
		NodeId:  uuid.NewString(),
		CertPem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}, nil
}

func (m *mockPanel) Heartbeat(ctx context.Context, req *nodepb.HeartbeatRequest) (*nodepb.HeartbeatAck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seenHB = append(m.seenHB, req.NodeId)
	return &nodepb.HeartbeatAck{Ok: true}, nil
}

func (m *mockPanel) StreamCommands(req *nodepb.StreamRequest, srv nodepb.NodeService_StreamCommandsServer) error {
	m.mu.Lock()
	cmds := append([]*nodepb.NodeCommand{}, m.commands...)
	m.mu.Unlock()
	for _, c := range cmds {
		if err := srv.Send(c); err != nil {
			return err
		}
	}
	<-srv.Context().Done()
	return srv.Context().Err()
}
