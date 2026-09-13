package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oikos/oikos/internal/agent"
	internalupdate "github.com/oikos/oikos/internal/update"
)

func TestRunUpdateSwitchesSlotAndBacksUpConfigAndDatabase(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	versions := filepath.Join(root, "versions")
	configPath := filepath.Join(root, "config.yaml")
	manifestPath := filepath.Join(root, "plugins.yaml")
	configBytes := []byte("node:\n  data_dir: " + dataDir + "\nplugins:\n  manifest_path: " + manifestPath + "\n")
	databaseBytes := []byte("database")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte("plugin-manifest"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "oikos.db"), databaseBytes, 0o600); err != nil {
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
	backupFiles, err := os.ReadDir(filepath.Join(versions, "backups", entries[0].Name()))
	if err != nil || len(backupFiles) != 3 {
		t.Fatalf("backup files = %d, err = %v", len(backupFiles), err)
	}
	configBackup, err := os.ReadFile(filepath.Join(versions, "backups", entries[0].Name(), "config.yaml"))
	if err != nil || !reflect.DeepEqual(configBackup, configBytes) {
		t.Fatalf("config backup = %q, err = %v", configBackup, err)
	}
	databaseBackup, err := os.ReadFile(filepath.Join(versions, "backups", entries[0].Name(), "oikos.db"))
	if err != nil || !reflect.DeepEqual(databaseBackup, databaseBytes) {
		t.Fatalf("database backup = %q, err = %v", databaseBackup, err)
	}
	manifest, err := os.ReadFile(filepath.Join(versions, "backups", entries[0].Name(), "plugins.yaml"))
	if err != nil || string(manifest) != "plugin-manifest" {
		t.Fatalf("manifest backup = %q, err = %v", manifest, err)
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
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "oikos.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("node:\n  data_dir: "+root+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runUpdate([]string{"--health-check-only", "--config", configPath}); err == nil {
		t.Fatal("health-check-only succeeded with an invalid data directory")
	}
}

func TestDefaultHealthRunnerUsesExactCandidateArguments(t *testing.T) {
	old := updateCommandRunner
	defer func() { updateCommandRunner = old }()
	var gotName string
	var gotArgs []string
	updateCommandRunner = func(name string, args ...string) error { gotName, gotArgs = name, args; return nil }
	if err := defaultHealthRunner("/candidate/oikos", "/etc/oikos/config.yaml"); err != nil {
		t.Fatal(err)
	}
	if gotName != "/candidate/oikos" || !reflect.DeepEqual(gotArgs, []string{"--health-check-only", "--config", "/etc/oikos/config.yaml"}) {
		t.Fatalf("command = %q %v", gotName, gotArgs)
	}
}

func TestHealthCheckUsesDockerPingDependency(t *testing.T) {
	old := updateDockerPing
	defer func() { updateDockerPing = old }()
	called := ""
	updateDockerPing = func(host string) error { called = host; return nil }
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("node:\n  data_dir: "+root+"\nruntime:\n  engine: docker\n  docker_socket: pingable\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runUpdate([]string{"--health-check-only", "--config", configPath}); err != nil {
		t.Fatal(err)
	}
	if called != "pingable" {
		t.Fatalf("ping host = %q", called)
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
