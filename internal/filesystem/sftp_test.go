package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/config"
	"github.com/oikos/oikos/internal/store"
)

type credentialStore struct{}

func (credentialStore) GetSFTPCredential(string) (store.SFTPCredential, error) {
	return store.SFTPCredential{}, errors.New("not found")
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
