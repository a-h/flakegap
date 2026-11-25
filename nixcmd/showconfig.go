package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// GetSubstituters returns the URLs of the binary caches that Nix is configured to substitute from.
func GetSubstituters(stdout, stderr io.Writer) (urls []string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return nil, fmt.Errorf("failed to find nix on path: %w", err)
	}

	// Nix 2.18 does not have `nix config show`, and Nix 2.35 still supports `nix show-config`.
	var out bytes.Buffer
	cmd := exec.Command(nixPath, "show-config", "--json")
	cmd.Env = getEnv()
	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stdout = &out
	cmd.Stderr = w
	if err = closer(cmd.Run()); err != nil {
		return nil, fmt.Errorf("failed to run nix show-config: %w", err)
	}
	return parseSubstituters(&out)
}

type showConfigJSON struct {
	Substituters struct {
		Value []string `json:"value"`
	} `json:"substituters"`
}

func parseSubstituters(r io.Reader) (urls []string, err error) {
	var config showConfigJSON
	if err = json.NewDecoder(r).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode nix show-config output: %w", err)
	}
	return config.Substituters.Value, nil
}
