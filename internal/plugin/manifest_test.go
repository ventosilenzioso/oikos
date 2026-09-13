package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestLoadParsesYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugin.yaml")
	content := `id: log-to-file
name: Log to file
version: 1.0.0
binary: /var/lib/oikos/plugins/log-to-file
enabled: true
config: /etc/oikos/plugins/log-to-file.yaml
allowed_events:
  - server.crashed
allowed_routes:
  - /plugins/log-to-file/status
sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest() error = %v", err)
	}
	if manifest.ID != "log-to-file" || !manifest.Enabled || len(manifest.AllowedEvents) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestManifestValidationRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
	}{
		{name: "invalid id", edit: func(m *Manifest) { m.ID = "bad/id" }},
		{name: "global route", edit: func(m *Manifest) { m.AllowedRoutes = []string{"/admin"} }},
		{name: "route traversal", edit: func(m *Manifest) { m.AllowedRoutes = []string{"/plugins/p/../admin"} }},
		{name: "empty binary", edit: func(m *Manifest) { m.Binary = "" }},
		{name: "invalid checksum", edit: func(m *Manifest) { m.SHA256 = "short" }},
		{name: "invalid event", edit: func(m *Manifest) { m.AllowedEvents = []string{""} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validManifest()
			tt.edit(&manifest)
			if err := ValidateManifest(manifest); err == nil {
				t.Fatal("ValidateManifest() unexpectedly succeeded")
			}
		})
	}
}

func TestCapabilitiesRejectsEventOutsideManifest(t *testing.T) {
	manifest := validManifest()
	capability := Capability{Events: []string{"server.started"}}

	if err := ValidateCapabilities(manifest, capability); err == nil {
		t.Fatal("ValidateCapabilities() unexpectedly succeeded")
	}
}

func TestCapabilitiesAcceptsDeclaredEventsAndRoutes(t *testing.T) {
	manifest := validManifest()
	capability := Capability{
		Events: []string{"server.crashed"},
		Routes: []string{"/plugins/log-to-file/status"},
	}

	if err := ValidateCapabilities(manifest, capability); err != nil {
		t.Fatalf("ValidateCapabilities() error = %v", err)
	}
}

func validManifest() Manifest {
	return Manifest{
		ID:            "log-to-file",
		Name:          "Log to file",
		Version:       "1.0.0",
		Binary:        "/var/lib/oikos/plugins/log-to-file",
		AllowedEvents: []string{"server.crashed"},
		AllowedRoutes: []string{"/plugins/log-to-file/status"},
		SHA256:        strings.Repeat("a", 64),
	}
}
