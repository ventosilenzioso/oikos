package security

import (
	"errors"
	"os"
	"strings"
)

type LSMStatus struct {
	AppArmorAvailable bool
	SELinuxAvailable  bool
	SELinuxMode       string
}
type fileReader func(string) ([]byte, error)

func DetectLSM() LSMStatus { return DetectLSMFromFiles(os.ReadFile) }
func DetectLSMFromFiles(read fileReader) LSMStatus {
	status := LSMStatus{}
	if _, err := read("/sys/kernel/security/apparmor"); err == nil {
		status.AppArmorAvailable = true
	}
	if _, err := read("/sys/fs/selinux/enforce"); err == nil {
		status.SELinuxAvailable = true
		mode, _ := read("/sys/fs/selinux/enforce")
		if strings.TrimSpace(string(mode)) == "1" {
			status.SELinuxMode = "enforcing"
		} else {
			status.SELinuxMode = "permissive"
		}
	}
	return status
}

func ValidateLSMProfile(status LSMStatus, profile string) error {
	switch profile {
	case "apparmor":
		if !status.AppArmorAvailable {
			return ErrNotSupported
		}
	case "selinux":
		if !status.SELinuxAvailable {
			return ErrNotSupported
		}
	default:
		return errors.New("unknown LSM profile")
	}
	return nil
}
