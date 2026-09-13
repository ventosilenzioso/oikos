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
	priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
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
	caPEM, _, err := GenerateCA("test-ca")
	if err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(dir, "ca.crt")
	if err := SaveCert(caPath, caPEM); err != nil {
		t.Fatal(err)
	}
	priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
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
}

func TestLoadClientTLSMissingFile(t *testing.T) {
	if _, err := LoadClientTLS("/tidak/ada.crt", "/tidak/ada.key", "/tidak/ada-ca.crt"); err == nil {
		t.Fatal("harus error untuk file hilang")
	}
}
