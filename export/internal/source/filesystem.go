package source

import (
	"context"
	"fmt"
	"os"
)

// Filesystem copies the source directory as it is on disk, including files ignored by .gitignore.
type Filesystem struct {
	path   string
	ignore []string
}

func NewFilesystem(path string) Filesystem {
	return Filesystem{
		path:   path,
		ignore: []string{".direnv", "nix-export", "nix-export.tar.gz", "result", "coverage.out", ".DS_Store"},
	}
}

// Validate accepts any directory.
func (s Filesystem) Validate() error {
	return nil
}

// CopyTo copies the source directory into dir, skipping build outputs and previous exports.
func (s Filesystem) CopyTo(ctx context.Context, dir string) (err error) {
	if err = os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create source output directory: %w", err)
	}
	if err = os.CopyFS(dir, newFilteredFS(os.DirFS(s.path), s.ignore)); err != nil {
		return fmt.Errorf("failed to copy source directory: %w", err)
	}
	return nil
}
