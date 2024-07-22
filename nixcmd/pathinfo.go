package nixcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

func PathInfo(stdout, stderr io.Writer, ref string) (path string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return "", fmt.Errorf("failed to find nix on path: %v", err)
	}

	// Run the command.
	cmd := exec.Command(nixPath, "path-info", "--json", ref)
	// NIXPKGS_ALLOW_UNFREE is required for nix to build unfree packages such as Terraform.
	// HOME is required for git to find the user's global gitconfig.
	cmd.Env = append(cmd.Env, "NIXPKGS_ALLOW_UNFREE=1", "HOME=/root")
	cmd.Dir = "/code"
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", NewCommandError("failed to run nix path-info", err, string(output))
	}

	// Parse the output.
	var m map[string]any
	err = json.Unmarshal(output, &m)
	if err != nil {
		return "", NewCommandError("failed to parse nix output", err, string(output))
	}
	if len(m) != 1 {
		return "", fmt.Errorf("expected one path, got %d", len(m))
	}
	for k := range m {
		path = k
		break
	}

	return path, nil
}
