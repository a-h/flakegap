package nixcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const storeDir = "/nix/store/"

// Derivation is the part of a Nix derivation needed to find the build inputs to export.
type Derivation struct {
	// InputDrvToOutputs maps the path of each input derivation to the names of the outputs that the derivation uses.
	InputDrvToOutputs map[string][]string
	// OutputToPath maps each output name to its store path. The path is empty when the JSON does not include it,
	// for example for fixed-output and content-addressed derivations in the version 4 format.
	OutputToPath map[string]string
	// FixedOutput is true when the derivation declares the hash of its output, e.g. a source download.
	FixedOutput bool
}

// ShowDerivations returns the derivation of the flake reference, and every derivation it depends on, keyed by
// store path.
func ShowDerivations(stdout, stderr io.Writer, codeDir, ref string) (pathToDerivation map[string]Derivation, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return nil, fmt.Errorf("failed to find nix on path: %w", err)
	}

	// nix derivation show --recursive <ref>
	cmd := exec.Command(nixPath, "derivation", "show", "--recursive", ref)
	cmd.Env = getEnv()
	cmd.Dir = codeDir
	w, closer := ErrorBuffer(stdout, stderr)
	cmd.Stderr = w
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return nil, closer(fmt.Errorf("failed to start nix derivation show: %w", err))
	}

	pathToDerivation, parseErr := parseDerivations(r)
	if parseErr != nil {
		// Drain the output so that the command can exit.
		if _, err = io.Copy(io.Discard, r); err != nil {
			parseErr = errors.Join(parseErr, err)
		}
	}
	if err = closer(cmd.Wait()); err != nil {
		return nil, fmt.Errorf("failed to run nix derivation show --recursive %s: %w", ref, err)
	}
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse nix derivation show --recursive %s: %w", ref, parseErr)
	}
	if len(pathToDerivation) == 0 {
		return nil, fmt.Errorf("nix derivation show --recursive %s returned no derivations", ref)
	}
	return pathToDerivation, nil
}

type derivationJSON struct {
	InputDrvs map[string]inputJSON `json:"inputDrvs"`
	Inputs    struct {
		Drvs map[string]inputJSON `json:"drvs"`
	} `json:"inputs"`
	Outputs map[string]struct {
		Path string `json:"path"`
		Hash string `json:"hash"`
	} `json:"outputs"`
}

type inputJSON struct {
	Outputs []string `json:"outputs"`
}

func parseDerivations(r io.Reader) (pathToDerivation map[string]Derivation, err error) {
	var top map[string]json.RawMessage
	if err = json.NewDecoder(r).Decode(&top); err != nil {
		return nil, fmt.Errorf("failed to decode derivations: %w", err)
	}

	// Nix 2.18 to 2.31 write the derivations at the top level, keyed by absolute store path, with no version field.
	// Nix 2.35 writes {"derivations": {...}, "version": 4}, keyed by store path basename, with inputs.drvs in place
	// of inputDrvs.
	pathToJSON := top
	if rawVersion, ok := top["version"]; ok {
		var version int
		if err = json.Unmarshal(rawVersion, &version); err != nil {
			return nil, fmt.Errorf("failed to decode derivation format version: %w", err)
		}
		if version != 4 {
			return nil, fmt.Errorf("unsupported derivation format version %d", version)
		}
		pathToJSON = nil
		if err = json.Unmarshal(top["derivations"], &pathToJSON); err != nil {
			return nil, fmt.Errorf("failed to decode derivations: %w", err)
		}
	}

	pathToDerivation = make(map[string]Derivation, len(pathToJSON))
	for path, raw := range pathToJSON {
		var dj derivationJSON
		if err = json.Unmarshal(raw, &dj); err != nil {
			return nil, fmt.Errorf("failed to decode derivation %q: %w", path, err)
		}
		inputs := dj.InputDrvs
		if inputs == nil {
			inputs = dj.Inputs.Drvs
		}
		d := Derivation{
			InputDrvToOutputs: make(map[string][]string, len(inputs)),
			OutputToPath:      make(map[string]string, len(dj.Outputs)),
		}
		for inputPath, input := range inputs {
			d.InputDrvToOutputs[convertToStorePath(inputPath)] = input.Outputs
		}
		for name, output := range dj.Outputs {
			var outputPath string
			if output.Path != "" {
				outputPath = convertToStorePath(output.Path)
			}
			d.OutputToPath[name] = outputPath
			if output.Hash != "" {
				d.FixedOutput = true
			}
		}
		pathToDerivation[convertToStorePath(path)] = d
	}
	return pathToDerivation, nil
}

func convertToStorePath(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return storeDir + path
}
