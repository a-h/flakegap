package buildinputs

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/a-h/flakegap/nixcmd"
)

type availabilityChecker interface {
	FindAvailablePaths(ctx context.Context, paths []string) (pathToAvailable map[string]bool, err error)
}

// Selector selects the derivation outputs that an airgapped machine needs to rebuild a flake output.
//
// Nix builds a derivation from the outputs of its direct inputs, so the airgapped machine needs the direct
// inputs of each derivation that it might rebuild. Derivations built from the flake's own source are not in a
// binary cache, so the search follows derivations that no binary cache holds, and stops at those that a binary
// cache holds. This excludes the bootstrap chain and the sources of nixpkgs packages.
type Selector struct {
	checker availabilityChecker
}

// NewSelector creates a Selector that uses checker to find the derivations that a binary cache holds.
func NewSelector(checker availabilityChecker) Selector {
	return Selector{checker: checker}
}

// Selection is the result of selecting build inputs.
type Selection struct {
	// Uncached is the sorted paths of the derivations whose direct inputs are exported.
	Uncached []string
	// Inputs is the sorted direct inputs of the uncached derivations, in the `/nix/store/x.drv!out,dev` form
	// accepted by `nix-store --realise`.
	Inputs []string
}

// SelectBuildInputs selects the build inputs from pathToDerivation, which must contain the derivation of a flake
// output and every derivation it depends on.
func (s Selector) SelectBuildInputs(ctx context.Context, pathToDerivation map[string]nixcmd.Derivation) (selection Selection, err error) {
	roots, err := s.findRoots(pathToDerivation)
	if err != nil {
		return selection, err
	}

	// Roots always need their inputs, so that `nix develop` works for shells that are in a binary cache.
	drvToUncached := make(map[string]bool)
	drvToClassified := make(map[string]bool)
	for _, root := range roots {
		drvToUncached[root] = true
		drvToClassified[root] = true
	}
	frontier := roots
	for len(frontier) > 0 {
		if ctx.Err() != nil {
			return selection, ctx.Err()
		}
		var candidates []string
		for _, drvPath := range frontier {
			for input := range pathToDerivation[drvPath].InputDrvToOutputs {
				if drvToClassified[input] {
					continue
				}
				drvToClassified[input] = true
				d, ok := pathToDerivation[input]
				if !ok {
					return selection, fmt.Errorf("input derivation %q of %q is missing from the derivations", input, drvPath)
				}
				if d.FixedOutput {
					continue
				}
				candidates = append(candidates, input)
			}
		}
		slices.Sort(candidates)

		var outputPaths []string
		for _, candidate := range candidates {
			for _, p := range pathToDerivation[candidate].OutputToPath {
				if p != "" {
					outputPaths = append(outputPaths, p)
				}
			}
		}
		pathToAvailable, err := s.checker.FindAvailablePaths(ctx, outputPaths)
		if err != nil {
			return selection, fmt.Errorf("failed to check binary caches: %w", err)
		}

		frontier = nil
		for _, candidate := range candidates {
			if s.checkOutputsAvailable(pathToDerivation[candidate], pathToAvailable) {
				continue
			}
			drvToUncached[candidate] = true
			frontier = append(frontier, candidate)
		}
	}

	inputToOutputs := make(map[string]map[string]bool)
	for drvPath := range drvToUncached {
		for input, outputs := range pathToDerivation[drvPath].InputDrvToOutputs {
			if inputToOutputs[input] == nil {
				inputToOutputs[input] = make(map[string]bool)
			}
			for _, output := range outputs {
				inputToOutputs[input][output] = true
			}
		}
	}
	selection.Uncached = slices.Sorted(maps.Keys(drvToUncached))
	for _, input := range slices.Sorted(maps.Keys(inputToOutputs)) {
		outputs := slices.Sorted(maps.Keys(inputToOutputs[input]))
		selection.Inputs = append(selection.Inputs, input+"!"+strings.Join(outputs, ","))
	}
	return selection, nil
}

func (s Selector) findRoots(pathToDerivation map[string]nixcmd.Derivation) (roots []string, err error) {
	drvToIsInput := make(map[string]bool)
	for _, d := range pathToDerivation {
		for input := range d.InputDrvToOutputs {
			drvToIsInput[input] = true
		}
	}
	for drvPath := range pathToDerivation {
		if !drvToIsInput[drvPath] {
			roots = append(roots, drvPath)
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("no root derivation found in %d derivations", len(pathToDerivation))
	}
	slices.Sort(roots)
	return roots, nil
}

func (s Selector) checkOutputsAvailable(d nixcmd.Derivation, pathToAvailable map[string]bool) bool {
	if len(d.OutputToPath) == 0 {
		return false
	}
	for _, p := range d.OutputToPath {
		if p == "" || !pathToAvailable[p] {
			return false
		}
	}
	return true
}
