package runtime

import (
	"context"
	"testing"
)

func TestFakeLifecycle(t *testing.T) {
	r := NewFake()
	ctx := context.Background()
	id, err := r.Create(ctx, ContainerSpec{ServerID: "srv-1", Image: "img:test", Command: []string{"run"}, Env: map[string]string{"A": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	st, err := r.Status(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st != StatusRunning {
		t.Fatalf("status = %q", st)
	}
	if err := r.Stop(ctx, id, 5); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, id, false); err != nil {
		t.Fatal(err)
	}
}
