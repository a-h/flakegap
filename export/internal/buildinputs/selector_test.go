package buildinputs

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/a-h/flakegap/nixcmd"
	"github.com/google/go-cmp/cmp"
)

type fakeChecker struct {
	pathToAvailable map[string]bool
	err             error
	queriedPaths    []string
}

func (f *fakeChecker) FindAvailablePaths(ctx context.Context, paths []string) (pathToAvailable map[string]bool, err error) {
	f.queriedPaths = append(f.queriedPaths, paths...)
	if f.err != nil {
		return nil, f.err
	}
	pathToAvailable = make(map[string]bool, len(paths))
	for _, p := range paths {
		pathToAvailable[p] = f.pathToAvailable[p]
	}
	return pathToAvailable, nil
}

func createDerivation(outputToPath map[string]string, inputDrvToOutputs map[string][]string) nixcmd.Derivation {
	return nixcmd.Derivation{InputDrvToOutputs: inputDrvToOutputs, OutputToPath: outputToPath}
}

func TestSelectBuildInputs(t *testing.T) {
	bootstrap := createDerivation(map[string]string{"out": "/nix/store/bootstrap"}, nil)
	gcc := createDerivation(map[string]string{"out": "/nix/store/gcc", "lib": "/nix/store/gcc-lib"}, map[string][]string{"/nix/store/bootstrap.drv": {"out"}})
	bash := createDerivation(map[string]string{"out": "/nix/store/bash"}, map[string][]string{"/nix/store/bootstrap.drv": {"out"}})
	curl := createDerivation(map[string]string{"out": "/nix/store/curl"}, nil)
	src := nixcmd.Derivation{
		InputDrvToOutputs: map[string][]string{"/nix/store/curl.drv": {"out"}},
		OutputToPath:      map[string]string{"out": ""},
		FixedOutput:       true,
	}
	cached := map[string]bool{
		"/nix/store/bootstrap": true,
		"/nix/store/gcc":       true,
		"/nix/store/gcc-lib":   true,
		"/nix/store/bash":      true,
		"/nix/store/curl":      true,
	}
	tests := []struct {
		name                 string
		pathToDerivation     map[string]nixcmd.Derivation
		expected             Selection
		expectedQueriedPaths []string
	}{
		{
			name: "the direct inputs of the root are selected when every input is cached",
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv":       createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{"/nix/store/bash.drv": {"out"}}),
				"/nix/store/bash.drv":      bash,
				"/nix/store/bootstrap.drv": bootstrap,
			},
			expected: Selection{
				Uncached: []string{"/nix/store/app.drv"},
				Inputs:   []string{"/nix/store/bash.drv!out"},
			},
			expectedQueriedPaths: []string{"/nix/store/bash"},
		},
		{
			name: "the inputs of uncached derivations are selected, and the inputs of cached derivations are not",
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv": createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{
					"/nix/store/lib.drv": {"out"},
					"/nix/store/gcc.drv": {"out"},
				}),
				"/nix/store/lib.drv": createDerivation(map[string]string{"out": "/nix/store/lib"}, map[string][]string{
					"/nix/store/gcc.drv":  {"lib"},
					"/nix/store/bash.drv": {"out"},
				}),
				"/nix/store/gcc.drv":       gcc,
				"/nix/store/bash.drv":      bash,
				"/nix/store/bootstrap.drv": bootstrap,
			},
			expected: Selection{
				Uncached: []string{"/nix/store/app.drv", "/nix/store/lib.drv"},
				Inputs:   []string{"/nix/store/bash.drv!out", "/nix/store/gcc.drv!lib,out", "/nix/store/lib.drv!out"},
			},
			expectedQueriedPaths: []string{"/nix/store/bash", "/nix/store/gcc", "/nix/store/gcc-lib", "/nix/store/lib"},
		},
		{
			name: "fixed-output inputs are selected, and their inputs are not",
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv":  createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{"/nix/store/src.drv": {"out"}}),
				"/nix/store/src.drv":  src,
				"/nix/store/curl.drv": curl,
			},
			expected: Selection{
				Uncached: []string{"/nix/store/app.drv"},
				Inputs:   []string{"/nix/store/src.drv!out"},
			},
		},
		{
			name: "derivations with unknown output paths are treated as uncached",
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv":       createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{"/nix/store/ca.drv": {"out"}}),
				"/nix/store/ca.drv":        createDerivation(map[string]string{"out": ""}, map[string][]string{"/nix/store/bash.drv": {"out"}}),
				"/nix/store/bash.drv":      bash,
				"/nix/store/bootstrap.drv": bootstrap,
			},
			expected: Selection{
				Uncached: []string{"/nix/store/app.drv", "/nix/store/ca.drv"},
				Inputs:   []string{"/nix/store/bash.drv!out", "/nix/store/ca.drv!out"},
			},
			expectedQueriedPaths: []string{"/nix/store/bash"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeChecker{pathToAvailable: cached}
			got, err := NewSelector(checker).SelectBuildInputs(context.Background(), tt.pathToDerivation)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Error(diff)
			}
			slices.Sort(checker.queriedPaths)
			if diff := cmp.Diff(tt.expectedQueriedPaths, checker.queriedPaths); diff != "" {
				t.Errorf("queried paths: %s", diff)
			}
		})
	}
}

func TestSelectBuildInputsErrors(t *testing.T) {
	tests := []struct {
		name             string
		checkerErr       error
		pathToDerivation map[string]nixcmd.Derivation
		expected         string
	}{
		{
			name:       "binary cache errors are returned",
			checkerErr: errors.New("network unreachable"),
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv":  createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{"/nix/store/bash.drv": {"out"}}),
				"/nix/store/bash.drv": createDerivation(map[string]string{"out": "/nix/store/bash"}, nil),
			},
			expected: "failed to check binary caches: network unreachable",
		},
		{
			name: "input derivations missing from the derivations are reported",
			pathToDerivation: map[string]nixcmd.Derivation{
				"/nix/store/app.drv": createDerivation(map[string]string{"out": "/nix/store/app"}, map[string][]string{"/nix/store/bash.drv": {"out"}}),
			},
			expected: `input derivation "/nix/store/bash.drv" of "/nix/store/app.drv" is missing from the derivations`,
		},
		{
			name:             "an empty set of derivations is reported",
			pathToDerivation: map[string]nixcmd.Derivation{},
			expected:         "no root derivation found in 0 derivations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSelector(&fakeChecker{err: tt.checkerErr}).SelectBuildInputs(context.Background(), tt.pathToDerivation)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if diff := cmp.Diff(tt.expected, err.Error()); diff != "" {
				t.Error(diff)
			}
		})
	}
}
