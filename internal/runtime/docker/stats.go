package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/oikos/oikos/internal/runtime"
)

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func safeInt64(value uint64) int64 {
	const maxInt64 = uint64(^uint64(0) >> 1)
	if value > maxInt64 {
		return int64(maxInt64)
	}
	return int64(value)
}

func (d *dockerRuntime) Stats(ctx context.Context, containerID string) (runtime.Stats, error) {
	resp, err := d.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return runtime.Stats{}, fmt.Errorf("container stats: %w", err)
	}
	defer resp.Body.Close()
	var s dockerStats
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return runtime.Stats{}, fmt.Errorf("decode stats: %w", err)
	}
	var cpu float64
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemCPUUsage - s.PreCPUStats.SystemCPUUsage)
	if sysDelta > 0 && cpuDelta > 0 {
		cpus := float64(s.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1
		}
		cpu = (cpuDelta / sysDelta) * cpus * 100.0
	}
	var rx, tx uint64
	for _, n := range s.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	return runtime.Stats{
		CPUPercent:    cpu,
		MemoryUsedMB:  safeInt64(s.MemoryStats.Usage / 1024 / 1024),
		MemoryLimitMB: safeInt64(s.MemoryStats.Limit / 1024 / 1024),
		NetRxBytes:    safeInt64(rx),
		NetTxBytes:    safeInt64(tx),
	}, nil
}

func (d *dockerRuntime) Logs(ctx context.Context, containerID string, follow bool) (<-chan string, error) {
	reader, err := d.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
	})
	if err != nil {
		return nil, fmt.Errorf("container logs: %w", err)
	}
	ch := make(chan string, 100)
	go func() {
		defer close(ch)
		defer reader.Close()
		pr, pw := io.Pipe()
		go func() {
			_, copyErr := stdcopy.StdCopy(pw, pw, reader)
			_ = pw.CloseWithError(copyErr)
		}()
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r\n")
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
