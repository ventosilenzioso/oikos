//go:build linux

package resource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ApplyLimits writes cgroup v2 limits to an existing cgroup directory.
func ApplyLimits(dir string, lim Limits) error {
	if lim.CPUMillicores > 0 {
		if err := writeCgroupFile(dir, "cpu.max", strconv.FormatInt(lim.CPUMillicores*100, 10)+" 100000"); err != nil {
			return err
		}
	}
	if lim.MemoryBytes > 0 {
		if err := writeCgroupFile(dir, "memory.max", strconv.FormatInt(lim.MemoryBytes, 10)); err != nil {
			return err
		}
	}
	if lim.PIDMax > 0 {
		if err := writeCgroupFile(dir, "pids.max", strconv.FormatInt(lim.PIDMax, 10)); err != nil {
			return err
		}
	}
	return nil
}

// ReadLimits reads limits from a cgroup directory; "max" is returned as 0.
func ReadLimits(dir string) (Limits, error) {
	var lim Limits
	cpu, err := readCgroupFile(dir, "cpu.max")
	if err != nil {
		return Limits{}, err
	}
	if f := strings.Fields(cpu); len(f) > 0 && f[0] != "max" {
		quota, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse cpu.max: %w", err)
		}
		lim.CPUMillicores = quota / 100
	}
	mem, err := readCgroupFile(dir, "memory.max")
	if err != nil {
		return Limits{}, err
	}
	if strings.TrimSpace(mem) != "max" {
		lim.MemoryBytes, err = strconv.ParseInt(strings.TrimSpace(mem), 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse memory.max: %w", err)
		}
	}
	pids, err := readCgroupFile(dir, "pids.max")
	if err != nil {
		return Limits{}, err
	}
	if strings.TrimSpace(pids) != "max" {
		lim.PIDMax, err = strconv.ParseInt(strings.TrimSpace(pids), 10, 64)
		if err != nil {
			return Limits{}, fmt.Errorf("parse pids.max: %w", err)
		}
	}
	return lim, nil
}

// FindContainerCgroup mencari direktori cgroup milik container (driver systemd
// "docker-<id>.scope" dulu, lalu fallback nama id penuh/pendek).
func FindContainerCgroup(root, containerID string) (string, error) {
	short := containerID
	if len(short) > 12 {
		short = short[:12]
	}
	var exact, partial string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || (exact != "" && partial != "") {
			return nil
		}
		base := filepath.Base(p)
		switch {
		case base == "docker-"+containerID+".scope" || base == containerID:
			exact = p
		case partial == "" && base == short:
			partial = p
		}
		return nil
	})
	if exact != "" {
		return exact, nil
	}
	if partial != "" {
		return partial, nil
	}
	return "", fmt.Errorf("cgroup untuk container %s tidak ditemukan", short)
}

func writeCgroupFile(dir, name, content string) error {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		return fmt.Errorf("tulis %s: %w", name, err)
	}
	return nil
}

func readCgroupFile(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("baca %s: %w", name, err)
	}
	return string(data), nil
}
