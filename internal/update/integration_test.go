//go:build integration

package update

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateRollbackOnCandidateHealthFailureIntegration(t *testing.T) {
	root := t.TempDir()
	writeSlot(t, root, "v1", "known-good")
	link(t, filepath.Join(root, "current"), "v1")
	body := []byte("bad-candidate")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	health := HealthRunnerFunc(func(path string) error {
		if filepath.Base(path) == "v2" {
			return errors.New("candidate unhealthy")
		}
		return nil
	})
	mgr := NewManager(SlotConfig{Root: root}, testVerifier{}, health)
	err := mgr.Apply(Release{Version: "v2", BinaryURL: server.URL, SHA256: checksum(body), Signature: []byte("sig")})
	if err == nil {
		t.Fatal("unhealthy candidate accepted")
	}
	if got := linkTarget(t, filepath.Join(root, "current")); got != "v1" {
		t.Fatalf("current=%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "v2")); !os.IsNotExist(err) {
		t.Fatalf("candidate remains: %v", err)
	}
}
