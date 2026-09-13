package docker

import "testing"

func TestUint64ToInt64Clamps(t *testing.T) {
	if got := safeInt64(^uint64(0)); got != int64(^uint64(0)>>1) { t.Fatalf("got %d", got) }
	if got := safeInt64(42); got != 42 { t.Fatalf("got %d", got) }
}
