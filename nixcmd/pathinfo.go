package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

func PathInfo(stdout, stderr io.Writer, ref string) (path string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return path, fmt.Errorf("failed to find nix on path: %w", err)
	}

	stdoutBuffer := new(bytes.Buffer)

	cmd := exec.Command(nixPath, "path-info", "--json", ref)
	// NIXPKGS_ALLOW_UNFREE is required for nix to build unfree packages such as Terraform.
	// HOME is required for git to find the user's global gitconfig.
	cmd.Env = append(cmd.Env, "NIXPKGS_ALLOW_UNFREE=1", "HOME=/root")
	cmd.Stdout = io.MultiWriter(stdoutBuffer, stdout)
	cmd.Stderr = stderr
	cmd.Dir = "/code"
	if err = cmd.Run(); err != nil {
		return path, fmt.Errorf("failed to run nix path-info: %w", err)
	}

	// Parse the output.
	var m map[string]any
	err = json.Unmarshal(stdoutBuffer.Bytes(), &m)
	if err != nil {
		return path, fmt.Errorf("failed to parse nix path-info output: %w", err)
	}
	if len(m) != 1 {
		return path, fmt.Errorf("expected one path, got %d", len(m))
	}
	for k := range m {
		path = k
		break
	}

	return path, nil
}
