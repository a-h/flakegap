package nixcmd

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Develop runs the development shell.
// In the case of:
//
//	nix develop --command python --version
//
// The following command will be executed within the development shell:
//
//	python --version
func Develop(ctx context.Context, stdout, stderr io.Writer, codeDir string, args ...string) (err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Execute.
	cmd := exec.Command(nixPath, append([]string{"develop", "--command"}, args...)...)
	cmd.Env = getEnv()
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	cmd.Stdout = w
	return closer(cmd.Run())
}
