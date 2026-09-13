package plugin

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Version       string   `yaml:"version"`
	Binary        string   `yaml:"binary"`
	Enabled       bool     `yaml:"enabled"`
	Config        string   `yaml:"config"`
	AllowedEvents []string `yaml:"allowed_events"`
	AllowedRoutes []string `yaml:"allowed_routes"`
	SHA256        string   `yaml:"sha256"`
}

var (
	manifestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	eventNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	sha256Pattern     = regexp.MustCompile(`^[A-Fa-f0-9]{64}$`)
)

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if !manifestIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid plugin id %q", manifest.ID)
	}
	if strings.TrimSpace(manifest.Binary) == "" {
		return fmt.Errorf("plugin binary must not be empty")
	}
	if !sha256Pattern.MatchString(manifest.SHA256) {
		return fmt.Errorf("invalid sha256 checksum")
	}
	for _, event := range manifest.AllowedEvents {
		if !eventNamePattern.MatchString(event) {
			return fmt.Errorf("invalid event name %q", event)
		}
	}
	for _, route := range manifest.AllowedRoutes {
		if err := validateRoute(manifest.ID, route); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCapabilities(manifest Manifest, capability Capability) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	events := make(map[string]struct{}, len(manifest.AllowedEvents))
	for _, event := range manifest.AllowedEvents {
		events[event] = struct{}{}
	}
	routes := make(map[string]struct{}, len(manifest.AllowedRoutes))
	for _, route := range manifest.AllowedRoutes {
		routes[route] = struct{}{}
	}
	for _, event := range capability.Events {
		if _, ok := events[event]; !ok {
			return fmt.Errorf("capability event %q is not allowed", event)
		}
	}
	for _, route := range capability.Routes {
		if _, ok := routes[route]; !ok {
			return fmt.Errorf("capability route %q is not allowed", route)
		}
	}
	return nil
}

func validateRoute(id, route string) error {
	prefix := "/plugins/" + id + "/"
	if !strings.HasPrefix(route, prefix) || strings.Contains(route, "..") {
		return fmt.Errorf("invalid plugin route %q", route)
	}
	return nil
}
