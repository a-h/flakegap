package export

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"slices"

	"github.com/a-h/flakegap/archive"
	"github.com/a-h/flakegap/nixcmd"
	"github.com/dustin/go-humanize"
	"github.com/nix-community/go-nix/pkg/narinfo"
	cp "github.com/otiai10/copy"
)

type Args struct {
	// Code is the path to the repo on disk that contains a flake.nix file.
	Code string
	// ExportFileName is the path to write the output to, e.g. /tmp/nix-export.tar.gz.
	ExportFileName string
	// Architecture to build for.
	Architecture string
	// Platform to build for.
	Platform string
	// TemporaryPath is the path to write temporary files to. Defaults to a temporary directory in your home folder, because
	// some Linux systems have a very small /tmp partition or hold /tmp in memory (e.g. tmpfs) which makes it unsuitable for
	// large builds.
	TemporaryPath string
	// Help shows usage and quits.
	Help bool
}

func (a Args) Validate() error {
	var errs []error
	if a.Code == "" {
		errs = append(errs, fmt.Errorf("source-path is required"))
	}
	if a.ExportFileName == "" {
		errs = append(errs, fmt.Errorf("export-filename is required"))
	}
	if a.Architecture == "" {
		errs = append(errs, fmt.Errorf("architecture is required"))
	}
	if a.Platform == "" {
		errs = append(errs, fmt.Errorf("platform is required"))
	}
	return errors.Join(errs...)
}

