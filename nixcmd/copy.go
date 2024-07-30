package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

func CopyFrom(stdout, stderr io.Writer) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.Command(nixPath, "copy", "--all", "--offline", "--impure", "--no-check-sigs", "--from", "file:///nix-export/nix-store/")
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}

func CopyTo(stdout, stderr io.Writer) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.Command(nixPath, "copy", "--derivation", "--all", "--to", "file:///nix-export/nix-store/")
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
