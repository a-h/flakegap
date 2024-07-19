package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Args struct {
	// SourcePath to the repo on disk that contains a flake.nix file.
	SourcePath string
	// TargetPath is the path to write the output to.
	TargetPath string
}

func (a Args) Validate() error {
	var errs []error
	if a.SourcePath == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.TargetPath == "" {
		errs = append(errs, fmt.Errorf("target-path is required"))
	}
	return errors.Join(errs...)
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	log.Info("Running container")

	tgtPath, err := os.MkdirTemp("", "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tgtPath)

	if err = runContainer(ctx, args.SourcePath, tgtPath); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	log.Info("Collecting output")
	archivePath := filepath.Join(args.TargetPath, "nix-export.tar.gz")
	if err = archive(ctx, tgtPath, archivePath); err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	log.Info("Complete")
	return nil
}

func runContainer(ctx context.Context, srcPath, tgtPath string) (err error) {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	srcPath, err = filepath.Abs(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute source path: %w", err)
	}
	tgtPath, err = filepath.Abs(tgtPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute target path: %w", err)
	}

	cconf := &container.Config{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Image:        "ghcr.io/a-h/flakegap:latest",
	}
	hconf := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: srcPath,
				Target: "/code",
			},
			{
				Type:   mount.TypeBind,
				Source: tgtPath,
				Target: "/nix-export",
			},
		},
	}
	nconf := &network.NetworkingConfig{}
	platform := &ocispec.Platform{}
	var containerName string
	cont, err := cli.ContainerCreate(ctx, cconf, hconf, nconf, platform, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		respChan, errChan := cli.ContainerWait(ctx, cont.ID, container.WaitConditionNextExit)
		select {
		case resp := <-respChan:
			if resp.Error != nil {
				runErr = fmt.Errorf("container wait error: %v", resp.Error)
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

	wg.Wait()

	return runErr
}

func archive(ctx context.Context, srcPath, tgtPath string) (err error) {
	f, err := os.Create(tgtPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)

	filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		hdr := &tar.Header{
			Name:     path,
			Size:     info.Size(),
			Typeflag: tar.TypeReg,
			Mode:     0644,
		}
		if info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0755
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}
		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", path, err)
			}
			if _, err := io.Copy(tw, data); err != nil {
				return fmt.Errorf("failed to copy file %q: %w", path, err)
			}
		}
		return nil
	})

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return nil
}
