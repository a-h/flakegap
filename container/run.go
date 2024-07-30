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

func Run(ctx context.Context, log *slog.Logger, imageRef, mode, codePath, nixExportPath, binaryCacheURL string) (err error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := pullImage(ctx, cli, imageRef); err != nil {
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
	platform := &ocispec.Platform{}
	var containerName string
	cont, err := cli.ContainerCreate(ctx, cconf, hconf, nconf, platform, containerName)
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

func pullImage(ctx context.Context, cli *client.Client, imageRef string) (err error) {
	pull, err := cli.ImagePull(ctx, imageRef, image.PullOptions{})
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
