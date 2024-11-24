package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// nix copy --to file://$PWD/nix-export/nix-store `nix-store --realise $(nix path-info --recursive --derivation .#)`
func PathInfo(stdout, stderr io.Writer, codeDir string, recursive, derivation bool, ref string) (paths []string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return paths, fmt.Errorf("failed to find nix on path: %w", err)
	}

	stdoutBuffer := new(bytes.Buffer)

	args := []string{"path-info", "--json"}
	if recursive {
		args = append(args, "--recursive")
	}
	if derivation {
		args = append(args, "--derivation")
	}
	args = append(args, ref)
	cmd := exec.Command(nixPath, args...)
	cmd.Env = getEnv()
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderr
	cmd.Dir = codeDir
	if err = cmd.Run(); err != nil {
		return paths, fmt.Errorf("failed to run nix path-info: %w", err)
	}

	// Parse the output.
	var op []pathInfoOutput
	err = json.Unmarshal(stdoutBuffer.Bytes(), &op)
	if err != nil {
		return paths, fmt.Errorf("failed to parse nix path-info output: %w", err)
	}
	paths = make([]string, len(op))
	for i, pio := range op {
		paths[i] = pio.Path
	}

	return paths, nil
}

type pathInfoOutput struct {
	Path string `json:"path"`
}
