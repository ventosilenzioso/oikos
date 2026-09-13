package update

import "testing"

func TestVersionsToRemovePreservesCurrentPreviousAndNewest(t *testing.T) {
	versions := []string{"v1", "v2", "v3", "v4"}
	got := VersionsToRemove(versions, "v4", "v3", 1)
	if len(got) != 1 || got[0] != "v1" {
		t.Fatalf("remove=%v", got)
	}
}

func TestVersionsToRemoveDoesNotRemoveProtectedSlots(t *testing.T) {
	got := VersionsToRemove([]string{"v1", "v2"}, "v1", "v2", 0)
	if len(got) != 0 {
		t.Fatalf("remove=%v", got)
	}
}
