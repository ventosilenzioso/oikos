package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testVerifier struct {
	err error
}

func (v testVerifier) Verify(Release, []byte) error { return v.err }

type testHealth struct {
	paths []string
	errs  []error
}

func (h *testHealth) Run(path string) error {
	h.paths = append(h.paths, path)
	if len(h.errs) >= len(h.paths) {
		return h.errs[len(h.paths)-1]
	}
	return nil
}

func TestApplyChecksumMismatchLeavesCurrentUnchanged(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")

	server := releaseServer("candidate")
	defer server.Close()
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, &testHealth{})

	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: "0000000000000000000000000000000000000000000000000000000000000000", Signature: []byte("signature")})
	if err == nil {
		t.Fatal("Apply succeeded with a checksum mismatch")
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
}

func TestApplySwitchesAtomicallyAndRollbackRestoresPrevious(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")

	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	backupCalls := 0
	health := &testHealth{}
	mgr := NewManager(SlotConfig{Root: root, Backup: func() error { backupCalls++; return nil }}, testVerifier{}, health)
	release := Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")}
	if err := mgr.Apply(release); err != nil {
		t.Fatal(err)
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v2" {
		t.Fatalf("current target = %q, want v2", got)
	}
	if got := linkTarget(t, filepath.Join(root, "previous")); got != "v1" {
		t.Fatalf("previous target = %q, want v1", got)
	}
	if backupCalls != 1 {
		t.Fatalf("backup calls = %d, want 1", backupCalls)
	}
	if err := mgr.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current after rollback = %q, want v1", got)
	}
	if len(health.paths) != 2 {
		t.Fatalf("health calls = %d, want 2", len(health.paths))
	}
}

func TestApplyHealthFailureBeforeSwitchPreservesKnownGood(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	health := &testHealth{errs: []error{errors.New("unhealthy")}}
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, health)

	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
	if err == nil {
		t.Fatal("Apply succeeded with an unhealthy candidate")
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "previous")); !os.IsNotExist(err) {
		t.Fatalf("previous exists after pre-switch failure: %v", err)
	}
}

func TestApplyHealthFailureAfterSwitchRestoresKnownGood(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	health := &testHealth{errs: []error{nil, errors.New("unhealthy")}}
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, health)

	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
	if err == nil {
		t.Fatal("Apply succeeded with an unhealthy switched slot")
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "previous")); !os.IsNotExist(err) {
		t.Fatalf("previous projection exists after restoring state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "v2")); err != nil {
		t.Fatalf("candidate slot was deleted: %v", err)
	}
}

func TestApplyVerifiesSignatureAndMakesCandidateExecutable(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	verifier := &recordingVerifier{}
	mgr := NewManager(SlotConfig{Root: root}, verifier, nil)
	release := Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature"), GOOS: "linux", GOARCH: "amd64"}
	if err := mgr.Apply(release); err != nil {
		t.Fatal(err)
	}
	if !verifier.called {
		t.Fatal("signature verifier was not called")
	}
	info, err := os.Stat(filepath.Join(root, "v2"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("candidate mode = %o, want 755", info.Mode().Perm())
	}
}

func TestApplyRejectsUnsafeVersionWithoutDeletingExistingPath(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	sentinel := filepath.Join(root, "sentinel")
	writeSlot(t, root, "sentinel", "do not delete")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, nil)
	for _, version := range []string{"", ".", "..", "../sentinel", "/tmp/evil", "nested/v2"} {
		t.Run(version, func(t *testing.T) {
			err := mgr.Apply(Release{Version: version, BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
			if err == nil {
				t.Fatal("Apply accepted unsafe version")
			}
		})
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "do not delete" {
		t.Fatalf("sentinel changed: %q, %v", got, err)
	}
}

func TestApplyRequiresExactLowercaseChecksum(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, nil)
	valid := checksum(body)
	for _, digest := range []string{strings.ToUpper(valid), " " + valid, valid + " "} {
		t.Run(digest, func(t *testing.T) {
			if err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: digest, Signature: []byte("signature")}); err == nil {
				t.Fatal("Apply accepted non-exact checksum")
			}
		})
	}
}

func TestApplyRequiresVerifierAndSignature(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	release := Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body)}
	if err := NewManager(SlotConfig{Root: root}, nil, nil).Apply(release); err == nil {
		t.Fatal("Apply accepted nil verifier")
	}
	if err := NewManager(SlotConfig{Root: root}, testVerifier{}, nil).Apply(release); err == nil {
		t.Fatal("Apply accepted empty signature")
	}
}

func TestApplyVerifierAndBackupFailuresPreserveCurrent(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	release := Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")}
	if err := NewManager(SlotConfig{Root: root, Backup: func() error { return errors.New("backup failed") }}, testVerifier{}, nil).Apply(release); err == nil {
		t.Fatal("Apply accepted backup failure")
	}
	if err := NewManager(SlotConfig{Root: root}, testVerifier{err: errors.New("bad signature")}, nil).Apply(release); err == nil {
		t.Fatal("Apply accepted verifier failure")
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
}

func TestStateManifestKeepsSlotsConsistentAcrossManagerInstances(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	release := Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")}
	if err := NewManager(SlotConfig{Root: root}, testVerifier{}, nil).Apply(release); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if err := NewManager(SlotConfig{Root: root}, testVerifier{}, nil).Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
	if got := linkTarget(t, filepath.Join(root, "previous")); got != "v2" {
		t.Fatalf("previous target = %q, want v2", got)
	}
}

type recordingVerifier struct{ called bool }

func (v *recordingVerifier) Verify(Release, []byte) error {
	v.called = true
	return nil
}

func releaseServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
}

func checksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func writeSlot(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func linkTarget(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	return target
}
