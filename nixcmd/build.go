package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

func Build(stdout, stderr io.Writer, ref string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.Command(nixPath, "build", ref)
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
