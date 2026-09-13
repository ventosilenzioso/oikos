//go:build linux

package resource

import "os"

// DetectCgroupVersion detects the host cgroup version: "v2" when
// cgroup.controllers exists, and "v1" otherwise.
func DetectCgroupVersion() string {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return "v2"
	}
	return "v1"
}
