package export

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/a-h/flakegap/export/internal/buildinputs"
	"github.com/a-h/flakegap/nixcmd"
)

const (
	// BuildInputsUncached walks the dependencies of each flake output and exports each dependency. It stops at
	// dependencies that a binary cache holds, and follows dependencies that no binary cache holds, such as those
	// built from the flake's source, so that the airgapped machine can rebuild them after the source changes.
	BuildInputsUncached = "uncached"
	// BuildInputsAll exports the outputs of every derivation in the build closure, including the bootstrap chain.
	BuildInputsAll = "all"
)

// closureCopier copies flake outputs, and the build inputs needed to rebuild them, to a binary cache store.
type closureCopier struct {
	log         *slog.Logger
	codeDir     string
	targetStore string
	buildInputs string
	selector    buildinputs.Selector
}

func newClosureCopier(log *slog.Logger, codeDir, targetStore, buildInputs string, selector buildinputs.Selector) closureCopier {
	return closureCopier{
		log:         log,
		codeDir:     codeDir,
		targetStore: targetStore,
		buildInputs: buildInputs,
		selector:    selector,
	}
}

// Copy copies the derivations and outputs of ref, and the outputs of its build inputs, to the target store.
func (c closureCopier) Copy(ctx context.Context, ref string) (realisedPathCount int, err error) {
	// nix copy --to file://$PWD/nix-export/nix-store --derivation <ref>
	if err = nixcmd.CopyTo(os.Stdout, os.Stderr, c.codeDir, c.targetStore, true, ref); err != nil {
		return 0, fmt.Errorf("failed to copy derivation: %w", err)
	}
	// nix copy --to file://$PWD/nix-export/nix-store <ref>
	if err = nixcmd.CopyTo(os.Stdout, os.Stderr, c.codeDir, c.targetStore, false, ref); err != nil {
		return 0, fmt.Errorf("failed to copy path: %w", err)
	}

	drvs, err := c.findBuildInputs(ctx, ref)
	if err != nil {
		return 0, fmt.Errorf("failed to find build inputs: %w", err)
	}
	if len(drvs) == 0 {
		return 0, nil
	}
	// nix-store --realise /nix/store/<hash>-<name>.drv!out,dev ...
	realisedPaths, err := nixcmd.NixStoreRealise(os.Stdout, os.Stderr, c.targetStore, drvs)
	if err != nil {
		return 0, fmt.Errorf("failed to realise derivations: %w", err)
	}
	if len(realisedPaths) == 0 {
		return 0, nil
	}
	// nix copy --to file://$PWD/nix-export/nix-store $paths_from_previous_command
	if err = nixcmd.CopyTo(os.Stdout, os.Stderr, c.codeDir, c.targetStore, false, realisedPaths...); err != nil {
		return 0, fmt.Errorf("failed to copy realised paths: %w", err)
	}
	return len(realisedPaths), nil
}

func (c closureCopier) findBuildInputs(ctx context.Context, ref string) (drvs []string, err error) {
	if c.buildInputs == BuildInputsAll {
		// nix path-info --recursive --derivation <ref>
		return nixcmd.PathInfo(os.Stdout, os.Stderr, c.codeDir, true, true, ref)
	}
	// nix derivation show --recursive <ref>
	pathToDerivation, err := nixcmd.ShowDerivations(os.Stdout, os.Stderr, c.codeDir, ref)
	if err != nil {
		return nil, err
	}
	selection, err := c.selector.SelectBuildInputs(ctx, pathToDerivation)
	if err != nil {
		return nil, fmt.Errorf("failed to select build inputs of %q: %w", ref, err)
	}
	c.log.Info("Selected build inputs",
		slog.String("ref", ref),
		slog.Int("derivations", len(pathToDerivation)),
		slog.Int("uncachedDerivations", len(selection.Uncached)),
		slog.Int("buildInputs", len(selection.Inputs)))
	// A derivation that depends on others always has inputs, so no inputs means that the JSON was not understood.
	if len(selection.Inputs) == 0 && len(pathToDerivation) > 1 {
		return nil, fmt.Errorf("no build inputs found for %q in %d derivations", ref, len(pathToDerivation))
	}
	return selection.Inputs, nil
}
