package nixcmd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseSubstituters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "substituters are read from nix show-config output",
			input: `{
  "sandbox": {"aliases": [], "defaultValue": true, "description": "", "value": true},
  "substituters": {"aliases": ["binary-caches"], "defaultValue": ["https://cache.nixos.org/"], "description": "", "value": ["https://cache.nixos.org/", "https://cache.flakehub.com/"]}
}`,
			expected: []string{"https://cache.nixos.org/", "https://cache.flakehub.com/"},
		},
		{
			name:     "an empty substituter list can be read",
			input:    `{"substituters": {"value": []}}`,
			expected: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSubstituters(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}
