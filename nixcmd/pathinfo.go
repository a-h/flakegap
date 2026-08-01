package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

// nix copy --to file://$PWD/nix-export/nix-store `nix-store --realise $(nix path-info --recursive --derivation .#)`
func PathInfo(stdout, stderr io.Writer, codeDir string, recursive, derivation bool, ref string) (paths []string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return paths, fmt.Errorf("failed to find nix on path: %w", err)
	}

	base := []string{"path-info", "--json"}
	if recursive {
		base = append(base, "--recursive")
	}
	if derivation {
		base = append(base, "--derivation")
	}

	args := append(slices.Clone(base), "--json-format", "1", ref)
	outBytes, errBytes, runErr := runPathInfo(nixPath, codeDir, args)
	if runErr != nil {
		if strings.Contains(string(errBytes), "unrecognised flag") {
			// --json-format is not supported by this Nix version; retry without it.
			args = append(base, ref)
			outBytes, errBytes, runErr = runPathInfo(nixPath, codeDir, args)
		}
		if runErr != nil {
			stderr.Write(errBytes) //nolint
			return paths, fmt.Errorf("failed to run nix %s: %w", strings.Join(args, " "), runErr)
		}
	}
	stderr.Write(errBytes) //nolint

	paths, err = getPathInfo(outBytes)
	if err != nil {
		return paths, fmt.Errorf("failed to get path info from nix %s: %w", strings.Join(args, " "), err)
	}
	return paths, nil
}

func runPathInfo(nixPath, codeDir string, args []string) (out []byte, errOut []byte, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(nixPath, args...)
	cmd.Env = getEnv()
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = codeDir
	err = cmd.Run()
	return outBuf.Bytes(), errBuf.Bytes(), err
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
		slices.Sort(paths)
		return paths, nil
	}

	return paths, fmt.Errorf("unexpected output: %s", string(stdout))
}

type pathInfoOutput struct {
	Path string `json:"path"`
}
