package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var linuxAMD64 = Platform{
	Architecture: "amd64",
	OS:           "linux",
}

var linuxARM64 = Platform{
	Architecture: "arm64",
	OS:           "linux",
}

var platforms = map[string]Platform{
	"linux/amd64":   linuxAMD64,
	"amd64":         linuxAMD64,
	"x86_64":        linuxAMD64,
	"x86_64-linux":  linuxAMD64,
	"linux/arm64":   linuxARM64,
	"arm64":         linuxARM64,
	"aarch64":       linuxARM64,
	"aarch64-linux": linuxARM64,
}

func NewPlatform(s string) (Platform, error) {
	p, ok := platforms[s]
	if !ok {
		return Platform{}, fmt.Errorf("unknown platform %q", s)
	}
	return p, nil
}

type Platform struct {
	Architecture string
	OS           string
}

func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Architecture)
}

func Run(ctx context.Context, log *slog.Logger, imageRef, mode, codePath, nixExportPath, binaryCacheURL string, platform Platform) (err error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := pullImage(ctx, cli, imageRef, platform); err != nil {
		log.Warn("failed to pull image", slog.String("image", imageRef), slog.Any("error", err))
	}

	codePath, err = filepath.Abs(codePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}
	nixExportPath, err = filepath.Abs(nixExportPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute target path: %w", err)
	}

	entrypoint := []string{"/usr/local/bin/runtime", mode}
	if mode == "export" && binaryCacheURL != "" {
		entrypoint = append(entrypoint, "-substituter", binaryCacheURL)
	}

	cconf := &container.Config{
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Image:        imageRef,
		Entrypoint:   entrypoint,
	}
	if mode == "validate" {
		cconf.NetworkDisabled = true
	}
	hconf := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: codePath,
				Target: "/code",
			},
			{
				Type:   mount.TypeBind,
				Source: nixExportPath,
				Target: "/nix-export",
			},
		},
	}
	if mode == "export" {
		// Enable host-based networking so that the container can connect back to the host's binary cache.
		hconf.NetworkMode = "host"
	}
	nconf := &network.NetworkingConfig{}
	p := &ocispec.Platform{
		Architecture: platform.Architecture,
		OS:           platform.OS,
	}
	var containerName string
	cont, err := cli.ContainerCreate(ctx, cconf, hconf, nconf, p, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	var wg sync.WaitGroup

	// Wait for the container to finish and collect any errors.
	var runErr, logErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		respChan, errChan := cli.ContainerWait(ctx, cont.ID, container.WaitConditionNextExit)
		select {
		case resp := <-respChan:
			if resp.Error != nil {
				runErr = fmt.Errorf("container wait error: %v", resp.Error)
			}
			if resp.StatusCode != 0 {
				runErr = fmt.Errorf("container exited with non-zero status: %d", resp.StatusCode)
			}
		case err := <-errChan:
			runErr = fmt.Errorf("container wait error: %w", err)
		case <-ctx.Done():
			runErr = fmt.Errorf("container wait cancelled")
		}
	}()

	if err := cli.ContainerStart(ctx, cont.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Stream the logs, now that the container is running.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := cli.ContainerLogs(ctx, cont.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		})
		if err != nil {
			logErr = fmt.Errorf("failed to get container logs: %w", err)
		}
		defer r.Close()
		_, logErr = io.Copy(os.Stdout, r)
	}()

	wg.Wait()

	return errors.Join(runErr, logErr)
}

func pullImage(ctx context.Context, cli *client.Client, imageRef string, platform Platform) (err error) {
	pull, err := cli.ImagePull(ctx, imageRef, image.PullOptions{
		Platform: platform.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer pull.Close()
	_, err = io.Copy(os.Stdout, pull)
	if err != nil {
		return fmt.Errorf("failed to read image pull response: %w", err)
	}
	return nil
}
