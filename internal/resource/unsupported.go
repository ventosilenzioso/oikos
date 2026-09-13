//go:build !linux

package resource

func ApplyLimits(string, Limits) error { return ErrNotSupported }

func ReadLimits(string) (Limits, error) { return Limits{}, ErrNotSupported }

func FindContainerCgroup(string, string) (string, error) { return "", ErrNotSupported }

func DetectCgroupVersion() string { return "unsupported" }
