package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

func FlakeArchive(stdout, stderr io.Writer) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Inside the Docker container, the export is hard coded to /nix-export/nix-store/
	cmd := exec.Command(nixPath, "flake", "archive", "--to", "file:///nix-export/nix-store/")
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
