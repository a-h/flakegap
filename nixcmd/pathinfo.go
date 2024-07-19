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
	cmd.Dir = "/code"
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run nix: %v", err)
	}

	// Parse the output.
	var m map[string]any
	err = json.Unmarshal(output, &m)
	if err != nil {
		return "", fmt.Errorf("failed to parse nix output %q: %v", string(output), err)
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
