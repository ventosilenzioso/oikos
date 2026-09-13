package security

import (
	"errors"
	"os"
	"testing"
)

func TestDetectLSMFromFixtures(t *testing.T) {
	status := DetectLSMFromFiles(func(path string) ([]byte, error) {
		switch path {
		case "/sys/kernel/security/apparmor":
			return []byte("1"), nil
		case "/sys/fs/selinux/enforce":
			return []byte("1\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	})
	if !status.AppArmorAvailable || !status.SELinuxAvailable || status.SELinuxMode != "enforcing" {
		t.Fatalf("status=%+v", status)
	}
}

func TestValidateLSMProfileFallback(t *testing.T) {
	status := LSMStatus{}
	if err := ValidateLSMProfile(status, "apparmor"); !errors.Is(err, ErrNotSupported) {
		t.Fatal(err)
	}
}
