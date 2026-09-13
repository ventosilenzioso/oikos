# Oikos Fase 1 Lanjutan (Security + Agent) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pairing token→mTLS penuh dan agent (dial, heartbeat, command stream) yang teruji melawan mock Panel.

**Architecture:** Dua tahap pairing (plaintext gRPC untuk tukar CSR→cert sekali, lalu mTLS untuk semua). Private key ED25519 tidak pernah meninggalkan node (deviasi terdokumentasi dari rencana induk yang menaruh keygen di Panel). Mock Panel in-process via bufconn untuk semua test jaringan.

**Tech Stack:** Go 1.26, `google.golang.org/grpc` (+ `test/bufconn`, `credentials/insecure`), x509/ED25519 stdlib, SQLite `nodes` table yang sudah ada di migrasi 0001.

**Spec:** `docs/superpowers/specs/2026-09-13-oikos-fase1-fondasi-design.md` + keputusan ronde ini (security+agent dulu, disetujui user).

## Global Constraints

- Module Go: `github.com/oikos/oikos`.
- Go minimum: 1.23.
- SQLite driver: `modernc.org/sqlite` (tanpa cgo).
- File key/cert ditulis dengan permission 0600.
- Proto service bersifat aditif; field `key_pem` dihapus dari `PairingResponse` karena key tidak pernah ditransfer (deviasi dari rencana induk, disengaja demi keamanan).
- Semua operasi runtime di test memakai FakeRuntime; semua test jaringan memakai bufconn (tanpa port TCP asli).

---

### Task 1: Proto service + regen + config ca_path + dep grpc

**Files:**
- Modify: `proto/node/node.proto`, `internal/config/config.go`, `internal/config/defaults.go`, `config.yaml`, `internal/config/config_test.go`, `go.mod`, `go.sum`
- Create: `gen/go/node/node.pb.go`, `gen/go/node/node_grpc.pb.go` (via make proto)

**Interfaces:**
- Consumes: tidak ada.
- Produces: package `nodepb "github.com/oikos/oikos/gen/go/node"` dengan `NodeServiceClient/NodeServiceServer`, pesan `PairingRequest/Response{node_id,cert_pem}`, `HeartbeatRequest`, `HeartbeatAck{ok,message}`, `StreamRequest{node_id}`, `NodeCommand{command_id,action,server_id,args}`; `config.PanelConfig.CAPath`.

- [ ] **Step 1: Tambah dep grpc dan tulis proto baru**

Run:

```bash
go get google.golang.org/grpc@latest
```

`proto/node/node.proto` menjadi:

```proto
syntax = "proto3";

package node;

option go_package = "github.com/oikos/oikos/gen/go/node";

message RegisterNodeRequest {
  string name = 1;
  string version = 2;
}

message HeartbeatRequest {
  string node_id = 1;
  int64 uptime_seconds = 2;
}

message HeartbeatAck {
  bool ok = 1;
  string message = 2;
}

message PairingRequest {
  string token = 1;
  string node_name = 2;
  bytes csr_pem = 3;
}

message PairingResponse {
  string node_id = 1;
  bytes cert_pem = 2;
}

message StreamRequest {
  string node_id = 1;
}

message NodeCommand {
  string command_id = 1;
  string action = 2;
  string server_id = 3;
  map<string, string> args = 4;
}

service NodeService {
  rpc Pair(PairingRequest) returns (PairingResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatAck);
  rpc StreamCommands(StreamRequest) returns (stream NodeCommand);
}
```

- [ ] **Step 2: Tambah ca_path ke config**

`internal/config/config.go`, struct PanelConfig menjadi:

```go
type PanelConfig struct {
	Address string `yaml:"address"`
	CAPath  string `yaml:"ca_path"`
}
```

`internal/config/defaults.go`, Panel default menjadi:

```go
Panel: PanelConfig{Address: "panel.example.com:9091", CAPath: "/etc/oikos/certs/ca.crt"},
```

`config.yaml`, bagian panel menjadi:

```yaml
panel:
  address: "panel.example.com:9091"
  ca_path: "/etc/oikos/certs/ca.crt"
```

Tambah ke `internal/config/config_test.go`:

```go
func TestDefaultCAPath(t *testing.T) {
	c := Default()
	if c.Panel.CAPath == "" {
		t.Fatal("default ca_path kosong")
	}
}
```

- [ ] **Step 3: Regen proto dan verifikasi**

Run:

```bash
make proto && go build ./... && go test ./internal/config/ -v
```

