//go:build integration

package backup

import (
	"os"
	"testing"
)

func TestHotBackupIntegration(t *testing.T) {
	if os.Getenv("OIKOS_FILESYSTEM_INTEGRATION") != "1" {
		t.Skip("filesystem integration dilewati: set OIKOS_FILESYSTEM_INTEGRATION=1")
	}
	t.Skip("large-file hot backup benchmark membutuhkan fixture host yang disediakan deployment test")
}
