package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type Metrics struct {
	Files int
	Dirs  int
}

// Unarchive extracts the .tar.gz file at src into the existing directory dst. Every write goes through an os.Root opened on dst,
// so an entry cannot create or overwrite a file outside dst, even by traversing a symlink extracted earlier.
func Unarchive(ctx context.Context, log *slog.Logger, src, dst string) (m Metrics, err error) {
	file, err := os.Open(src)
	if err != nil {
		return m, fmt.Errorf("failed to open .tar.gz file %q: %w", src, err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return m, err
	}
	defer gzipReader.Close()

	root, err := os.OpenRoot(dst)
	if err != nil {
		return m, fmt.Errorf("failed to open destination %q: %w", dst, err)
	}
	defer root.Close()

	tarReader := tar.NewReader(gzipReader)
	lastLog := time.Now()
	for {
		if ctx.Err() != nil {
			return m, ctx.Err()
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, err
		}
		if time.Since(lastLog) >= 15*time.Second {
			log.Info("Extracting archive", slog.Int("files-extracted", m.Files), slog.Int("dirs-extracted", m.Dirs))
			lastLog = time.Now()
		}

		if !filepath.IsLocal(header.Name) {
			return m, fmt.Errorf("tar contains invalid path: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			m.Dirs++
			if err := root.MkdirAll(header.Name, os.FileMode(header.Mode)); err != nil {
				return m, err
			}
		case tar.TypeReg:
			m.Files++
			if err := root.MkdirAll(filepath.Dir(header.Name), os.ModePerm); err != nil {
				return m, err
			}
			outFile, err := root.OpenFile(header.Name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return m, err
			}
			if err := outFile.Chmod(os.FileMode(header.Mode)); err != nil {
				outFile.Close()
				return m, err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return m, err
			}
			if err := outFile.Close(); err != nil {
				return m, err
			}
		case tar.TypeSymlink:
			m.Files++
			if err := root.MkdirAll(filepath.Dir(header.Name), os.ModePerm); err != nil {
				return m, err
			}
			if err := root.Symlink(header.Linkname, header.Name); err != nil {
				return m, err
			}
		default:
			return m, fmt.Errorf("unknown type: %v in %s", header.Typeflag, header.Name)
		}
	}
	return m, nil
}
