package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerWritesRequiredJSONFields(t *testing.T) {
	var buf bytes.Buffer
	NewLogger("test.component", &buf).Info("hello")
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"time", "level", "component", "message"} {
		if got[key] == nil {
			t.Fatalf("missing %s: %v", key, got)
		}
	}
	if got["component"] != "test.component" || got["message"] != "hello" {
		t.Fatalf("log=%v", got)
	}
}

func TestLoggerDoesNotWriteSecret(t *testing.T) {
	var buf bytes.Buffer
	NewLogger("security", &buf).Info("pairing token rejected", "token", "super-secret")
	if strings.Contains(buf.String(), "super-secret") {
		t.Fatal("secret leaked")
	}
}
