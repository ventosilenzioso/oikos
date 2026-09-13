package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type HealthRunner interface {
	Run(string) error
}

type HealthRunnerFunc func(string) error

func (f HealthRunnerFunc) Run(path string) error { return f(path) }

type UpdateManager interface {
	Check(Release) error
	Apply(Release) error
	Rollback() error
}

type manager struct {
	config SlotConfig
	verify ReleaseVerifier
	health HealthRunner
}

func NewManager(config SlotConfig, verifier ReleaseVerifier, health HealthRunner) UpdateManager {
	return &manager{config: config, verify: verifier, health: health}
}

func (m *manager) Check(release Release) error {
	body, err := download(release.BinaryURL)
	if err != nil {
		return err
	}
	return verifyRelease(release, body, m.verify)
}

func (m *manager) Apply(release Release) error {
	if err := m.config.validate(); err != nil {
		return err
	}
	body, err := download(release.BinaryURL)
	if err != nil {
		return err
	}
	if err := verifyRelease(release, body, m.verify); err != nil {
		return err
	}
	state, err := m.config.loadState()
	if err != nil {
		return fmt.Errorf("read current slot: %w", err)
	}
	if release.Version == state.Current || release.Version == state.Previous {
		return fmt.Errorf("release version %q is already an active or previous slot", release.Version)
	}
	if m.config.Backup != nil {
		if err := m.config.Backup(); err != nil {
			return fmt.Errorf("backup before update: %w", err)
		}
	}
	if err := os.MkdirAll(m.config.Root, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(m.config.Root, ".candidate-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	candidate := filepath.Join(m.config.Root, release.Version)
	if err := os.RemoveAll(candidate); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, candidate); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(candidate)
		}
	}()
	if m.health != nil {
		if err := m.health.Run(candidate); err != nil {
			return fmt.Errorf("candidate health check: %w", err)
		}
	}
	newState := slotState{Current: release.Version, Previous: state.Current}
	if err := m.config.saveState(newState); err != nil {
		return err
	}
	// Once the state manifest is saved, the candidate becomes a historical slot
	// intentionally retained for diagnostics and rollback.
	committed = true
	if err := m.config.projectState(newState); err != nil {
		return m.config.restoreState(state, fmt.Errorf("project slot state: %w", err))
	}
	if m.health != nil {
		if err := m.health.Run(filepath.Join(m.config.Root, "current")); err != nil {
			return m.config.restoreState(state, fmt.Errorf("post-switch health check: %w", err))
		}
	}
	return nil
}

func (m *manager) Rollback() error {
	if err := m.config.validate(); err != nil {
		return err
	}
	state, err := m.config.loadState()
	if err != nil {
		return fmt.Errorf("read previous slot: %w", err)
	}
	if state.Previous == "" {
		return fmt.Errorf("previous slot is unavailable")
	}
	newState := slotState{Current: state.Previous, Previous: state.Current}
	if err := m.config.saveState(newState); err != nil {
		return err
	}
	if err := m.config.projectState(newState); err != nil {
		return m.config.restoreState(state, fmt.Errorf("rollback projection: %w", err))
	}
	return nil
}

func download(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download release: HTTP %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

var _ UpdateManager = (*manager)(nil)
