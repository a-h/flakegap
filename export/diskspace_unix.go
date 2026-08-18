//go:build darwin || linux

package export

import (
	"fmt"
	"path/filepath"
	"syscall"
)

func availableBytes(path string) (uint64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve path: %w", err)
	}
	var stat syscall.Statfs_t
	if err = syscall.Statfs(abs, &stat); err != nil {
		return 0, fmt.Errorf("failed to stat filesystem at %q: %w", abs, err)
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
