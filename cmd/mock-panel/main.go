// Command mock-panel adalah Panel palsu untuk development/manual testing
// (pairing, heartbeat, command stream). JANGAN dipakai di produksi:
// token statis tanpa autentikasi admin dan tanpa persistensi.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	nodepb "github.com/oikos/oikos/gen/go/node"
	"github.com/oikos/oikos/internal/security"
)

type panel struct {
	nodepb.UnimplementedNodeServiceServer
	mu     sync.Mutex
	tokens map[string]bool
	caCert *x509.Certificate
	caPriv ed25519.PrivateKey
}

func (p *panel) Pair(_ context.Context, req *nodepb.PairingRequest) (*nodepb.PairingResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	used, ok := p.tokens[req.Token]
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
	p.tokens[req.Token] = true
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      csr.Subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, p.caCert, csr.PublicKey, p.caPriv)
	if err != nil {
		return nil, fmt.Errorf("sign cert: %w", err)
	}
	fmt.Printf("paired node %q\n", req.NodeName)
	return &nodepb.PairingResponse{
		NodeId:  uuid.NewString(),
		CertPem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
	}, nil
}

func (p *panel) Heartbeat(_ context.Context, req *nodepb.HeartbeatRequest) (*nodepb.HeartbeatAck, error) {
	fmt.Printf("heartbeat node=%s uptime=%ds\n", req.NodeId, req.UptimeSeconds)
	return &nodepb.HeartbeatAck{Ok: true}, nil
}

func (p *panel) StreamCommands(_ *nodepb.StreamRequest, srv nodepb.NodeService_StreamCommandsServer) error {
	<-srv.Context().Done()
	return srv.Context().Err()
}

func main() {
	plainAddr := flag.String("plain-addr", "127.0.0.1:19091", "alamat pairing plaintext")
	tlsAddr := flag.String("tls-addr", "127.0.0.1:19092", "alamat mTLS heartbeat/stream")
	tokens := flag.String("tokens", "", "token sekali pakai, dipisah koma")
	caOut := flag.String("ca-out", "mock-ca.crt", "path tulis CA cert")
	flag.Parse()
	if *tokens == "" {
		fmt.Fprintln(os.Stderr, "mock-panel: --tokens wajib diisi")
		os.Exit(1)
	}
	caPEM, caPriv, err := security.GenerateCA("mock-panel-ca")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	block, _ := pem.Decode(caPEM)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*caOut, caPEM, 0600); err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	p := &panel{tokens: map[string]bool{}, caCert: caCert, caPriv: caPriv}
	for _, tok := range strings.Split(*tokens, ",") {
		if strings.TrimSpace(tok) != "" {
			p.tokens[strings.TrimSpace(tok)] = false
		}
	}
	plainLis, err := net.Listen("tcp", *plainAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	plainSrv := grpc.NewServer()
	nodepb.RegisterNodeServiceServer(plainSrv, p)
	go func() { _ = plainSrv.Serve(plainLis) }()
	defer plainSrv.Stop()

	_, srvKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "panel.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, srvKey.Public(), caPriv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	srvCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER})
	srvKeyPEM, err := security.MarshalKeyPEM(srvKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	srvCert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	tlsLis, err := net.Listen("tcp", *tlsAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mock-panel:", err)
		os.Exit(1)
	}
	tlsSrv := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	})))
	nodepb.RegisterNodeServiceServer(tlsSrv, p)
	go func() { _ = tlsSrv.Serve(tlsLis) }()
	defer tlsSrv.Stop()

	fmt.Printf("mock-panel jalan: plain=%s tls=%s ca=%s\n", *plainAddr, *tlsAddr, *caOut)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
