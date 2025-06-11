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

func FlakeShow(stdout, stderr io.Writer, codeDir string) (op FlakeShowOutput, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return op, fmt.Errorf("failed to find nix on path: %w", err)
	}

	stdoutBuffer := new(bytes.Buffer)

	cmd := exec.Command(nixPath, "flake", "show", "--json", "--all-systems")
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stdout = io.MultiWriter(stdoutBuffer, w)
	cmd.Stderr = w
	if err = closer(cmd.Run()); err != nil {
		return op, fmt.Errorf("failed to run nix flake show: %w", err)
	}

	err = json.Unmarshal(stdoutBuffer.Bytes(), &op)
	if err != nil {
		return op, fmt.Errorf("failed to parse nix flake show output: %w", err)
	}

	return op, err
}

type FlakeShowOutput map[string]any

// Derivations returns the derivations for the given architecture and platform, e.g. "x86_64-linux".
func (fso FlakeShowOutput) Derivations(architecture, platform string) (matches []string) {
	matches = findDerivation(fmt.Sprintf("%s-%s", architecture, platform), []string{}, fso)
	slices.Sort(matches)
	return matches
}

func findDerivation(architectureAndPlatform string, parents []string, m map[string]any) (matches []string) {
	for k, v := range m {
		if k == "type" && v == "derivation" && slices.Contains(parents, architectureAndPlatform) {
			matches = append(matches, ".#"+strings.Join(parents, "."))
			continue
		}
		parents := slices.Clone(append(parents, k))
		if m, ok := v.(map[string]any); ok {
			if children := findDerivation(architectureAndPlatform, parents, m); len(children) > 0 {
				matches = append(matches, children...)
			}
		}
	}
	return matches
}
