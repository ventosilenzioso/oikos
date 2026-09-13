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
	if m.health != nil {
		if err := m.health.Run(candidate); err != nil {
			return fmt.Errorf("candidate health check: %w", err)
		}
	}
	newState := slotState{Current: release.Version, Previous: state.Current}
	if err := m.config.saveState(newState); err != nil {
		return err
	}
	if err := m.config.projectState(newState); err != nil {
		return err
	}
	if m.health != nil {
		if err := m.health.Run(filepath.Join(m.config.Root, "current")); err != nil {
			if rollbackErr := m.config.saveState(state); rollbackErr != nil {
				return fmt.Errorf("post-switch health check: %w; restore: %v", err, rollbackErr)
			}
			_ = m.config.projectState(state)
			return fmt.Errorf("post-switch health check: %w", err)
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
		return err
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
