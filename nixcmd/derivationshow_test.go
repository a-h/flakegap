package nixcmd

import (
	"embed"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The files in testdata/derivationshow contain the testproject, zlib, and m4 source derivations from the output of
// `nix derivation show --recursive` for each Nix version.
//
//go:embed testdata/derivationshow/*.json
var derivationShowOutputs embed.FS

func TestParseDerivations(t *testing.T) {
	testproject := Derivation{
		InputDrvToOutputs: map[string][]string{
			"/nix/store/8jp4mlcmafq35726pp4dvr140j5k2b7g-bash-5.2p37.drv":                   {"out"},
			"/nix/store/ai7q7dcjrq2rbg2c0blbw546l7j5dgfg-npm-build-hook.drv":                {"out"},
			"/nix/store/i22g9na0ll0m0kxydcjb71bg46bv8hdm-nodejs-22.14.0.drv":                {"dev", "out"},
			"/nix/store/jvhi1b6mammjkzc9jmixf0djihvxfk4r-python3-3.12.10.drv":               {"out"},
			"/nix/store/k6lnm9sfkpbz125c8g21pq4z6ghily3v-bash-interactive-5.2p37.drv":       {"out"},
			"/nix/store/n5lz9a8qxypzgh7d7qfdjcc1dqj57aa4-stdenv-darwin.drv":                 {"out"},
			"/nix/store/qpx2n4ynd42wbcllpkn89000jisi6q3l-nodetestproject-1.0.0-sources.drv": {"out"},
			"/nix/store/rwr21y9rx9wib69js183s4xdgn145wmh-cctools-1010.6.drv":                {"dev"},
			"/nix/store/slljfvvxzvjasswdk8kgayizwkibz9ah-npm-install-hook.drv":              {"out"},
			"/nix/store/yil9i5c9kdbv652lp9fv56saxa9f70ia-npm-config-hook.drv":               {"out"},
		},
		OutputToPath: map[string]string{
			"out": "/nix/store/im5v8wkzivxnz97cjv8594lz9w09v2xw-testproject-0.0.1",
		},
	}
	zlib := Derivation{
		InputDrvToOutputs: map[string][]string{
			"/nix/store/029p46kxzk7b0jnsl3sr9zc9wnybmw73-bootstrap-stage2-stdenv-darwin.drv": {"out"},
			"/nix/store/skcffxrm0ak4pbjg82db711cmi2kgg9s-bash-5.2p37.drv":                    {"out"},
			"/nix/store/z1mb7k4vd3dxjvls6611cvrz14kvg02s-zlib-1.3.1.tar.gz.drv":              {"out"},
		},
		OutputToPath: map[string]string{
			"dev":    "/nix/store/kslccv8lpcq68f4jfppsx4d0zxmjcmrf-zlib-1.3.1-dev",
			"out":    "/nix/store/nvss2yn25aqrsri04bxj6izw2gwb65fy-zlib-1.3.1",
			"static": "/nix/store/l1h4zhp9k3jdmqb1999nqjrn5dq9nw6r-zlib-1.3.1-static",
		},
	}
	m4InputDrvToOutputs := map[string][]string{
		"/nix/store/8jp4mlcmafq35726pp4dvr140j5k2b7g-bash-5.2p37.drv":   {"out"},
		"/nix/store/8z7s9p7b8y2fra09kz0cscmn8wh3pdcf-mirrors-list.drv":  {"out"},
		"/nix/store/9ik82bg1375bjvigyx5ghgl99fjng3lf-curl-8.13.0.drv":   {"dev"},
		"/nix/store/w2r0qjcc39kkr0j1p3sjqi760x3ndzs0-stdenv-darwin.drv": {"out"},
	}
	earlierFormat := map[string]Derivation{
		"/nix/store/582lknjwzdbbbds6c10va12alqnx2f5b-testproject-0.0.1.drv": testproject,
		"/nix/store/amwav2plnd3aq40pg3wsc3mbqifyj8c6-zlib-1.3.1.drv":        zlib,
		"/nix/store/04n7ridibb3z1i6sjl5js0h5kyxqxfsb-m4-1.4.19.tar.bz2.drv": {
			InputDrvToOutputs: m4InputDrvToOutputs,
			OutputToPath:      map[string]string{"out": "/nix/store/mcayf992diwks293vs12iwx1ywgm4ijg-m4-1.4.19.tar.bz2"},
			FixedOutput:       true,
		},
	}
	tests := []struct {
		name     string
		file     string
		expected map[string]Derivation
	}{
		{
			name: "the version 4 format of Nix 2.35 can be parsed, and fixed-output paths are unknown",
			file: "nix-2.35.6.json",
			expected: map[string]Derivation{
				"/nix/store/582lknjwzdbbbds6c10va12alqnx2f5b-testproject-0.0.1.drv": testproject,
				"/nix/store/amwav2plnd3aq40pg3wsc3mbqifyj8c6-zlib-1.3.1.drv":        zlib,
				"/nix/store/04n7ridibb3z1i6sjl5js0h5kyxqxfsb-m4-1.4.19.tar.bz2.drv": {
					InputDrvToOutputs: m4InputDrvToOutputs,
					OutputToPath:      map[string]string{"out": ""},
					FixedOutput:       true,
				},
			},
		},
		{
			name:     "the format of Nix 2.31 can be parsed",
			file:     "nix-2.31.6.json",
			expected: earlierFormat,
		},
		{
			name:     "the format of Nix 2.28 can be parsed",
			file:     "nix-2.28.8.json",
			expected: earlierFormat,
		},
		{
			name:     "the format of Nix 2.18 can be parsed",
			file:     "nix-2.18.8.json",
			expected: earlierFormat,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := derivationShowOutputs.Open("testdata/derivationshow/" + tt.file)
			if err != nil {
				t.Fatalf("failed to open test file: %v", err)
			}
			defer f.Close()
			got, err := parseDerivations(f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestParseDerivationsErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unknown format versions are rejected",
			input:    `{"derivations": {}, "version": 5}`,
			expected: "unsupported derivation format version 5",
		},
		{
			name:     "invalid JSON is rejected",
			input:    `{`,
			expected: "failed to decode derivations: unexpected EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDerivations(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if diff := cmp.Diff(tt.expected, err.Error()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
