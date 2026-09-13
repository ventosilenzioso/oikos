package resource

import "errors"

// Limits are resource ceilings; zero means unlimited.
type Limits struct {
	CPUMillicores int64
	MemoryBytes   int64
	PIDMax        int64
}

// ErrNotSupported indicates that resource enforcement is unavailable.
var ErrNotSupported = errors.New("resource limits not supported on this platform")

// LimitEnforcer applies and reads runtime resource limits.
type LimitEnforcer interface {
	Apply(string, Limits) error
	Read(string) (Limits, error)
}
