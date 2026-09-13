package security

import (
	"errors"
	"testing"
)

func TestApplyUserNamespaceModes(t *testing.T) {
	status := UserNamespaceStatus{Supported: true, Mode: "private"}
	if got, err := ApplyUserNamespace(status, true, false); err != nil || !got.Enabled {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := ApplyUserNamespace(UserNamespaceStatus{Supported: false}, true, false); !errors.Is(err, ErrNotSupported) {
		t.Fatal(err)
	}
	if got, err := ApplyUserNamespace(UserNamespaceStatus{Supported: false}, true, true); err != nil || got.Enabled {
		t.Fatalf("permissive got=%+v err=%v", got, err)
	}
}
