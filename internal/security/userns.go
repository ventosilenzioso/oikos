package security

import "fmt"

type UserNamespaceStatus struct {
	Supported bool
	Enabled   bool
	Mode      string
}

func DetectUserNamespace(supported bool) UserNamespaceStatus {
	return UserNamespaceStatus{Supported: supported, Enabled: false, Mode: "host"}
}

func ApplyUserNamespace(status UserNamespaceStatus, requested, permissive bool) (UserNamespaceStatus, error) {
	if !requested {
		return status, nil
	}
	if !status.Supported {
		if permissive {
			status.Mode = "host"
			status.Enabled = false
			return status, nil
		}
		return status, fmt.Errorf("%w: user namespace", ErrNotSupported)
	}
	status.Enabled = true
	status.Mode = "private"
	return status, nil
}