Expected: file `gen/go/node/node.pb.go` dan `gen/go/node/node_grpc.pb.go` ada; build sukses; 4 test config PASS.

- [ ] **Step 4: Commit**

```bash
git add proto/node/node.proto gen/go/node/ internal/config/ config.yaml go.mod go.sum
git commit -m "feat: proto NodeService dan config ca_path"
```

---

### Task 2: security/mtls.go (key, CSR, CA, TLS config)

**Files:**
- Create: `internal/security/mtls.go`, `internal/security/mtls_test.go`
- Test: `internal/security/mtls_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces: `security.GenerateKey() (ed25519.PrivateKey, error)`; `security.MarshalKeyPEM(priv) ([]byte, error)`; `security.GenerateCSR(priv, commonName) ([]byte, error)`; `security.GenerateCA(commonName) (certPEM []byte, priv ed25519.PrivateKey, err error)`; `security.SaveKey(path, priv) error`; `security.SaveCert(path, pem) error` (keduanya 0600); `security.LoadClientTLS(certPath, keyPath, caPath) (*tls.Config, error)`; `security.LoadServerTLS(certPath, keyPath, caPath) (*tls.Config, error)`.

- [ ] **Step 1: Write the failing test**

`internal/security/mtls_test.go`:

```go
package security

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestCSRSignatureValid(t *testing.T) {
	priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	csrPEM, err := GenerateCSR(priv, "node-uji")
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		t.Fatalf("blok PEM salah: %+v", block)
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := csr.CheckSignature(); err != nil {
		t.Fatal(err)
	}
	if csr.Subject.CommonName != "node-uji" {
		t.Fatalf("CN = %q", csr.Subject.CommonName)
	}
}

func TestSaveKeyCertPermission0600(t *testing.T) {
	dir := t.TempDir()
	priv, _ := GenerateKey()
	keyPath := filepath.Join(dir, "node.key")
	if err := SaveKey(keyPath, priv); err != nil {
		t.Fatal(err)
	}
	caPEM, _, err := GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "node.crt")
	if err := SaveCert(certPath, caPEM); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{keyPath, certPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Fatalf("%s mode = %o, mau 600", p, fi.Mode().Perm())
		}
	}
}

func TestLoadClientTLSRoundtrip(t *testing.T) {
	dir := t.TempDir()
	caPEM, caKey, err := GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(dir, "ca.crt")
	if err := SaveCert(caPath, caPEM); err != nil {
		t.Fatal(err)
	}
	priv, _ := GenerateKey()
	keyPath := filepath.Join(dir, "node.key")
	if err := SaveKey(keyPath, priv); err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "node.crt")
	if err := SaveCert(certPath, caPEM); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadClientTLS(certPath, keyPath, caPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Certificates) != 1 || cfg.RootCAs == nil {
		t.Fatal("tls.Config tidak lengkap")
	}
	_ = caKey
}

func TestLoadClientTLSMissingFile(t *testing.T) {
	if _, err := LoadClientTLS("/tidak/ada.crt", "/tidak/ada.key", "/tidak/ada-ca.crt"); err == nil {
		t.Fatal("harus error untuk file hilang")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/security/ -run 'TestCSR|TestSaveKey|TestLoadClient' -v`
Expected: FAIL `undefined`.

- [ ] **Step 3: Write minimal implementation**

`internal/security/mtls.go`:

```go
package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// GenerateKey membuat private key ED25519 baru.
func GenerateKey() (ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return priv, nil
}

func MarshalKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// GenerateCSR membuat certificate signing request untuk node.
func GenerateCSR(priv ed25519.PrivateKey, commonName string) ([]byte, error) {
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: commonName},
	}, priv)
	if err != nil {
		return nil, fmt.Errorf("buat CSR: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}), nil
}

// GenerateCA membuat CA self-signed (untuk test dan tooling dev).
func GenerateCA(commonName string) (certPEM []byte, priv ed25519.PrivateKey, err error) {
	priv, err = GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		return nil, nil, fmt.Errorf("buat CA: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), priv, nil
}

// SaveKey menyimpan private key dengan permission 0600.
func SaveKey(path string, priv ed25519.PrivateKey) error {
	pemBytes, err := MarshalKeyPEM(priv)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return fmt.Errorf("tulis key: %w", err)
	}
	return nil
}

// SaveCert menyimpan cert PEM dengan permission 0600.
func SaveCert(path string, pemBytes []byte) error {
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return fmt.Errorf("tulis cert: %w", err)
	}
	return nil
}

