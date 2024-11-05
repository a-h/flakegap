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
func CopyToAll(stdout, stderr io.Writer, targetStore, path string) (err error) {
	if err := CopyTo(stdout, stderr, targetStore, path, true); err != nil {
		return fmt.Errorf("failed to copy derivation: %w", err)
	}
	if err := CopyTo(stdout, stderr, targetStore, path, false); err != nil {
		return fmt.Errorf("failed to copy path: %w", err)
	}
	return nil
}

func CopyTo(stdout, stderr io.Writer, targetStore, path string, derivation bool) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	args := []string{"copy", "--to", targetStore}
	if derivation {
		args = append(args, "--derivation")
	}
	args = append(args, path)
	cmd := exec.Command(nixPath, args...)

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}
