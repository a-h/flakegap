package nixcmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

func FlakeShow() (op FlakeShowOutput, err error) {
	nixPath, err := exec.LookPath("nix")
	if err != nil {
		return op, fmt.Errorf("failed to find nix on path: %v", err)
	}

	cmd := exec.Command(nixPath, "flake", "show", "--json")
	cmd.Dir = "/code"
	output, err := cmd.Output()
	if err != nil {
		return op, fmt.Errorf("failed to run nix: %v", err)
	}

	err = json.Unmarshal(output, &op)
	if err != nil {
		return op, fmt.Errorf("failed to parse nix output %q: %v", string(output), err)
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

/*
	{
	  "packages": {
	    "aarch64-darwin": {
	      "default": {}
	    },
	    "aarch64-linux": {
	      "default": {}
	    },
	    "x86_64-darwin": {
	      "default": {}
	    },
	    "x86_64-linux": {
	      "default": {
	        "name": "github-runner-manager",
	        "type": "derivation"
	      }
	    }
	  },
	  "vms": {
	    "type": "unknown"
	  }
	}
*/
