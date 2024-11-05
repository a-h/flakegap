package nixcmd

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// nix copy --to file://$PWD/nix-export/nix-store `nix-store --realise $(nix path-info --recursive --derivation .#)`
func NixStoreRealise(stdout, stderr io.Writer, targetStore string, pathsToRealise []string) (realisedPaths []string, err error) {
	nixPath, err := exec.LookPath("nix-store")
	if err != nil {
		return realisedPaths, fmt.Errorf("failed to find nix-store on path: %w", err)
	}

	stdoutBuffer := new(bytes.Buffer)
	w, closer := ErrorBuffer(stdout, stderr)

	args := append([]string{"--realise"}, pathsToRealise...)
	cmd := exec.Command(nixPath, args...)
	cmd.Env = getEnv()
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = w
	if err = closer(cmd.Run()); err != nil {
		return realisedPaths, fmt.Errorf("failed to run nix-store --realise: %w", err)
	}

	return strings.Split(strings.TrimSpace(stdoutBuffer.String()), "\n"), nil
}
