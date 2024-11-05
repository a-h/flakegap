package nixcmd

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFlakeShowDerivations(t *testing.T) {
	tests := []struct {
		input        string
		architecture string
		platform     string
		expected     []string
	}{
		{
			input: `{
	  "packages": {
	    "aarch64-darwin": {
	      "default": {}
	    },
	    "aarch64-linux": {
	      "default": {
	        "name": "github-runner-manager",
	        "type": "derivation"
	      }
	    },
	    "x86_64-darwin": {
	      "default": {
	        "name": "github-runner-manager",
	        "type": "derivation"
	      }
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
	}`,
			architecture: "x86_64",
			platform:     "linux",
			expected: []string{
				".#packages.x86_64-linux.default",
			},
		},
		{
			input: `{
  "devShells": {
    "aarch64-darwin": {
      "default": {
        "name": "nix-shell",
        "type": "derivation"
      }
    },
    "aarch64-linux": {
      "default": {
        "name": "nix-shell",
        "type": "derivation"
      }
    },
    "x86_64-darwin": {
      "default": {
        "name": "nix-shell",
        "type": "derivation"
      }
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
      "default": {
        "name": "app",
        "type": "derivation"
      },
      "docker-image": {
        "name": "docker-image-app.tar.gz",
        "type": "derivation"
      }
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
}`,
			architecture: "x86_64",
			platform:     "linux",
			expected: []string{
				".#devShells.x86_64-linux.default",
				".#packages.x86_64-linux.default",
				".#packages.x86_64-linux.docker-image",
			},
		},
	}
	for _, test := range tests {
		var fso FlakeShowOutput
		err := json.Unmarshal([]byte(test.input), &fso)
		if err != nil {
			t.Fatalf("failed to unmarshal json: %v", err)
		}
		actual := fso.Derivations(test.architecture, test.platform)
		if diff := cmp.Diff(test.expected, actual); diff != "" {
			t.Error(diff)
		}
	}
}
