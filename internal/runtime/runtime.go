// Package runtime mendefinisikan kontrak container engine (Docker, Podman, dll).
// Operations are idempotent whenever the underlying runtime permits it.
package runtime

import "context"

type ContainerSpec struct {
	ServerID          string
	Image             string
	Command           []string
	Env               map[string]string
	CPULimit          int64 // milicores
	MemoryLimit       int64 // bytes
	PIDLimit          int64
	WorkingDir        string
	MountSource       string // host path for the server data directory
	SeccompProfile    string
	AppArmorProfile   string
	SELinuxLabel      string
	UserNamespace     bool
	PermissiveSandbox bool
}

type ContainerStatus string

const (
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusCrashed ContainerStatus = "crashed"
)

type Stats struct {
	CPUPercent    float64
	MemoryUsedMB  int64
	MemoryLimitMB int64
	NetRxBytes    int64
	NetTxBytes    int64
}

// Runtime is the contract implemented by each container engine.
type Runtime interface {
	BuildImage(ctx context.Context, dockerfilePath, imageTag string) error
	Create(ctx context.Context, spec ContainerSpec) (containerID string, err error)
	Start(ctx context.Context, containerID string) error
	Stop(ctx context.Context, containerID string, timeoutSec int) error
	Restart(ctx context.Context, containerID string) error
	Delete(ctx context.Context, containerID string, force bool) error
	Status(ctx context.Context, containerID string) (ContainerStatus, error)
	Stats(ctx context.Context, containerID string) (Stats, error)
	Exec(ctx context.Context, containerID string, cmd []string) (output string, err error)
	Logs(ctx context.Context, containerID string, follow bool) (<-chan string, error)
}
