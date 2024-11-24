package nixcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"slices"
)

func DerivationShow(stdout, stderr io.Writer, codeDir, ref string) (inputDrvs []string, srcs []string, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find nix on path: %v", err)
	}

	stdoutBuffer := new(bytes.Buffer)

	cmd := exec.Command(nixPath, "derivation", "show", ref)
	cmd.Dir = codeDir

	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stdout = io.MultiWriter(stdoutBuffer, w)
	cmd.Stderr = w
	if err = closer(cmd.Run()); err != nil {
		return nil, nil, fmt.Errorf("failed to run nix derivation show: %v", err)
	}

	return getInputDrvs(stdoutBuffer.Bytes())
}

type Derivation struct {
	InputDrvs map[string]any `json:"inputDrvs"`
	InputSrcs []string       `json:"inputSrcs"`
}

func getInputDrvs(input []byte) (drvs []string, srcs []string, err error) {
	var m map[string]Derivation
	err = json.Unmarshal(input, &m)
	if err != nil {
		return drvs, srcs, fmt.Errorf("failed to unmarshal derivation: %v", err)
	}
	var drvKeys []string
	for k := range m {
		drvKeys = append(drvKeys, k)
	}
	if len(drvKeys) != 1 {
		return drvs, srcs, fmt.Errorf("expected exactly one key in the map, got %d", len(drvKeys))
	}
	drv := m[drvKeys[0]]
	for k := range drv.InputDrvs {
		drvs = append(drvs, k)
	}
	slices.Sort(drvs)
	return drvs, drv.InputSrcs, nil
}
