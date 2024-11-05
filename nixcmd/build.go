package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
)

// Build the flake reference that can be found in codeDir.
func Build(stdout, stderr io.Writer, codeDir, ref string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Execute.
	cmd := exec.Command(nixPath, "build", ref)
	cmd.Env = getEnv()
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}
