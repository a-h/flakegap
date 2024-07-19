package nixcmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

func FlakeShow() (op FlakeOutputs, err error) {
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

type FlakeOutputs map[string]ArchitectureToNameToOutput

func (fo FlakeOutputs) Derivations() (derivations []string) {
	for firstLevelName, architecture := range fo {
		for architectureName, output := range architecture {
			for outputName, outputDetails := range output {
				if outputDetails.Type == "derivation" {
					derivations = append(derivations, fmt.Sprintf(".#%s.%s.%s", firstLevelName, architectureName, outputName))
				}
			}
		}
	}
	return derivations
}

type ArchitectureToNameToOutput map[string]NameToOutput

type NameToOutput map[string]Output

type Output struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

/*
{
  "devShells": {
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
        "name": "nix-shell",
        "type": "derivation"
      }
    }
  },
  "packages": {
    "aarch64-darwin": {
      "default": {},
      "docker-image": {}
    },
    "aarch64-linux": {
      "default": {},
      "docker-image": {}
    },
    "x86_64-darwin": {
      "default": {},
      "docker-image": {}
    },
    "x86_64-linux": {
      "default": {
        "name": "app",
        "type": "derivation"
      },
      "docker-image": {
        "name": "docker-image-app.tar.gz",
        "type": "derivation"
      }
    }
  }
}
*/
