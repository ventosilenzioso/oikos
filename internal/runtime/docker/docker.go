package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/oikos/oikos/internal/runtime"
)

type dockerRuntime struct {
	cli *client.Client
}

// New membuat Runtime berbasis Docker. host kosong berarti baca dari environment
// (mis. DOCKER_HOST atau socket default).
func New(host string) (runtime.Runtime, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if host == "" {
		opts = append(opts, client.FromEnv)
	} else {
		opts = append(opts, client.WithHost(host))
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &dockerRuntime{cli: cli}, nil
}

func Ping(host string) error {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if host == "" {
		opts = append(opts, client.FromEnv)
	} else {
		opts = append(opts, client.WithHost(host))
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	_, err = cli.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("docker ping: %w", err)
	}
	return nil
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

func (d *dockerRuntime) Create(ctx context.Context, spec runtime.ContainerSpec) (string, error) {
	cfg := &container.Config{
		Image:      spec.Image,
		Cmd:        spec.Command,
		Env:        envList(spec.Env),
		WorkingDir: spec.WorkingDir,
		Labels:     map[string]string{"oikos.server_id": spec.ServerID},
	}
	hostCfg := &container.HostConfig{
		Resources: container.Resources{
			Memory:    spec.MemoryLimit,
			NanoCPUs:  spec.CPULimit * 1_000_000, // milicores -> NanoCPUs
			PidsLimit: &spec.PIDLimit,
		},
	}
	if spec.SeccompProfile != "" {
		profile, err := os.ReadFile(spec.SeccompProfile)
		if err != nil {
			return "", fmt.Errorf("read seccomp profile: %w", err)
		}
		hostCfg.SecurityOpt = append(hostCfg.SecurityOpt, "seccomp="+string(profile))
	}
	if spec.AppArmorProfile != "" {
		hostCfg.SecurityOpt = append(hostCfg.SecurityOpt, "apparmor="+spec.AppArmorProfile)
	}
	if spec.SELinuxLabel != "" {
		hostCfg.SecurityOpt = append(hostCfg.SecurityOpt, "label="+spec.SELinuxLabel)
	}
	if spec.UserNamespace {
		hostCfg.UsernsMode = "private"
	}
	if spec.MountSource != "" {
		hostCfg.Binds = []string{spec.MountSource + ":/data"}
	}
	resp, err := d.cli.ContainerCreate(ctx, cfg, hostCfg, &network.NetworkingConfig{}, nil, "oikos-"+spec.ServerID)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	return resp.ID, nil
}

func (d *dockerRuntime) Start(ctx context.Context, containerID string) error {
	if err := d.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start: %w", err)
	}
	return nil
}

func (d *dockerRuntime) Stop(ctx context.Context, containerID string, timeoutSec int) error {
	if err := d.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSec}); err != nil {
		return fmt.Errorf("container stop: %w", err)
	}
	return nil
}

func (d *dockerRuntime) Restart(ctx context.Context, containerID string) error {
	timeout := 10
	if err := d.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("container restart: %w", err)
	}
	return nil
}

func (d *dockerRuntime) Delete(ctx context.Context, containerID string, force bool) error {
	if err := d.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("container delete: %w", err)
	}
	return nil
}

func (d *dockerRuntime) Status(ctx context.Context, containerID string) (runtime.ContainerStatus, error) {
	info, err := d.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("container inspect: %w", err)
	}
	if info.State == nil {
		return runtime.StatusStopped, nil
	}
	if info.State.Running {
		return runtime.StatusRunning, nil
	}
	// 137 (SIGKILL) dan 143 (SIGTERM) adalah hasil normal dari Stop,
	// bukan crash. OOMKilled dicek terpisah karena juga berkode 137.
	if info.State.OOMKilled {
		return runtime.StatusCrashed, nil
	}
	switch info.State.ExitCode {
	case 0, 137, 143:
		return runtime.StatusStopped, nil
	default:
		return runtime.StatusCrashed, nil
	}
}

func (d *dockerRuntime) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	execResp, err := d.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}
	attach, err := d.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer attach.Close()
	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, attach.Reader); err != nil {
		return "", fmt.Errorf("exec baca output: %w", err)
	}
	inspect, err := d.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return "", fmt.Errorf("exec inspect: %w", err)
	}
	if inspect.ExitCode != 0 {
		return out.String(), fmt.Errorf("exec exit code %d", inspect.ExitCode)
	}
	return out.String(), nil
}
