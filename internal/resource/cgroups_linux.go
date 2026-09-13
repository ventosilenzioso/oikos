//go:build linux

package resource

// LinuxLimitEnforcer applies cgroup v2 limits.
type LinuxLimitEnforcer struct{}

func (LinuxLimitEnforcer) Apply(dir string, limits Limits) error { return ApplyLimits(dir, limits) }
func (LinuxLimitEnforcer) Read(dir string) (Limits, error)       { return ReadLimits(dir) }

// NewLimitEnforcer returns the Linux cgroup implementation.
func NewLimitEnforcer() LimitEnforcer { return LinuxLimitEnforcer{} }
