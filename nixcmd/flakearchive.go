package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

func FlakeArchive(stdout, stderr io.Writer, codeDir, targetStore string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Inside the Docker container, the export is hard coded to /nix-export/nix-store/
	// So, the targetStore would be file:///nix-export/nix-store/
	cmd := exec.Command(nixPath, "flake", "archive", "--to", targetStore)
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}
