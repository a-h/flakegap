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
	Architecture:    "amd64",
	Platform:        "linux",
	NixArchitecture: "x86_64",
}

var linuxARM64 = Platform{
	Architecture:    "arm64",
	Platform:        "linux",
	NixArchitecture: "aarch64",
}

var dockerPlatforms = map[string]Platform{
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
	p, ok := dockerPlatforms[s]
	if !ok {
		return Platform{}, fmt.Errorf("unknown docker platform %q", s)
	}
	return p, nil
}

type Platform struct {
	Architecture string
	Platform     string
	// NixArchitecture is the architecture in Nix system names, e.g. x86_64 in x86_64-linux.
	NixArchitecture string
}

func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.Platform, p.Architecture)
}

// Run validates the export at nixExportPath by building the flake at codePath in a container without network
// access, and running developCommand in each of its development shells.
func Run(ctx context.Context, log *slog.Logger, containerPlatform Platform, imageRef, codePath, nixExportPath, developCommand string) (err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := pullImage(ctx, cli, imageRef, containerPlatform); err != nil {
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

	cconf := &container.Config{
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Image:        imageRef,
		Entrypoint:   []string{"/usr/local/bin/validate"},
		Cmd: []string{
			"-architecture", containerPlatform.NixArchitecture,
			"-platform", containerPlatform.Platform,
			"-develop-command", developCommand,
		},
	}
	cconf.NetworkDisabled = true
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
	nconf := &network.NetworkingConfig{}
	p := &ocispec.Platform{
		Architecture: containerPlatform.Architecture,
		OS:           containerPlatform.Platform,
	}
	var containerName string
	cont, err := cli.ContainerCreate(ctx, cconf, hconf, nconf, p, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	var wg sync.WaitGroup

	// Wait for the container to finish and collect any errors.
	var runErr, logErr error
	wg.Go(func() {
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
			log.Info("Stopping container", slog.String("id", cont.ID))
			// The context is cancelled, so the stop request uses a context without cancellation.
			stopErr := cli.ContainerStop(context.WithoutCancel(ctx), cont.ID, container.StopOptions{})
			if stopErr != nil {
				stopErr = fmt.Errorf("failed to stop container: %w", stopErr)
			}
			runErr = errors.Join(fmt.Errorf("container wait cancelled: %w", ctx.Err()), stopErr)
		}
	})

	if err := cli.ContainerStart(ctx, cont.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Stream the logs, now that the container is running.
	wg.Go(func() {
		r, err := cli.ContainerLogs(ctx, cont.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Timestamps: false,
		})
		if err != nil {
			logErr = fmt.Errorf("failed to get container logs: %w", err)
			return
		}
		defer r.Close()
		_, logErr = io.Copy(os.Stdout, r)
	})

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
