package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Metrics struct {
	Files int
	Dirs  int
}

func Unarchive(ctx context.Context, src, dst string) (m Metrics, err error) {
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

	tarReader := tar.NewReader(gzipReader)
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

		if strings.Contains(header.Name, "..") {
			return m, fmt.Errorf("tar contains invalid path: %s", header.Name)
		}
		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			m.Dirs++
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return m, err
			}
		case tar.TypeReg:
			m.Files++
			if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
				return m, err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return m, err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return m, err
			}
			outFile.Close()
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				return m, err
			}
		default:
			return m, fmt.Errorf("unknown type: %v in %s", header.Typeflag, header.Name)
		}
	}
	return m, nil
}
