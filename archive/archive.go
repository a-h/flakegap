package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Archive(ctx context.Context, srcPath, tgtPath string) (size int64, err error) {
	f, err := os.Create(tgtPath)
	if err != nil {
		return size, fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)

	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if cancel := ctx.Err(); cancel != nil {
			return cancel
		}
		if err != nil {
			return err
		}

		name, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		hdr := &tar.Header{
			Name:     name,
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
			length, err := io.Copy(tw, data)
			if err != nil {
				return fmt.Errorf("failed to copy file %q: %w", path, err)
			}
			size += length
		}
		return nil
	})
	if err != nil {
		return size, fmt.Errorf("failed to walk source path: %w", err)
	}

	if err := tw.Close(); err != nil {
		return size, fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := zw.Close(); err != nil {
		return size, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return size, nil
}
