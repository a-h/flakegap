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

	cmd := exec.Command(nixPath, "build", "--no-link", "--impure", ref)
	// NIXPKGS_ALLOW_UNFREE is required for nix to build unfree packages such as Terraform.
	// HOME is required for git to find the user's global gitconfig.
	cmd.Env = append(cmd.Env, "NIXPKGS_ALLOW_UNFREE=1", "HOME=/root")
	cmd.Dir = "/code"
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	return cmd.Run()
}
