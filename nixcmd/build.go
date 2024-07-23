package nixcmd

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func Build(stdout, stderr io.Writer, ref string, substituters []string) error {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Build args.
	cmdArgs := []string{"build", "--no-link", "--impure"}
	if len(substituters) > 0 {
		cmdArgs = append(cmdArgs, "--substituters", strings.Join(substituters, " "))
	}
	cmdArgs = append(cmdArgs, ref)

	// Execute.
	cmd := exec.Command(nixPath, cmdArgs...)
	cmd.Env = getEnv()
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
