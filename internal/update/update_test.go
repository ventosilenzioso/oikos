package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	for _, version := range []string{"", ".", "..", "../sentinel", "/tmp/evil", "nested/v2", "current", "previous", "state.json"} {
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

func TestApplyRejectsActiveAndPreviousVersionsWithoutChangingProjections(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	writeSlot(t, root, "v2", "previous-good")
	link(t, filepath.Join(root, "current"), "v1")
	link(t, filepath.Join(root, "previous"), "v2")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, nil)
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			err := mgr.Apply(Release{Version: version, BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
			if err == nil {
				t.Fatal("Apply accepted an existing active or previous slot")
			}
			if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
				t.Fatalf("current target = %q, want v1", got)
			}
			if got := linkTarget(t, filepath.Join(root, "previous")); got != "v2" {
				t.Fatalf("previous target = %q, want v2", got)
			}
			assertFile(t, filepath.Join(root, "v1"), "known-good")
			assertFile(t, filepath.Join(root, "v2"), "previous-good")
		})
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

func TestApplyProjectionFailureRestoresPriorManifestAndProjections(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	projectCalls := 0
	mgr := NewManager(SlotConfig{Root: root, Project: func() error {
		projectCalls++
		if projectCalls == 1 {
			_ = os.Remove(filepath.Join(root, "current"))
			return errors.New("projection failed")
		}
		return nil
	}}, testVerifier{}, nil)
	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
	if err == nil {
		t.Fatal("Apply succeeded despite projection failure")
	}
	if projectCalls != 2 {
		t.Fatalf("projection calls = %d, want 2", projectCalls)
	}
	state, err := mgr.(*manager).config.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != "v1" || state.Previous != "" {
		t.Fatalf("state after projection failure = %+v, want current v1 and no previous", state)
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current target = %q, want v1", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "previous")); !os.IsNotExist(err) {
		t.Fatalf("previous projection after recovery: %v", err)
	}
}

func TestRollbackProjectionFailureRestoresPriorManifest(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	writeSlot(t, root, "v2", "candidate")
	link(t, filepath.Join(root, "current"), "v2")
	link(t, filepath.Join(root, "previous"), "v1")
	projectCalls := 0
	mgr := NewManager(SlotConfig{Root: root, Project: func() error {
		projectCalls++
		if projectCalls == 1 {
			_ = os.Remove(filepath.Join(root, "current"))
			return errors.New("rollback projection failed")
		}
		return nil
	}}, testVerifier{}, nil)
	if err := mgr.Rollback(); err == nil {
		t.Fatal("Rollback succeeded despite projection failure")
	} else if !strings.Contains(err.Error(), "rollback projection failed") {
		t.Fatalf("rollback error = %q, missing projection failure", err)
	}
	state, err := mgr.(*manager).config.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Current != "v2" || state.Previous != "v1" {
		t.Fatalf("state after rollback failure = %+v, want v2/v1", state)
	}
}

func TestRollbackRecoveryFailureIncludesDiagnosticsAndPriorState(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	writeSlot(t, root, "v2", "candidate")
	link(t, filepath.Join(root, "current"), "v2")
	link(t, filepath.Join(root, "previous"), "v1")
	projectCalls := 0
	mgr := NewManager(SlotConfig{Root: root, Project: func() error {
		projectCalls++
		return fmt.Errorf("projection failure %d", projectCalls)
	}}, testVerifier{}, nil)
	err := mgr.Rollback()
	if err == nil {
		t.Fatal("Rollback succeeded despite projection and recovery failures")
	}
	if !strings.Contains(err.Error(), "rollback projection") || !strings.Contains(err.Error(), "projection failure 2") {
		t.Fatalf("rollback recovery error = %q, missing diagnostics", err)
	}
	state, stateErr := mgr.(*manager).config.loadState()
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if state.Current != "v2" || state.Previous != "v1" {
		t.Fatalf("state after rollback recovery failure = %+v, want v2/v1", state)
	}
}

func TestPostSwitchHealthFailureSurfacesProjectionRecoveryFailure(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("candidate")
	server := releaseServer(string(body))
	defer server.Close()
	projectCalls := 0
	mgr := NewManager(SlotConfig{Root: root, Project: func() error {
		projectCalls++
		if projectCalls == 2 {
			return errors.New("recovery projection failed")
		}
		return nil
	}}, testVerifier{}, &testHealth{errs: []error{nil, errors.New("post-switch unhealthy")}})
	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("signature")})
	if err == nil {
		t.Fatal("Apply succeeded despite post-switch health failure")
	}
	if !strings.Contains(err.Error(), "post-switch unhealthy") || !strings.Contains(err.Error(), "recovery projection failed") {
		t.Fatalf("health recovery error = %q, missing diagnostics", err)
	}
	state, stateErr := mgr.(*manager).config.loadState()
	if stateErr != nil {
		t.Fatal(stateErr)
	}
	if state.Current != "v1" || state.Previous != "" {
		t.Fatalf("state after health recovery failure = %+v, want v1/no previous", state)
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

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}
