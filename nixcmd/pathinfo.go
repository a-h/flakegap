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
	var m map[string]any
	err = json.Unmarshal(stdoutBuffer.Bytes(), &m)
	if err != nil {
		return paths, fmt.Errorf("failed to parse nix path-info output: %w", err)
	}
	paths = make([]string, len(m))
	var i int
	for k := range m {
		paths[i] = k
		i++
	}

	return paths, nil
}
