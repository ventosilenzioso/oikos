package tunnel

import "testing"

func TestValidPortFitsProtoInt32(t *testing.T) {
	for _, port := range []int{1, 65535} { if err := validateLocalPort(port); err != nil { t.Fatal(err) } }
	for _, port := range []int{0, 65536, 1<<31} { if err := validateLocalPort(port); err == nil { t.Fatalf("port %d accepted", port) } }
}
