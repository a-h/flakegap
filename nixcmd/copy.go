package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

// CopyFrom copies all the paths from the sourceStore to the local nix store. The sourceStore is usually file:///nix-export/nix-store/.
func CopyFromAll(stdout, stderr io.Writer, codeDir, sourceStore string) (err error) {
	err = CopyFrom(stdout, stderr, codeDir, sourceStore, true)
	if err != nil {
		return fmt.Errorf("failed to copy derivations: %w", err)
	}
	err = CopyFrom(stdout, stderr, codeDir, sourceStore, false)
	if err != nil {
		return fmt.Errorf("failed to copy paths: %w", err)
	}
	return nil
}

// CopyFrom copies all the paths from the sourceStore to the local nix store. The sourceStore is usually file:///nix-export/nix-store/.
func CopyFrom(stdout, stderr io.Writer, codeDir, sourceStore string, derivation bool) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	args := []string{"copy", "--all", "--no-check-sigs"}
	if derivation {
		args = append(args, "--derivation")
	}
	args = append(args, "--from", sourceStore)
	cmd := exec.Command(nixPath, args...)
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}

// CopyToAll copies the paths from the local nix store to the targetStore.
// It copies both the derivations and the paths.
// Then copies the realised derivations.
func CopyToAll(stdout, stderr io.Writer, codeDir, targetStore, ref string) (realisedPathCount int, err error) {
	if err := CopyTo(stdout, stderr, codeDir, targetStore, true, ref); err != nil {
		return realisedPathCount, fmt.Errorf("failed to copy derivation: %w", err)
	}
	if err := CopyTo(stdout, stderr, codeDir, targetStore, false, ref); err != nil {
		return realisedPathCount, fmt.Errorf("failed to copy path: %w", err)
	}
	// We need to copy the realised derivations of the thing we're trying to transfer so that we can build it.
	// nix derivation show .# | jq -r '.[].inputDrvs | keys[]'
	drvs, err := DerivationShow(stdout, stderr, codeDir, ref)
	if err != nil {
		return realisedPathCount, fmt.Errorf("failed to get input derivations: %w", err)
	}
	// nix-store --realise $paths_from_previous_command
	realisedPaths, err := NixStoreRealise(stdout, stderr, targetStore, drvs)
	if err != nil {
		return realisedPathCount, fmt.Errorf("failed to realise derivations: %w", err)
	}
	if len(realisedPaths) == 0 {
		return realisedPathCount, nil
	}
	// Copy the realised paths.
	if err = CopyTo(stdout, stderr, codeDir, targetStore, false, realisedPaths...); err != nil {
		return realisedPathCount, fmt.Errorf("failed to copy realised paths: %w", err)
	}
	return len(realisedPaths), nil
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
