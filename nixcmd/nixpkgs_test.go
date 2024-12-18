package nixcmd

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var m = map[string]any{
	"a0": "a0_value",
	"b0": "b0_value",
	"c0": map[string]any{
		"c1": "c1_value",
		"c2": false, // Unsupported type.
		"d2": 1,     // Unsupported type.
		"e2": map[string]any{
			"a3": "a3_value",
			"b3": "b3_value",
		},
		"f2": []any{"f2_value"},       // Unsupported type.
		"g2": map[string]int{"g2": 1}, // Unsupported type.
	},
}

func TestJSONMapStringValue(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		expectedOK bool
		expected   string
	}{
		{
			name:       "nothing to find",
			expectedOK: false,
		},
		{
			name:       "key not found",
			keys:       []string{"a-1"},
			expectedOK: false,
		},
		{
			name:       "key found",
			keys:       []string{"a0"},
			expectedOK: true,
			expected:   "a0_value",
		},
		{
			name:       "nested key found",
			keys:       []string{"c0", "c1"},
			expectedOK: true,
			expected:   "c1_value",
		},
		{
			name:       "nested key not found",
			keys:       []string{"c0", "c-1"},
			expectedOK: false,
		},
		{
			name:       "nested key found, but unsupported type (bool)",
			keys:       []string{"c0", "c2"},
			expectedOK: false,
		},
		{
			name:       "nested key found, but unsupported type (int)",
			keys:       []string{"c0", "d2"},
			expectedOK: false,
		},
		{
			name:       "nested key found, but unsupported type (slice)",
			keys:       []string{"c0", "f2"},
			expectedOK: false,
		},
		{
			name:       "nested key found, but unsupported type (map)",
			keys:       []string{"c0", "g2"},
			expectedOK: false,
		},
		{
			name:       "nested key not found, missing intermediate key",
			keys:       []string{"c0", "e-1", "a3"},
			expectedOK: false,
		},
		{
			name:       "nested nested key found",
			keys:       []string{"c0", "e2", "a3"},
			expectedOK: true,
			expected:   "a3_value",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, ok := JSONMapValue[string](m, test.keys...)
			if ok != test.expectedOK {
				t.Errorf("unexpected ok: want %v, got %v", test.expectedOK, ok)
			}
			if actual != test.expected {
				t.Errorf("unexpected value: want %v, got %v", test.expected, actual)
			}
		})
	}
}

