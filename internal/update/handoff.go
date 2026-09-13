package update

import "context"

type HandoffActions struct {
	Backup      func(context.Context) error
	HealthCheck HealthRunner
	Migrate     func(context.Context) error
	Drain       func(context.Context) error
	Takeover    func(context.Context) error
	Exit        func(context.Context) error
}

func RunHandoff(ctx context.Context, actions HandoffActions, candidate string) error {
	if err := actions.Backup(ctx); err != nil {
		return err
	}
	if err := actions.HealthCheck.Run(candidate); err != nil {
		return err
	}
	if err := actions.Migrate(ctx); err != nil {
		return err
	}
	if err := actions.Drain(ctx); err != nil {
		return err
	}
	if err := actions.Takeover(ctx); err != nil {
		return err
	}
	return actions.Exit(ctx)
}
