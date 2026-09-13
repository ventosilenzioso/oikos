package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type fake struct {
	mu         sync.Mutex
	containers map[string]ContainerStatus
	seq        int
}

func NewFake() Runtime {
	return &fake{containers: map[string]ContainerStatus{}}
}

func (f *fake) BuildImage(ctx context.Context, dockerfilePath, imageTag string) error {
	return nil
}

func (f *fake) Create(ctx context.Context, spec ContainerSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("fake-%d", f.seq)
	f.containers[id] = StatusStopped
	return id, nil
}

func (f *fake) Start(ctx context.Context, containerID string) error {
	return f.set(containerID, StatusRunning)
}

func (f *fake) Stop(ctx context.Context, containerID string, timeoutSec int) error {
	return f.set(containerID, StatusStopped)
}

func (f *fake) Restart(ctx context.Context, containerID string) error {
	return f.set(containerID, StatusRunning)
}

func (f *fake) Delete(ctx context.Context, containerID string, force bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.containers, containerID)
	return nil
}

func (f *fake) Status(ctx context.Context, containerID string) (ContainerStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.containers[containerID]
	if !ok {
		return "", errors.New("container tidak ditemukan")
	}
	return st, nil
}

func (f *fake) Stats(ctx context.Context, containerID string) (Stats, error) {
	return Stats{}, nil
}

func (f *fake) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", nil
}

func (f *fake) Logs(ctx context.Context, containerID string, follow bool) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (f *fake) set(id string, st ContainerStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.containers[id]; !ok {
		return errors.New("container tidak ditemukan")
	}
	f.containers[id] = st
	return nil
}
