package nixcmd

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var mapStdout = `{
  "/nix/store/002vxafpvw71pmp69hw14ahgvqjyx320-source.drv": {
    "registrationTime": 1728231328,
    "signatures": [],
    "ultimate": false
  },
  "/nix/store/00qr10y7z2fcvrp9b2m46710nkjvj55z-update-autotools-gnu-config-scripts.sh": {
  }
}`

var sliceStdout = `[
  { "path": "/nix/store/002vxafpvw71pmp69hw14ahgvqjyx320-source.drv" },
	{ "path": "/nix/store/00qr10y7z2fcvrp9b2m46710nkjvj55z-update-autotools-gnu-config-scripts.sh" }
]`

func TestPathInfo(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		expected    []string
		expectedErr error
	}{
		{
			name:   "map",
			stdout: mapStdout,
			expected: []string{
				"/nix/store/002vxafpvw71pmp69hw14ahgvqjyx320-source.drv",
				"/nix/store/00qr10y7z2fcvrp9b2m46710nkjvj55z-update-autotools-gnu-config-scripts.sh",
			},
		},
		{
			name:   "slice",
			stdout: sliceStdout,
			expected: []string{
				"/nix/store/002vxafpvw71pmp69hw14ahgvqjyx320-source.drv",
				"/nix/store/00qr10y7z2fcvrp9b2m46710nkjvj55z-update-autotools-gnu-config-scripts.sh",
			},
		},
		{
			name:        "empty",
			stdout:      "",
			expected:    nil,
			expectedErr: fmt.Errorf("empty nix path-info output"),
		},
		{
			name:        "invalid",
			stdout:      "invalid",
			expected:    nil,
			expectedErr: fmt.Errorf("unexpected output: invalid"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := getPathInfo([]byte(tt.stdout))
			if err != nil {
				if tt.expectedErr == nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if err.Error() != tt.expectedErr.Error() {
					t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
				}
			}
			if diff := cmp.Diff(tt.expected, actual); diff != "" {
				t.Error(diff)
			}
		})
	}
}
