//go:build !linux

package resource

import (
	"errors"
	"testing"
)

func TestUnsupportedLimitsReturnErrNotSupported(t *testing.T) {
	enforcer := NewLimitEnforcer()
	if err := enforcer.Apply("unused", Limits{}); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Apply error = %v, want ErrNotSupported", err)
	}
	if _, err := enforcer.Read("unused"); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("Read error = %v, want ErrNotSupported", err)
	}
}
