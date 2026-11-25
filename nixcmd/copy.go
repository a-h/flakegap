package nixcmd

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

// CopyFromAll copies all paths from a file:// Nix binary cache to the local
// store. Paths are imported in dependency order to satisfy reference checks,
// working around nix copy --all not guaranteeing topological ordering.
func CopyFromAll(log *slog.Logger, stdout, stderr io.Writer, codeDir, sourceStore string) (err error) {
	storeDir := strings.TrimPrefix(sourceStore, "file://")
	paths, err := sortStorePathsByDependency(storeDir)
	if err != nil {
		return fmt.Errorf("failed to get store paths in dependency order: %w", err)
	}
	const batchSize = 500
	total := len(paths)
	for i := 0; i < total; i += batchSize {
		batch := paths[i:min(i+batchSize, total)]
		if err = copyFromPaths(stdout, stderr, codeDir, sourceStore, batch); err != nil {
			return err
		}
		log.Info("Copying paths to Nix store", slog.Int("paths-copied", i+len(batch)), slog.Int("total-paths", total))
	}
	log.Info("Copied all paths to Nix store", slog.Int("total-paths", total))
	return nil
}

func copyFromPaths(stdout, stderr io.Writer, codeDir, sourceStore string, paths []string) error {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %w", err)
	}
	// --no-check-sigs: do not verify signatures on the source store paths.
	// --option require-sigs false: allow the destination (local) store to accept
	// paths that have no trusted signature; without this, nix.conf's require-sigs
	// setting rejects unsigned paths even when --no-check-sigs is present.
	// --offline: do not contact substituters (e.g. cache.nixos.org) since the
	// source is a local file:// binary cache in an airgapped environment.
	args := append([]string{"copy", "--no-check-sigs", "--option", "require-sigs", "false", "--offline", "--from", sourceStore}, paths...)
	cmd := exec.Command(nixPath, args...)
	cmd.Dir = codeDir
	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}

func CopyTo(stdout, stderr io.Writer, codeDir, targetStore string, derivation bool, paths ...string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	args := []string{"copy", "--to", targetStore}
	if derivation {
		args = append(args, "--derivation")
	}
	args = append(args, paths...)
	cmd := exec.Command(nixPath, args...)
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}
