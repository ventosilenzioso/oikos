//go:build linux

package resource

import "os"

// DetectCgroupVersion mendeteksi versi cgroup host: "v2" bila file
// cgroup.controllers ada, selain itu "v1".
func DetectCgroupVersion() string {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		return "v2"
	}
	return "v1"
}
