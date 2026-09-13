package tunnel

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/oikos/oikos/internal/platform"
)

type ProcessController interface {
	Start(context.Context, string) error
	Stop(context.Context) error
	Reload(context.Context, string) error
	Wait() <-chan error
}

type osProcessController struct {
	binary string
	mu     sync.Mutex
	cmd    *exec.Cmd
	wait   chan error
}

func NewOSProcessController(binary string) ProcessController {
	return &osProcessController{binary: binary}
}

func (p *osProcessController) Start(ctx context.Context, configPath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, p.binary, "-c", configPath)
	if err := platform.NewProcessGroup().Configure(cmd); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start frpc: %w", err)
	}
	p.cmd = cmd
	p.wait = make(chan error, 1)
	go func() { p.wait <- cmd.Wait(); close(p.wait) }()
	return nil
}

func (p *osProcessController) Stop(ctx context.Context) error {
	p.mu.Lock()
	cmd := p.cmd
	wait := p.wait
	p.cmd = nil
	p.wait = nil
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = platform.NewProcessGroup().Terminate(cmd)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-wait:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		<-wait
		return nil
	}
}

func (p *osProcessController) Reload(ctx context.Context, configPath string) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx, configPath)
}

func (p *osProcessController) Wait() <-chan error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.wait == nil {
		return closedErrorChannel()
	}
	return p.wait
}

func closedErrorChannel() <-chan error { ch := make(chan error); close(ch); return ch }

type fakeProcessController struct {
	mu                     sync.Mutex
	starts, reloads, stops int
	wait                   chan error
}

func NewFakeProcessController() ProcessController {
	return &fakeProcessController{wait: make(chan error)}
}
func (p *fakeProcessController) Start(context.Context, string) error {
	p.mu.Lock()
	p.starts++
	p.mu.Unlock()
	return nil
}
func (p *fakeProcessController) Stop(context.Context) error {
	p.mu.Lock()
	p.stops++
	p.mu.Unlock()
	return nil
}
func (p *fakeProcessController) Reload(context.Context, string) error {
	p.mu.Lock()
	p.reloads++
	p.mu.Unlock()
	return nil
}
func (p *fakeProcessController) Wait() <-chan error { return p.wait }