func getTemporaryPath(log *slog.Logger, current string) (updated string) {
	if current != "" {
		return current
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	log.Warn("Home directory not found, using system temp directory which may be too small for large builds.")
	return ""
}

func Run(ctx context.Context, log *slog.Logger, args Args) (err error) {
	var wg sync.WaitGroup

	nixExportPath, err := os.MkdirTemp(getTemporaryPath(log, args.TemporaryPath), "flakegap")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(nixExportPath)

	// export NIXPKGS_COMMIT=`jq -r '.nodes.[.nodes.[.root].inputs.nixpkgs].locked | "\(.type):\(.owner)/\(.repo)/\(.rev)"' flake.lock`
	// nix copy --to file://$PWD/export "$NIXPKGS_COMMIT#legacyPackages.x86_64-linux.bashInteractive"
	// # Copy the packages.
	// nix copy --to file://$PWD/export .#packages.x86_64-linux.default
	// nix copy --derivation --to file://$PWD/export .#packages.x86_64-linux.default
	// # Copy the devshell contents.
	// nix copy --to file://$PWD/export .#devShells.x86_64-linux.default
	// nix copy --derivation --to file://$PWD/export .#devShells.x86_64-linux.default
	// Copy realised paths of inputs to the derivation so that we can build it.
	// export paths=$(nix derivation show .# | jq -r '.[].inputDrvs | keys[]')
	// export realised_paths=$(nix-store --realise $paths)
	// nix copy --to file://$PWD/export $realised_paths
	// # Copy the flake inputs to the store.
	// nix flake archive --to file://$PWD/export

	targetStore := (&url.URL{
		Scheme: "file",
		Path:   filepath.Join(nixExportPath, "nix-store"),
	}).String()

	op, err := nixcmd.FlakeShow(os.Stdout, os.Stderr, args.Code)
	if err != nil {
		return fmt.Errorf("failed to gather nix outputs: %w", err)
	}
	drvs := op.Derivations(args.Architecture, args.Platform)

	log.Info("Building", slog.Any("outputs", drvs), slog.String("architecture", args.Architecture), slog.String("platform", args.Platform))

	f, err := os.Open(filepath.Join(args.Code, "flake.lock"))
	if err != nil {
		return fmt.Errorf("failed to open flake.lock: %w", err)
	}
	defer f.Close()
	// export NIXPKGS_COMMIT=`jq -r '.nodes.[.nodes.[.root].inputs.nixpkgs].locked | "\(.type):\(.owner)/\(.repo)/\(.rev)"' flake.lock`
	// nix copy --to file://$PWD/export "$NIXPKGS_COMMIT#legacyPackages.x86_64-linux.bashInteractive"
	nixpkgsRef, err := nixcmd.GetNixpkgsReference(f)
	if err != nil {
		return fmt.Errorf("failed to get nixpkgs reference: %w", err)
	}
	suffixes := []string{
		fmt.Sprintf("#legacyPackages.%s-%s.bashInteractive", args.Architecture, args.Platform), // Required for nix develop.
	}
	for _, suffix := range suffixes {
		nixpkgsRefWithSuffix := nixpkgsRef + suffix
		log.Info("Copying nixpkgs to target", slog.String("target", targetStore), slog.String("ref", nixpkgsRefWithSuffix))
		realisedPathCount, err := nixcmd.CopyToAll(os.Stdout, os.Stderr, args.Code, targetStore, nixpkgsRefWithSuffix)
		if err != nil {
			return fmt.Errorf("failed to copy nixpkgs to %q: %w", targetStore, err)
		}
		log.Info("Copied nixpkgs to target", slog.String("target", targetStore), slog.String("ref", nixpkgsRefWithSuffix), slog.Int("realisedPaths", realisedPathCount))
	}

	for i, ref := range drvs {
		log.Info("Building", slog.String("ref", ref))
		// nix build <ref>
		if err := nixcmd.Build(os.Stdout, os.Stderr, args.Code, ref); err != nil {
			log.Error("failed to build", slog.Any("error", err))
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
		// nix copy --to file://$PWD/export .#packages.x86_64-linux.default
		// nix copy --derivation --to file://$PWD/export .#packages.x86_64-linux.default
		// nix copy --to file://$PWD/nix-export/nix-store `nix-store --realise $(nix path-info --recursive --derivation .#)`
		log.Info("Copying Nix closures to target", slog.String("ref", ref), slog.String("target", targetStore))
		realisedPathCount, err := nixcmd.CopyToAll(os.Stdout, os.Stderr, args.Code, targetStore, ref)
		if err != nil {
			return fmt.Errorf("failed to copy %q to %q: %w", ref, targetStore, err)
		}
		log.Info("Copied Nix closures to target", slog.String("ref", ref), slog.Int("realisedPaths", realisedPathCount))
		targetDirParts := strings.Split(strings.TrimPrefix(ref, ".#"), ".")
		target := filepath.Join(append([]string{nixExportPath, "outputs"}, targetDirParts...)...)
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create outputs directory %q: %w", target, err)
		}
		evaluatedPath, err := filepath.EvalSymlinks("./result")
		if err != nil {
			return fmt.Errorf("failed to evaluate symlinks for %q: %w", "./result", err)
		}
		fi, err := os.Stat(evaluatedPath)
		if err != nil {
			return fmt.Errorf("failed to stat evaluated result path %q: %w", evaluatedPath, err)
		}
		if !fi.IsDir() {
			target = filepath.Join(target, "result")
		}
		log.Info("Copying build outputs to target", slog.String("ref", ref), slog.String("target", target))
		opt := cp.Options{
			OnSymlink: func(src string) cp.SymlinkAction {
				return cp.Deep
			},
			Sync: true,
		}
		if err := cp.Copy("./result", target, opt); err != nil {
			return fmt.Errorf("failed to copy output %q to %q: %w", "./result", target, err)
		}
		log.Info("Completed operation", slog.String("ref", ref), slog.Int("item", i+1), slog.Int("total", len(drvs)))
	}

	log.Info("Copying flake archive to output")
	// nix flake archive --to file:///nix-export/nix-store/
	if err := nixcmd.FlakeArchive(os.Stdout, os.Stderr, args.Code, targetStore); err != nil {
		log.Error("failed to archive flake", slog.Any("error", err))
		return fmt.Errorf("failed to archive flake: %w", err)
	}
	// End of the manually exported code.

	log.Info("Copying source code")
	srcOutputDir := filepath.Join(nixExportPath, "source")
	if err := os.MkdirAll(srcOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create source output directory: %w", err)
	}
	ignore := []string{".direnv", "nix-export", "nix-export.tar.gz", "result", "coverage.out", ".DS_Store"}
	symlinks := make(map[string]struct{})
	opt := cp.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			return slices.Contains(ignore, srcinfo.Name()), nil
		},
		OnSymlink: func(src string) cp.SymlinkAction {
			symlinks[src] = struct{}{}
			return cp.Deep
		},
		OnError: func(src, dest string, err error) error {
			if _, ok := symlinks[src]; ok {
				// Ignore errors for symlinks.
				log.Warn("Ignoring symlink error", slog.String("src", src), slog.String("dest", dest), slog.Any("error", err))
				return nil
			}
			return err
		},
		Sync: true,
	}
	if err := cp.Copy(args.Code, srcOutputDir, opt); err != nil {
		return fmt.Errorf("failed to copy source code: %w", err)
	}

	log.Info("Collecting store paths")
	if err = writeManifest(ctx, nixExportPath); err != nil {
		return fmt.Errorf("failed to get store paths: %w", err)
	}

	log.Info("Archiving output")
	size, err := archive.Archive(ctx, nixExportPath, args.ExportFileName)
	if err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	wg.Wait()

	log.Info("Complete", slog.String("uncompressedSize", humanize.Bytes(uint64(size))))
	return nil
}

func writeManifest(ctx context.Context, nixExportPath string) (err error) {
	exportManifestFileName := filepath.Join(nixExportPath, "nix-export.txt")
	w, err := os.Create(exportManifestFileName)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer w.Close()

	return filepath.Walk(nixExportPath, func(path string, info os.FileInfo, err error) error {
		if cancel := ctx.Err(); cancel != nil {
			return cancel
		}
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".narinfo" || info.IsDir() {
			return nil
		}

		r, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %q: %w", path, err)
		}
		defer r.Close()
		ni, err := narinfo.Parse(r)
		if err != nil {
			return fmt.Errorf("failed to parse narinfo %q: %w", path, err)
		}
		if _, err = fmt.Fprintf(w, "%s\n", ni.StorePath); err != nil {
			return fmt.Errorf("failed to write store path %q: %w", ni.StorePath, err)
		}
		return nil
	})
}
