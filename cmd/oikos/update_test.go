package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/oikos/oikos/internal/agent"
	internalupdate "github.com/oikos/oikos/internal/update"
)

func TestRunUpdateSwitchesSlotAndBacksUpConfigAndDatabase(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	versions := filepath.Join(root, "versions")
	configPath := filepath.Join(root, "config.yaml")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("node:\n  data_dir: "+dataDir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "oikos.db"), []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUpdateSlot(t, versions, "v1")
	linkUpdate(t, filepath.Join(versions, "current"), "v1")

	body := []byte("candidate")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	oldVerifier, oldHealth := updateVerifier, updateHealthRunner
	updateVerifier = acceptingVerifier{}
	updateHealthRunner = func(string, string) error { return nil }
	defer func() { updateVerifier, updateHealthRunner = oldVerifier, oldHealth }()

	if err := runUpdate([]string{"--version", "v2", "--url", server.URL, "--sha256", updateChecksum(body), "--signature", "sig", "--versions-dir", versions, "--config", configPath}); err != nil {
		t.Fatal(err)
	}
	if got := updateLinkTarget(t, filepath.Join(versions, "current")); got != "v2" {
		t.Fatalf("current target = %q, want v2", got)
	}
	entries, err := os.ReadDir(filepath.Join(versions, "backups"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("backup directory entries = %d, err = %v", len(entries), err)
	}
}

func TestRunUpdateFailedHealthCheckKeepsCurrentSlot(t *testing.T) {
	root := t.TempDir()
	versions := filepath.Join(root, "versions")
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("node:\n  data_dir: "+root+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUpdateSlot(t, versions, "v1")
	linkUpdate(t, filepath.Join(versions, "current"), "v1")
	body := []byte("candidate")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	oldVerifier, oldHealth := updateVerifier, updateHealthRunner
	updateVerifier = acceptingVerifier{}
	updateHealthRunner = func(string, string) error { return errors.New("unhealthy") }
	defer func() { updateVerifier, updateHealthRunner = oldVerifier, oldHealth }()

	err := runUpdate([]string{"--version", "v2", "--url", server.URL, "--sha256", updateChecksum(body), "--signature", "sig", "--versions-dir", versions, "--config", configPath})
	if err == nil {
		t.Fatal("runUpdate succeeded with an unhealthy candidate")
	}
	if got := updateLinkTarget(t, filepath.Join(versions, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
}

func TestRunUpdateHealthCheckOnlyRejectsInvalidSQLite(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("node:\n  data_dir: /no/such/directory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runUpdate([]string{"--health-check-only", "--config", configPath}); err == nil {
		t.Fatal("health-check-only succeeded with an invalid data directory")
	}
}

func TestDaemonHandoffInvokesDrainWithoutRuntimeLifecycle(t *testing.T) {
	called := false
	args := agent.DaemonArgs{Handoff: func() { called = true }}
	args.TriggerHandoff()
	if !called {
		t.Fatal("handoff callback was not invoked")
	}
}

type acceptingVerifier struct{}

func (acceptingVerifier) Verify(internalupdate.Release, []byte) error { return nil }

func updateChecksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func writeUpdateSlot(t *testing.T, root, name string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o755); err != nil {
		t.Fatal(err)
	}
}

func linkUpdate(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func updateLinkTarget(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	return target
}
