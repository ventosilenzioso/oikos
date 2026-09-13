package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrNotSupported = errors.New("fitur tidak didukung platform")
var ErrSandboxUnavailable = errors.New("sandbox profile tidak tersedia")

type SandboxOptions struct {
	SeccompProfile  string
	AppArmorProfile string
	SELinuxLabel    string
	UserNamespace   bool
	Permissive      bool
}

func LoadDefaultSandboxOptions() (SandboxOptions, error) {
	return LoadDefaultSandboxOptionsForPlatform(runtimePlatform())
}
func LoadDefaultSandboxOptionsForPlatform(platform string) (SandboxOptions, error) {
	if platform != "linux" {
		return SandboxOptions{}, ErrNotSupported
	}
	for _, path := range []string{"deploy/docker/seccomp-default.json", "../../deploy/docker/seccomp-default.json", "../../../deploy/docker/seccomp-default.json"} {
		if _, err := os.Stat(path); err == nil {
			absolute, _ := filepath.Abs(path)
			return SandboxOptions{SeccompProfile: absolute}, nil
		}
	}
	return SandboxOptions{}, fmt.Errorf("%w: deploy/docker/seccomp-default.json", ErrSandboxUnavailable)
}
func runtimePlatform() string { return "linux" }
