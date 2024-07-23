package nixcmd

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

func Run(ctx context.Context, stdout, stderr io.Writer, ref string, args ...string) (cmd *exec.Cmd, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return cmd, fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmdArgs := append([]string{"run", ref}, args...)
	cmd = exec.CommandContext(ctx, nixPath, cmdArgs...)
	cmd.Env = getEnv()
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd, cmd.Start()
}
