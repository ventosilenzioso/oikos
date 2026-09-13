package security

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestDefaultSeccompProfileIsValidJSON(t *testing.T) {
	data, err := os.ReadFile("../../deploy/docker/seccomp-default.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile map[string]any
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile["defaultAction"] != "SCMP_ACT_ERRNO" {
		t.Fatal(profile)
	}
}

func TestUnsupportedSandboxReturnsExplicitError(t *testing.T) {
	if _, err := LoadDefaultSandboxOptionsForPlatform("windows"); !errors.Is(err, ErrNotSupported) {
		t.Fatal(err)
	}
}