// LoadClientTLS membangun tls.Config mTLS untuk koneksi node → Panel.
func LoadClientTLS(certPath, keyPath, caPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("baca CA: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse CA gagal")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      roots,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// LoadServerTLS membangun tls.Config untuk server yang mewajibkan client cert.
func LoadServerTLS(certPath, keyPath, caPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("baca CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse CA gagal")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/security/ -v`
Expected: PASS semua (termasuk TestGenerateAndValidate lama).

- [ ] **Step 5: Commit**

```bash
git add internal/security/
git commit -m "feat: mtls key csr ca dan tls config"
```

---

### Task 3: store Node (SaveNode/GetNode)

**Files:**
- Modify: `internal/store/models.go`, `internal/store/sqlite.go`
- Create: `internal/store/nodes_test.go`
- Test: `internal/store/nodes_test.go`

**Interfaces:**
- Consumes: tabel `nodes` dari migrasi 0001 (sudah ada).
- Produces: `store.Node{ID, Name, PanelURL, CertPath, KeyPath string; PairedAt time.Time}`; `(*DB).SaveNode(Node) error`; `(*DB).GetNode() (Node, error)` (ErrNoRows bila belum pairing).

- [ ] **Step 1: Write the failing test**

`internal/store/nodes_test.go`:

```go
package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openMigrated(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNodeRoundtrip(t *testing.T) {
	db := openMigrated(t)
	if _, err := db.GetNode(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("db kosong harus ErrNoRows, dapat %v", err)
	}
	n := Node{ID: "node-1", Name: "node-uji", PanelURL: "panel.test:9091",
		CertPath: "/c/node.crt", KeyPath: "/c/node.key", PairedAt: time.Now().UTC().Truncate(time.Second)}
	if err := db.SaveNode(n); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetNode()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "node-1" || got.Name != "node-uji" || got.PanelURL != "panel.test:9091" {
		t.Fatalf("node salah: %+v", got)
	}
	if !got.PairedAt.Equal(n.PairedAt) {
		t.Fatalf("paired_at = %v, mau %v", got.PairedAt, n.PairedAt)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestNodeRoundtrip -v`
Expected: FAIL `undefined: Node`.

- [ ] **Step 3: Write minimal implementation**

`internal/store/models.go`, tambah:

```go
type Node struct {
	ID        string
	Name      string
	PanelURL  string
	CertPath  string
	KeyPath   string
	PairedAt  time.Time
}
```

Butuh import `time` di models.go. `internal/store/sqlite.go`, tambah:

```go
func (d *DB) SaveNode(n Node) error {
	_, err := d.sql.Exec(`INSERT OR REPLACE INTO nodes(id,name,panel_url,cert_path,key_path,paired_at) VALUES(?,?,?,?,?,?)`,
		n.ID, n.Name, n.PanelURL, n.CertPath, n.KeyPath, n.PairedAt.UTC().Format(time.RFC3339))
	return err
}

func (d *DB) GetNode() (Node, error) {
	var n Node
	var pairedAt string
	err := d.sql.QueryRow(`SELECT id,name,panel_url,cert_path,key_path,paired_at FROM nodes LIMIT 1`).
		Scan(&n.ID, &n.Name, &n.PanelURL, &n.CertPath, &n.KeyPath, &pairedAt)
	if err != nil {
		return Node{}, err
	}
	n.PairedAt, err = time.Parse(time.RFC3339, pairedAt)
	if err != nil {
		return Node{}, fmt.Errorf("parse paired_at: %w", err)
	}
	return n, nil
}
```

Butuh import `time` di sqlite.go.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS (termasuk test lama).

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: store node pairing"
```

---

### Task 4: Mock Panel (bufconn, test-only)

**Files:**
- Create: `internal/agent/panel_mock_test.go`
- Test: file ini sendiri adalah helper; verifikasi via `go vet` + dipakai Task 5–8.

**Interfaces:**
- Consumes: `nodepb` (Task 1), `security.GenerateCA` (Task 2).
- Produces (hanya untuk test dalam package agent): `newMockPanel(t, tokens ...string) *mockPanel`; `(*mockPanel).nodeClient(t) nodepb.NodeServiceClient` (plaintext bufconn); `(*mockPanel).mtlsClient(t) nodepb.NodeServiceClient` (mTLS bufconn, ServerName "localhost"); `(*mockPanel).heartbeats() []string`; `(*mockPanel).queue(cmd *nodepb.NodeCommand)`; `(*mockPanel).caPath() string`; `(*mockPanel).pairCount() int`.

- [ ] **Step 1: Write the mock**

`internal/agent/panel_mock_test.go`:

```go
package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
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
	lis      *bufconn.Listener
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
		t:      t,
		tokens: map[string]bool{},
		caPEM:  caPEM,
		caPriv: caPriv,
		caCert: caCert,
		lis:    bufconn.Listen(1 << 20),
	}
	for _, tok := range tokens {
		m.tokens[tok] = false
	}
	srvCertPEM, srvKey := m.issue("panel.test")
	srv := grpc.NewServer(grpc.Creds(mustServerTLS(t, srvCertPEM, srvKey, caPEM)))
	nodepb.RegisterNodeServiceServer(srv, m)
	go func() {
		if err := srv.Serve(m.lis); err != nil {
			t.Log("mock panel stop:", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })
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

// issue menandatangani cert client/server dari CSR-less public key (test-only).
func (m *mockPanel) issue(cn string) (certPEM []byte, key ed25519.PrivateKey) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		m.t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkixName(cn),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, m.caCert, key.Public(), m.caPriv)
	if err != nil {
		m.t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key
}

func (m *mockPanel) dial(t *testing.T, tlsCfg *tls.Config) nodepb.NodeServiceClient {
	t.Helper()
	opts := []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return m.lis.DialContext(ctx)
	})}
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
	return m.dial(t, nil)
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
	})
}

func (m *mockPanel) caPath(t *testing.T) string {
	t.Helper()
	p := t.TempDir() + "/ca.crt"
	if err := osWriteFile(p, m.caPEM); err != nil {
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
```

Helper kecil di file yang sama:

```go
func pkixName(cn string) pkix.Name { return pkix.Name{CommonName: cn} }

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
```

Butuh import `crypto/x509/pkix` dan `os`.

- [ ] **Step 2: Verifikasi compile**

Run: `go vet ./internal/agent/`
Expected: sukses (file `_test.go` ikut di-compile vet; Unused belum dipakai tidak masalah karena dipakai task berikut — bila vet protes unused, biarkan, task 5 langsung memakai).

- [ ] **Step 3: Commit**

```bash
git add internal/agent/panel_mock_test.go
git commit -m "test: mock panel bufconn untuk agent"
```

---

### Task 5: Pairing penuh + CLI wiring

**Files:**
- Modify: `internal/agent/pairing.go`, `cmd/oikos/main.go`
- Create: `internal/agent/pairing_panel_test.go`
- Test: `internal/agent/pairing_panel_test.go`

**Interfaces:**
- Consumes: `security` (Task 2), `store.SaveNode/GetNode` (Task 3), mock panel (Task 4).
- Produces: `agent.PairWithPanel(ctx, panelAddr, token, nodeName string, cfg *config.Config, db *store.DB, configPath string) (string, error)`; CLI `oikos install --token --panel --name --config` (panel kosong = offline seperti sebelumnya).

- [ ] **Step 1: Write the failing test**

`internal/agent/pairing_panel_test.go`:

```go
package agent

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

func testConfigDB(t *testing.T) (*config.Config, *store.DB, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Node.DataDir = filepath.Join(dir, "data")
	cfg.Node.CertPath = filepath.Join(dir, "certs", "node.crt")
	cfg.Node.KeyPath = filepath.Join(dir, "certs", "node.key")
	cfg.Panel.Address = "mock:0"
	db, err := store.Open(filepath.Join(dir, "oikos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	return cfg, db, filepath.Join(dir, "config.yaml")
}

func TestPairWithPanelOK(t *testing.T) {
	tok := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	panel := newMockPanel(t, tok)
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nodeID, err := PairWithPanel(ctx, "passthrough:///bufnet", tok, "node-uji", cfg, db, configPath, panel.nodeClient(t))
	if err != nil {
		t.Fatal(err)
	}
	if nodeID == "" {
		t.Fatal("node id kosong")
	}
	for _, p := range []string{cfg.Node.KeyPath, cfg.Node.CertPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Fatalf("%s mode = %o", p, fi.Mode().Perm())
		}
	}
	got, err := db.GetNode()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != nodeID || got.Name != "node-uji" {
		t.Fatalf("node salah: %+v", got)
	}
	if panel.pairCount() != 1 {
		t.Fatalf("pairCount = %d", panel.pairCount())
	}
}

func TestPairRejectsBadToken(t *testing.T) {
	panel := newMockPanel(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := PairWithPanel(ctx, "passthrough:///bufnet", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "n", cfg, db, configPath, panel.nodeClient(t))
	if err == nil {
		t.Fatal("harus error untuk token tak dikenal")
	}
	if _, err := db.GetNode(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("db harus tetap kosong setelah gagal")
	}
}

func TestPairRefusesWhenAlreadyPaired(t *testing.T) {
	tok1 := "1111111111111111111111111111111111111111111111111111111111111111"
	tok2 := "2222222222222222222222222222222222222222222222222222222222222222"
	panel := newMockPanel(t, tok1, tok2)
	cfg, db, configPath := testConfigDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := PairWithPanel(ctx, "passthrough:///bufnet", tok1, "n", cfg, db, configPath, panel.nodeClient(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := PairWithPanel(ctx, "passthrough:///bufnet", tok2, "n", cfg, db, configPath, panel.nodeClient(t)); err == nil {
		t.Fatal("pairing kedua harus ditolak (sudah paired)")
	}
}
```

Catatan: `PairWithPanel` menerima `nodepb.NodeServiceClient` sebagai parameter terakhir agar test bisa inject client bufconn tanpa TCP. Signature:

```go
func PairWithPanel(ctx context.Context, panelAddr, token, nodeName string, cfg *config.Config, db *store.DB, configPath string, client nodepb.NodeServiceClient) (string, error)
```

`panelAddr` dipakai hanya untuk pesan error dan field PanelURL ("mock:0" di test tidak dipakai dial). Bila client nil, fungsi dial sendiri via `grpc.NewClient(panelAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))` — ini jalur produksi yang dipakai CLI.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestPair -v`
Expected: FAIL `undefined: PairWithPanel`.

- [ ] **Step 3: Write minimal implementation**

`internal/agent/pairing.go`, tambah (import baru: context, database/sql, errors, time, grpc, insecure, nodepb, store):

```go
// PairWithPanel menjalankan pairing penuh: generate key + CSR, tukar ke Panel,
// simpan key/cert 0600, catat node di SQLite, tulis config.
// Bila client nil, koneksi plaintext dibuat ke panelAddr (jalur produksi/CLI);
// test menginject client bufconn.
func PairWithPanel(ctx context.Context, panelAddr, token, nodeName string, cfg *config.Config, db *store.DB, configPath string, client nodepb.NodeServiceClient) (string, error) {
	if err := security.ValidateFormat(token); err != nil {
		return "", fmt.Errorf("token pairing: %w", err)
	}
	if _, err := db.GetNode(); err == nil {
		return "", fmt.Errorf("node sudah paired, tolak pairing ulang")
	} else if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("cek status pairing: %w", err)
	}
	priv, err := security.GenerateKey()
	if err != nil {
		return "", err
	}
	csrPEM, err := security.GenerateCSR(priv, nodeName)
	if err != nil {
		return "", err
	}
	if client == nil {
		conn, err := grpc.NewClient(panelAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return "", fmt.Errorf("dial panel: %w", err)
		}
		defer conn.Close()
		client = nodepb.NewNodeServiceClient(conn)
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resp, err := client.Pair(callCtx, &nodepb.PairingRequest{Token: token, NodeName: nodeName, CsrPem: csrPEM})
	if err != nil {
		return "", fmt.Errorf("pair ke panel: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Node.KeyPath), 0750); err != nil {
		return "", fmt.Errorf("buat dir cert: %w", err)
	}
	if err := security.SaveKey(cfg.Node.KeyPath, priv); err != nil {
		return "", err
	}
	if err := security.SaveCert(cfg.Node.CertPath, resp.CertPem); err != nil {
		return "", err
	}
	if err := db.SaveNode(store.Node{
		ID: resp.NodeId, Name: nodeName, PanelURL: panelAddr,
		CertPath: cfg.Node.CertPath, KeyPath: cfg.Node.KeyPath,
		PairedAt: time.Now().UTC(),
	}); err != nil {
		return "", fmt.Errorf("simpan node: %w", err)
	}
	if err := writeConfig(cfg, configPath); err != nil {
		return "", err
	}
	return resp.NodeId, nil
}

func writeConfig(cfg *config.Config, configPath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("tulis config: %w", err)
	}
	return nil
}
```

Refaktor `PairWithToken` lama agar memakai `writeConfig` (badan mkdir tetap, tulis via writeConfig).

CLI `cmd/oikos/main.go` runInstall: tambah flag `--name` (default hostname via os.Hostname(), fallback "oikos-node"). Bila `*panel != ""`: mkdir datadir, open+Migrate db di `<datadir>/oikos.db`, panggil PairWithPanel dengan client nil, cetak node id. Bila kosong: perilaku offline lama. Kode:

```go
name := fs.String("name", "", "nama node (default hostname)")
...
nodeName := *name
if nodeName == "" {
	nodeName, _ = os.Hostname()
	if nodeName == "" {
		nodeName = "oikos-node"
	}
}
if *panel != "" {
	cfg.Panel.Address = *panel
	if err := os.MkdirAll(cfg.Node.DataDir, 0750); err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(cfg.Node.DataDir, "oikos.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		return err
	}
	id, err := agent.PairWithPanel(context.Background(), cfg.Panel.Address, *token, nodeName, cfg, db, *configPath, nil)
	if err != nil {
		return err
	}
	fmt.Println("pairing ok, node id", id)
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -v`
Expected: PASS 3 test pairing.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ cmd/oikos/main.go
git commit -m "feat: pairing panel penuh dan wiring cli"
```

---

### Task 6: agent client (mTLS dial + backoff)

**Files:**
- Create: `internal/agent/client.go`, `internal/agent/client_test.go`
- Test: `internal/agent/client_test.go`

**Interfaces:**
- Consumes: `security.LoadClientTLS` (Task 2), mock panel (Task 4).
- Produces: `agent.DialPanel(ctx, panelAddr, certPath, keyPath, caPath string) (*grpc.ClientConn, error)`; `agent.ComputeBackoff(attempt int) time.Duration` (1s,2s,4s… cap 30s).

- [ ] **Step 1: Write the failing test**

`internal/agent/client_test.go`:

```go
package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

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
	caPEM, _, _ := security.GenerateCA("ca")
	caPath := filepath.Join(dir, "ca.crt")
	os.WriteFile(caPath, caPEM, 0600)
	priv, _ := security.GenerateKey()
	keyPath := filepath.Join(dir, "k")
	security.SaveKey(keyPath, priv)
	certPath := filepath.Join(dir, "c")
	security.SaveCert(certPath, caPEM)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := DialPanel(ctx, "127.0.0.1:1", certPath, keyPath, caPath); err == nil {
		t.Fatal("dial ke port mati harus error")
	}
}

func TestDialWithConfigBufconn(t *testing.T) {
	panel := newMockPanel(t)
	certPEM, key := panel.issue("node-uji")
	keyPEM, _ := security.MarshalKeyPEM(key)
	cert, _ := tls.X509KeyPair(certPEM, keyPEM)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(panel.caPEM)
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := dialWithConfig(ctx, "passthrough:///bufnet", tlsCfg, grpc.WithContextDialer(panel.bufDialer()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
}
```

Mock perlu metode `bufDialer()`:

```go
func (m *mockPanel) bufDialer() func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return m.lis.DialContext(ctx)
	}
}
```

Tambahkan metode ini ke `panel_mock_test.go` pada task ini (edit kecil, tuliskan ulang blok dial agar memakai bufDialer — atau biarkan duplikasi dan tambah metode baru; pilih tambah metode baru + refaktor `dial` memakainya).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run 'TestComputeBackoff|TestDial' -v`
Expected: FAIL `undefined`.

- [ ] **Step 3: Write minimal implementation**

`internal/agent/client.go`:

```go
package agent

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	"github.com/oikos/oikos/internal/security"
)

// ComputeBackoff mengembalikan 2^attempt detik, cap 30 detik.
func ComputeBackoff(attempt int) time.Duration {
	d := time.Second << attempt
	if d <= 0 || d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

// DialPanel membuka koneksi gRPC mTLS ke Panel, menunggu READY dengan backoff.
func DialPanel(ctx context.Context, panelAddr, certPath, keyPath, caPath string) (*grpc.ClientConn, error) {
	tlsCfg, err := security.LoadClientTLS(certPath, keyPath, caPath)
	if err != nil {
		return nil, err
	}
	return dialWithConfig(ctx, panelAddr, tlsCfg)
}

func dialWithConfig(ctx context.Context, target string, tlsCfg *tls.Config, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))}, extra...)
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	if err := waitReady(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func waitReady(ctx context.Context, conn *grpc.ClientConn) error {
	attempt := 0
	conn.Connect()
	for {
		if conn.GetState() == connectivity.Ready {
			return nil
		}
		timer := time.NewTimer(ComputeBackoff(attempt))
		if attempt < 10 {
			attempt++
		}
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			conn.Connect()
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -v -timeout 60s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/
git commit -m "feat: dial mtls panel dengan backoff"
```

---

### Task 7: agent heartbeat

**Files:**
- Create: `internal/agent/heartbeat.go`, `internal/agent/heartbeat_test.go`
- Test: `internal/agent/heartbeat_test.go`

**Interfaces:**
- Consumes: `nodepb.NodeServiceClient`, `ComputeBackoff` (Task 6).
- Produces: `agent.DefaultHeartbeatInterval = 15*time.Second`; `agent.RunHeartbeat(ctx, client, nodeID string, interval time.Duration, since time.Time) error` (kembali hanya saat ctx selesai; tiap tick kirim Heartbeat, gagal → backoff lalu lanjut).

- [ ] **Step 1: Write the failing test**

`internal/agent/heartbeat_test.go`:

```go
package agent

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatSendsPeriodically(t *testing.T) {
	panel := newMockPanel(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunHeartbeat(ctx, panel.mtlsClient(t), "node-1", 10*time.Millisecond, time.Now()) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if len(panel.heartbeats()) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("heartbeat tidak terkirim")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("err = %v, mau context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunHeartbeat tidak berhenti setelah cancel")
	}
	for _, id := range panel.heartbeats() {
		if id != "node-1" {
			t.Fatalf("node id = %q", id)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestHeartbeat -v`
Expected: FAIL `undefined: RunHeartbeat`.

- [ ] **Step 3: Write minimal implementation**

`internal/agent/heartbeat.go`:

```go
package agent

import (
	"context"
	"time"

	nodepb "github.com/oikos/oikos/gen/go/node"
)

// DefaultHeartbeatInterval sesuai rencana Fase 1 (15 detik).
const DefaultHeartbeatInterval = 15 * time.Second

// RunHeartbeat mengirim heartbeat periodik hingga ctx selesai.
func RunHeartbeat(ctx context.Context, client nodepb.NodeServiceClient, nodeID string, interval time.Duration, since time.Time) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fails := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			_, err := client.Heartbeat(callCtx, &nodepb.HeartbeatRequest{
				NodeId:        nodeID,
				UptimeSeconds: int64(now.Sub(since).Seconds()),
			})
			cancel()
			if err != nil {
				fails++
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(ComputeBackoff(fails)):
				}
				continue
			}
			fails = 0
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -run TestHeartbeat -v -timeout 60s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/
git commit -m "feat: heartbeat periodik dengan backoff"
```

---

### Task 8: agent stream (dispatch command → orchestrator)

**Files:**
- Modify: `internal/store/sqlite.go` (tambah ListServers)
- Create: `internal/agent/stream.go`, `internal/agent/stream_test.go`
- Test: `internal/agent/stream_test.go`

**Interfaces:**
- Consumes: `orchestrator.New(db, rt, bus)` (ada), mock panel `queue` (Task 4).
- Produces: `agent.RunCommandStream(ctx, client, lc *orchestrator.Lifecycle) error`; `(*store.DB).ListServers() ([]Server, error)`.

- [ ] **Step 1: Write the failing test**

`internal/agent/stream_test.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/orchestrator"
	"github.com/oikos/oikos/internal/runtime"
	"github.com/oikos/oikos/internal/store"

	nodepb "github.com/oikos/oikos/gen/go/node"
)

func TestStreamDispatch(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateEgg(store.Egg{ID: "egg-1", Name: "e", DockerfilePath: "d", MetadataPath: "m"}); err != nil {
		t.Fatal(err)
	}
	lc := orchestrator.New(db, runtime.NewFake(), orchestrator.NewEventBus())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srvID, err := lc.CreateServer(ctx, "srv", "egg-1", "run", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	envJSON, _ := json.Marshal(map[string]string{"A": "1"})
	panel := newMockPanel(t)
	panel.queue(&nodepb.NodeCommand{CommandId: "c1", Action: "create", Args: map[string]string{
		"name": "srv2", "egg_id": "egg-1", "startup_command": "run", "environment": string(envJSON)}})
	panel.queue(&nodepb.NodeCommand{CommandId: "c2", Action: "start", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c3", Action: "stop", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c4", Action: "bogus", ServerId: srvID})
	panel.queue(&nodepb.NodeCommand{CommandId: "c5", Action: "delete", ServerId: srvID})
	done := make(chan error, 1)
	go func() { done <- RunCommandStream(ctx, panel.mtlsClient(t), lc) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		servers, err := db.ListServers()
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]bool{}
		for _, s := range servers {
			names[s.Name] = true
		}
		if names["srv2"] && len(servers) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream belum selesai diproses: %+v", servers)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
}
```

Urutan queue menjamin srv ter-delete dan srv2 (create) tersisa sendiri → `len==1 && names[srv2]`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestStreamDispatch -v`
Expected: FAIL `undefined`.

- [ ] **Step 3: Write minimal implementation**

`internal/store/sqlite.go`, tambah:

```go
func (d *DB) ListServers() ([]Server, error) {
	rows, err := d.sql.Query(`SELECT id,name,egg_id,container_id,status,startup_command,environment FROM servers ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		var s Server
		var cid sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.EggID, &cid, &s.Status, &s.StartupCommand, &s.Environment); err != nil {
			return nil, err
		}
		s.ContainerID = cid.String
		out = append(out, s)
	}
	return out, rows.Err()
}
```

`internal/agent/stream.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/orchestrator"
)

// RunCommandStream menerima command Panel dan meneruskannya ke orchestrator.
// Command yang gagal diproses dilewati tanpa memutus stream; fungsi kembali
// hanya saat stream berakhir atau ctx selesai.
func RunCommandStream(ctx context.Context, client nodepb.NodeServiceClient, lc *orchestrator.Lifecycle) error {
	stream, err := client.StreamCommands(ctx, &nodepb.StreamRequest{})
	if err != nil {
		return fmt.Errorf("buka stream: %w", err)
	}
	for {
		cmd, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				return fmt.Errorf("recv command: %w", err)
			}
		}
		if err := dispatch(ctx, lc, cmd); err != nil {
			continue
		}
	}
}

func dispatch(ctx context.Context, lc *orchestrator.Lifecycle, cmd *nodepb.NodeCommand) error {
	switch cmd.Action {
	case "create":
		var env map[string]string
		if raw := cmd.Args["environment"]; raw != "" {
			if err := json.Unmarshal([]byte(raw), &env); err != nil {
				return err
			}
		}
		_, err := lc.CreateServer(ctx, cmd.Args["name"], cmd.Args["egg_id"], cmd.Args["startup_command"], env)
		return err
	case "start":
		return lc.StartServer(ctx, cmd.ServerId)
	case "stop":
		return lc.StopServer(ctx, cmd.ServerId)
	case "delete":
		return lc.DeleteServer(ctx, cmd.ServerId)
	default:
		return fmt.Errorf("action tak dikenal: %s", cmd.Action)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/ -v -timeout 60s`
Expected: PASS semua test agent.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/ internal/store/
git commit -m "feat: command stream dispatch ke orchestrator"
```

---

### Task 9: Verifikasi akhir ronde

**Files:** tidak ada (verifikasi saja).

- [ ] **Step 1: Suite penuh + vet + fmt + proto**

Run:

```bash
make proto && make build && go test ./... && go vet ./... && gofmt -l cmd internal gen | head -3
```

Expected: semua PASS, vet bersih, gofmt tanpa output.

- [ ] **Step 2: CLI manual**

Run:

```bash
./bin/oikos install --token <64-hex> --config /tmp/oikos-lanjut/inst.yaml
./bin/oikos server create --config /tmp/oikos-e2e/config.yaml --runtime fake --name t --egg e --startup "sleep 1"
```

Expected: install offline tetap ok; create tetap ok (regresi fondasi tidak rusak).

- [ ] **Step 3: Commit bila ada perbaikan**

```bash
git add -A && git commit -m "fix: hasil verifikasi ronde security-agent" || echo "nothing to commit"
```

---

## Self-Review

- Cakupan: token→CSR→cert (rencana #9/#10), client mTLS + retry/backoff, heartbeat 15s + backoff, stream→orchestrator — semua item security+agent terpetakan. cgroups + API lokal eksplisit di luar ronde ini.
- Placeholder: tidak ada; setiap langkah ada perintah run + expected konkret dan kode eksplisit (termasuk mock Panel lengkap).
- Konsistensi tipe: `nodepb` = `gen/go/node`; signature `PairWithPanel(ctx, panelAddr, token, nodeName, cfg, db, configPath, client)` dipakai sama di test, implementasi, dan CLI (CLI passing nil); `RunHeartbeat(ctx, client, nodeID, interval, since)`; `RunCommandStream(ctx, client, lc)`; `store.Node` field cocok dengan kolom `nodes`; `ComputeBackoff` dipakai client + heartbeat.
