package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SlotConfig struct {
	Root    string
	Backup  func() error
	Project func() error
}

type slotState struct {
	Current  string `json:"current"`
	Previous string `json:"previous"`
}

func (c SlotConfig) validate() error {
	if c.Root == "" {
		return errors.New("slot root is required")
	}
	return nil
}

func (c SlotConfig) loadState() (slotState, error) {
	data, err := os.ReadFile(filepath.Join(c.Root, "state.json"))
	if err == nil {
		var state slotState
		if err := json.Unmarshal(data, &state); err != nil {
			return slotState{}, fmt.Errorf("decode slot state: %w", err)
		}
		if err := validateVersion(state.Current); err != nil {
			return slotState{}, fmt.Errorf("invalid current slot state: %w", err)
		}
		if state.Previous != "" {
			if err := validateVersion(state.Previous); err != nil {
				return slotState{}, fmt.Errorf("invalid previous slot state: %w", err)
			}
		}
		return state, nil
	}
	if !os.IsNotExist(err) {
		return slotState{}, err
	}
	current, err := os.Readlink(filepath.Join(c.Root, "current"))
	if err != nil {
		return slotState{}, fmt.Errorf("read current slot: %w", err)
	}
	if err := validateVersion(current); err != nil {
		return slotState{}, err
	}
	previous, _ := os.Readlink(filepath.Join(c.Root, "previous"))
	if previous != "" {
		if err := validateVersion(previous); err != nil {
			return slotState{}, err
		}
	}
	return slotState{Current: current, Previous: previous}, nil
}

func (c SlotConfig) saveState(state slotState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(c.Root, ".state-")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(path, filepath.Join(c.Root, "state.json"))
}

func (c SlotConfig) projectState(state slotState) error {
	if c.Project != nil {
		return c.Project()
	}
	return c.projectStateDirect(state)
}

func (c SlotConfig) projectStateDirect(state slotState) error {
	if err := replaceSymlink(c.Root, "current", state.Current); err != nil {
		return err
	}
	if state.Previous != "" {
		return replaceSymlink(c.Root, "previous", state.Previous)
	}
	_ = os.Remove(filepath.Join(c.Root, "previous"))
	return nil
}

func replaceSymlink(root, name, target string) error {
	tmp, err := os.CreateTemp(root, "."+name+"-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	if err := os.Symlink(target, tmpPath); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(root, name)); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
