package docker

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/pkg/archive"
)

func (d *dockerRuntime) BuildImage(ctx context.Context, dockerfilePath, imageTag string) error {
	buildCtx, err := archive.TarWithOptions(filepath.Dir(dockerfilePath), &archive.TarOptions{})
	if err != nil {
		return fmt.Errorf("build context: %w", err)
	}
	resp, err := d.cli.ImageBuild(ctx, buildCtx, build.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: filepath.Base(dockerfilePath),
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("image build: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("baca output build: %w", err)
	}
	return nil
}
