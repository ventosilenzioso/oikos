package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SlotConfig struct {
	Root   string
	Backup func() error
}

func (c SlotConfig) validate() error {
	if c.Root == "" {
		return errors.New("slot root is required")
	}
	return nil
}

func (c SlotConfig) switchTo(target string) error {
	current := filepath.Join(c.Root, "current")
	previous := filepath.Join(c.Root, "previous")
	old, err := os.Readlink(current)
	if err != nil {
		return fmt.Errorf("read current slot: %w", err)
	}
	if err := replaceSymlink(c.Root, "previous", old); err != nil {
		return err
	}
	if err := replaceSymlink(c.Root, "current", target); err != nil {
		return err
	}
	_ = previous
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
