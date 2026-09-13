//go:build !linux

package resource

// UnsupportedLimitEnforcer makes unsupported platforms explicit to callers.
type UnsupportedLimitEnforcer struct{}

func (UnsupportedLimitEnforcer) Apply(string, Limits) error  { return ErrNotSupported }
func (UnsupportedLimitEnforcer) Read(string) (Limits, error) { return Limits{}, ErrNotSupported }

// NewLimitEnforcer returns the unsupported implementation.
func NewLimitEnforcer() LimitEnforcer { return UnsupportedLimitEnforcer{} }
