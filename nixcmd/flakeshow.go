package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func FlakeShow(stdout, stderr io.Writer) (op FlakeShowOutput, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return op, fmt.Errorf("failed to find nix on path: %w", err)
	}

	stdoutBuffer := new(bytes.Buffer)

	cmd := exec.Command(nixPath, "flake", "show", "--json")
	cmd.Stdout = io.MultiWriter(stdoutBuffer, stdout)
	cmd.Stderr = stderr
	cmd.Dir = "/code"
	if err = cmd.Run(); err != nil {
		return op, fmt.Errorf("failed to run nix flake show: %w", err)
	}

	err = json.Unmarshal(stdoutBuffer.Bytes(), &op)
	if err != nil {
		return op, fmt.Errorf("failed to parse nix flake show output: %w", err)
	}

	return op, err
}

type FlakeShowOutput map[string]any

func (fso FlakeShowOutput) Derivations() (matches []string) {
	return findDerivation([]string{}, fso)
}

func findDerivation(parents []string, m map[string]any) (matches []string) {
	for k, v := range m {
		if k == "type" && v == "derivation" {
			matches = append(matches, ".#"+strings.Join(parents, "."))
			continue
		}
		parents := append([]string{}, append(parents, k)...)
		if m, ok := v.(map[string]any); ok {
			if children := findDerivation(parents, m); len(children) > 0 {
				matches = append(matches, children...)
			}
		}
	}
	return matches
}
