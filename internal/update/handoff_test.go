package update

import (
	"context"
	"reflect"
	"testing"
)

type handoffStep struct{ name string }
type handoffRunner struct{ steps *[]handoffStep }

func (r handoffRunner) Run(name string) error {
	*r.steps = append(*r.steps, handoffStep{"health-check"})
	return nil
}

func TestHandoffRunsInRequiredOrder(t *testing.T) {
	steps := []handoffStep{}
	if err := RunHandoff(context.Background(), HandoffActions{
		Backup:      func(context.Context) error { steps = append(steps, handoffStep{"backup"}); return nil },
		HealthCheck: handoffRunner{&steps}, Migrate: func(context.Context) error { steps = append(steps, handoffStep{"migrate"}); return nil },
		Drain:    func(context.Context) error { steps = append(steps, handoffStep{"drain"}); return nil },
		Takeover: func(context.Context) error { steps = append(steps, handoffStep{"takeover"}); return nil },
		Exit:     func(context.Context) error { steps = append(steps, handoffStep{"exit"}); return nil },
	}, "candidate"); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, s := range steps {
		got = append(got, s.name)
	}
	want := []string{"backup", "health-check", "migrate", "drain", "takeover", "exit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order=%v want=%v", got, want)
	}
}