func TestGetNixpkgsReferences(t *testing.T) {
	tests := []struct {
		nixLockFile string
		expected    string
	}{
		{
			nixLockFile: `{
  "nodes": {
    "flake-compat": {
      "flake": false,
      "locked": {
        "lastModified": 1673956053,
        "narHash": "sha256-4gtG9iQuiKITOjNQQeQIpoIB6b16fm+504Ch3sNKLd8=",
        "owner": "edolstra",
        "repo": "flake-compat",
        "rev": "35bb57c0c8d8b62bbfd284272c928ceb64ddbde9",
        "type": "github"
      },
      "original": {
        "owner": "edolstra",
        "repo": "flake-compat",
        "type": "github"
      }
    },
    "flake-parts": {
      "inputs": {
        "nixpkgs-lib": [
          "nix",
          "nixpkgs"
        ]
      },
      "locked": {
        "lastModified": 1712014858,
        "narHash": "sha256-sB4SWl2lX95bExY2gMFG5HIzvva5AVMJd4Igm+GpZNw=",
        "owner": "hercules-ci",
        "repo": "flake-parts",
        "rev": "9126214d0a59633752a136528f5f3b9aa8565b7d",
        "type": "github"
      },
      "original": {
        "owner": "hercules-ci",
        "repo": "flake-parts",
        "type": "github"
      }
    },
    "flake-utils": {
      "inputs": {
        "systems": "systems"
      },
      "locked": {
        "lastModified": 1694529238,
        "narHash": "sha256-zsNZZGTGnMOf9YpHKJqMSsa0dXbfmxeoJ7xHlrt+xmY=",
        "owner": "numtide",
        "repo": "flake-utils",
        "rev": "ff7b65b44d01cf9ba6a71320833626af21126384",
        "type": "github"
      },
      "original": {
        "owner": "numtide",
        "repo": "flake-utils",
        "type": "github"
      }
    },
    "flake-utils_2": {
      "locked": {
        "lastModified": 1667395993,
        "narHash": "sha256-nuEHfE/LcWyuSWnS8t12N1wc105Qtau+/OdUAjtQ0rA=",
        "owner": "numtide",
        "repo": "flake-utils",
        "rev": "5aed5285a952e0b949eb3ba02c12fa4fcfef535f",
        "type": "github"
      },
      "original": {
        "owner": "numtide",
        "repo": "flake-utils",
        "type": "github"
      }
    },
    "git-hooks-nix": {
      "inputs": {
        "flake-compat": [
          "nix"
        ],
        "gitignore": [
          "nix"
        ],
        "nixpkgs": [
          "nix",
          "nixpkgs"
        ],
        "nixpkgs-stable": [
          "nix",
          "nixpkgs"
        ]
      },
      "locked": {
        "lastModified": 1730302582,
        "narHash": "sha256-W1MIJpADXQCgosJZT8qBYLRuZls2KSiKdpnTVdKBuvU=",
        "owner": "cachix",
        "repo": "git-hooks.nix",
        "rev": "af8a16fe5c264f5e9e18bcee2859b40a656876cf",
        "type": "github"
      },
      "original": {
        "owner": "cachix",
        "repo": "git-hooks.nix",
        "type": "github"
      }
    },
    "gitignore": {
      "inputs": {
        "nixpkgs": [
          "nixpkgs"
        ]
      },
      "locked": {
        "lastModified": 1709087332,
        "narHash": "sha256-HG2cCnktfHsKV0s4XW83gU3F57gaTljL9KNSuG6bnQs=",
        "owner": "hercules-ci",
        "repo": "gitignore.nix",
        "rev": "637db329424fd7e46cf4185293b9cc8c88c95394",
        "type": "github"
      },
      "original": {
        "owner": "hercules-ci",
        "repo": "gitignore.nix",
        "type": "github"
      }
    },
    "gomod2nix": {
      "inputs": {
        "flake-utils": "flake-utils",
        "nixpkgs": [
          "nixpkgs"
        ]
      },
      "locked": {
        "lastModified": 1729448365,
        "narHash": "sha256-oquZeWTYWTr5IxfwEzgsxjtD8SSFZYLdO9DaQb70vNU=",
        "owner": "nix-community",
        "repo": "gomod2nix",
        "rev": "5d387097aa716f35dd99d848dc26d8d5b62a104c",
        "type": "github"
      },
      "original": {
        "owner": "nix-community",
        "repo": "gomod2nix",
        "type": "github"
      }
    },
    "libgit2": {
      "flake": false,
      "locked": {
        "lastModified": 1715853528,
        "narHash": "sha256-J2rCxTecyLbbDdsyBWn9w7r3pbKRMkI9E7RvRgAqBdY=",
        "owner": "libgit2",
        "repo": "libgit2",
        "rev": "36f7e21ad757a3dacc58cf7944329da6bc1d6e96",
        "type": "github"
      },
      "original": {
        "owner": "libgit2",
        "ref": "v1.8.1",
        "repo": "libgit2",
        "type": "github"
      }
    },
    "nix": {
      "inputs": {
        "flake-compat": "flake-compat",
        "flake-parts": "flake-parts",
        "git-hooks-nix": "git-hooks-nix",
        "libgit2": "libgit2",
        "nixpkgs": "nixpkgs",
        "nixpkgs-23-11": "nixpkgs-23-11",
        "nixpkgs-regression": "nixpkgs-regression"
      },
      "locked": {
        "lastModified": 1730321079,
        "narHash": "sha256-XdeVy1/d6DEIYb3nOA6JIYF4fwMKNxtwJMgT3pHi+ko=",
        "owner": "nixos",
        "repo": "nix",
        "rev": "597fcc98e18e3178734d06a9e7306250e8cb8d74",
        "type": "github"
      },
      "original": {
        "owner": "nixos",
        "ref": "2.24.10",
        "repo": "nix",
        "type": "github"
      }
    },
    "nixpkgs": {
      "locked": {
        "lastModified": 1730327045,
        "narHash": "sha256-xKel5kd1AbExymxoIfQ7pgcX6hjw9jCgbiBjiUfSVJ8=",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "080166c15633801df010977d9d7474b4a6c549d7",
        "type": "github"
      },
      "original": {
        "owner": "NixOS",
        "ref": "nixos-24.05",
        "repo": "nixpkgs",
        "type": "github"
      }
    },
    "nixpkgs-23-11": {
      "locked": {
        "lastModified": 1717159533,
        "narHash": "sha256-oamiKNfr2MS6yH64rUn99mIZjc45nGJlj9eGth/3Xuw=",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "a62e6edd6d5e1fa0329b8653c801147986f8d446",
        "type": "github"
      },
      "original": {
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "a62e6edd6d5e1fa0329b8653c801147986f8d446",
        "type": "github"
      }
    },
    "nixpkgs-regression": {
      "locked": {
        "lastModified": 1643052045,
        "narHash": "sha256-uGJ0VXIhWKGXxkeNnq4TvV3CIOkUJ3PAoLZ3HMzNVMw=",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "215d4d0fd80ca5163643b03a33fde804a29cc1e2",
        "type": "github"
      },
      "original": {
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "215d4d0fd80ca5163643b03a33fde804a29cc1e2",
        "type": "github"
      }
    },
    "nixpkgs_2": {
      "locked": {
        "lastModified": 1730137625,
        "narHash": "sha256-9z8oOgFZiaguj+bbi3k4QhAD6JabWrnv7fscC/mt0KE=",
        "owner": "NixOS",
        "repo": "nixpkgs",
        "rev": "64b80bfb316b57cdb8919a9110ef63393d74382a",
        "type": "github"
      },
      "original": {
        "owner": "NixOS",
        "ref": "nixos-24.05",
        "repo": "nixpkgs",
        "type": "github"
      }
    },
    "root": {
      "inputs": {
        "gitignore": "gitignore",
        "gomod2nix": "gomod2nix",
        "nix": "nix",
        "nixpkgs": "nixpkgs_2",
        "xc": "xc"
      }
    },
    "systems": {
      "locked": {
        "lastModified": 1681028828,
        "narHash": "sha256-Vy1rq5AaRuLzOxct8nz4T6wlgyUR7zLU309k9mBC768=",
        "owner": "nix-systems",
        "repo": "default",
        "rev": "da67096a3b9bf56a91d16901293e51ba5b49a27e",
        "type": "github"
      },
      "original": {
        "owner": "nix-systems",
        "repo": "default",
        "type": "github"
      }
    },
    "xc": {
      "inputs": {
        "flake-utils": "flake-utils_2",
        "nixpkgs": [
          "nixpkgs"
        ]
      },
      "locked": {
        "lastModified": 1726502039,
        "narHash": "sha256-Zbzr88XKEpLx2D6jaq6KT8S5Cxe76Q9BeYpOcy3fXQk=",
        "owner": "joerdav",
        "repo": "xc",
        "rev": "6183dd54f074aa3a1b3efb716a04966e4b8bf6e5",
        "type": "github"
      },
      "original": {
        "owner": "joerdav",
        "repo": "xc",
        "type": "github"
      }
    }
  },
  "root": "root",
  "version": 7
}`,
			expected: "github:NixOS/nixpkgs/64b80bfb316b57cdb8919a9110ef63393d74382a",
		},
	}
	for _, test := range tests {
		actual, err := GetNixpkgsReference(bytes.NewReader([]byte(test.nixLockFile)))
		if err != nil {
			t.Fatalf("failed to get nixpkgs reference: %v", err)
		}
		if diff := cmp.Diff(test.expected, actual); diff != "" {
			t.Errorf("unexpected nixpkgs reference (-want +got):\n%s", diff)
		}
	}
}
