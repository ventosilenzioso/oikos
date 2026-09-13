package filesystem

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type credentialStore struct{}

func (credentialStore) GetSFTPCredential(string) (store.SFTPCredential, error) {
	return store.SFTPCredential{}, errors.New("not found")
}

type liveCredentialStore struct{ key ssh.PublicKey }

func (s liveCredentialStore) GetSFTPCredential(string) (store.SFTPCredential, error) {
	return store.SFTPCredential{ServerID: "srv-1", Username: "srv-1", PublicKey: string(ssh.MarshalAuthorizedKey(s.key))}, nil
}

func TestNewSFTPServerCreatesHostKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host.key")
	srv, err := NewSFTPServer(config.SFTPConfig{BindAddr: "127.0.0.1:0", HostKeyPath: path}, NewManager(t.TempDir()), credentialStore{})
	if err != nil {
		t.Fatal(err)
	}
	if srv == nil {
		t.Fatal("server nil")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("host key mode=%o", info.Mode().Perm())
	}
}

func TestSFTPClientCanUseJailedManager(t *testing.T) {
	root := filepath.Join(t.TempDir(), "servers")
	manager := NewManager(root)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewSFTPServer(config.SFTPConfig{BindAddr: "127.0.0.1:0", HostKeyPath: filepath.Join(t.TempDir(), "host.key")}, manager, liveCredentialStore{key: sshPub})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	deadline := time.Now().Add(time.Second)
	for srv.Addr() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if srv.Addr() == nil {
		t.Fatal("SFTP listener tidak start")
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	clientCfg := &ssh.ClientConfig{User: "srv-1", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	conn, err := ssh.Dial("tcp", srv.Addr().String(), clientCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		t.Fatal(err)
	}
	defer sftpClient.Close()
	f, err := sftpClient.Create("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := sftpClient.Open("../../etc/passwd"); err == nil {
		t.Fatal("path traversal harus ditolak")
	}
	_ = done
}
