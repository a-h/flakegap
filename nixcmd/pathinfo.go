package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
		return paths, fmt.Errorf("failed to run nix %s: %w", strings.Join(args, " "), err)
	}

	paths, err = getPathInfo(stdoutBuffer.Bytes())
	if err != nil {
		return paths, fmt.Errorf("failed to get path info from nix %s: %w", strings.Join(args, " "), err)
	}
	return paths, nil
}

func getPathInfo(stdout []byte) (paths []string, err error) {
	if len(stdout) == 0 {
		return paths, fmt.Errorf("empty nix path-info output")
	}
	switch string(stdout[:1]) {
	case "[":
		var op []pathInfoOutput
		err = json.Unmarshal(stdout, &op)
		if err != nil {
			return paths, err
		}

		paths = make([]string, len(op))
		for i, pio := range op {
			paths[i] = pio.Path
		}
		return paths, nil
	case "{":
		var pio map[string]any
		err = json.Unmarshal(stdout, &pio)
		if err != nil {
			return paths, err
		}

		paths = make([]string, len(pio))
		var i int
		for k := range pio {
			paths[i] = k
			i++
		}
		return paths, nil
	}

	return paths, fmt.Errorf("unexpected output: %s", string(stdout))
}

type pathInfoOutput struct {
	Path string `json:"path"`
}
