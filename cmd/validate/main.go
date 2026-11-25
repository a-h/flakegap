package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/a-h/flakegap/nixcmd"
)

var version string

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	log = log.With(slog.String("version", version))
	log = log.With(slog.String("flakegap", "server"))

	var architecture, platform string
	var codeDir string
	var sourceStore string
	var developCommand string
	cmdFlags := flag.NewFlagSet("runtime", flag.ContinueOnError)
	cmdFlags.StringVar(&architecture, "architecture", "x86_64", "Architecture to build for, e.g. x86_64, aarch64")
	cmdFlags.StringVar(&platform, "platform", "linux", "Platform to build for, e.g. linux, darwin")
	cmdFlags.StringVar(&codeDir, "code-dir", "/code", "Code directory")
	cmdFlags.StringVar(&sourceStore, "source-store", "file:///nix-export/nix-store/", "Source store")
	cmdFlags.StringVar(&developCommand, "develop-command", "", "Command to run in each development shell. Each development shell is started as an interactive shell with nix develop whether or not a command is set.")
	cmdFlags.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, log, architecture, platform, codeDir, sourceStore, developCommand); err != nil {
		log.Error("fatal error", slog.Any("error", err))
		os.Exit(1)
	}
	log.Info("Runtime complete")
}

func run(ctx context.Context, log *slog.Logger, architecture, platform, codeDir, sourceStore, developCommand string) (err error) {
	log = log.With(slog.String("architecture", architecture), slog.String("platform", platform))
	log.Info("Restoring Nix store from export", slog.String("source-store", sourceStore))

	// nix copy --all --no-check-sigs --from file:///nix-export/nix-store/
	// nix copy --all --derivation --no-check-sigs --from file:///nix-export/nix-store/
	if err = nixcmd.CopyFromAll(log, os.Stdout, os.Stderr, codeDir, sourceStore); err != nil {
		return fmt.Errorf("failed to copy from /nix-export/nix-store: %w", err)
	}

	log.Info("Gathering Nix outputs")
	// nix flake show --json
	op, err := nixcmd.FlakeShow(os.Stdout, os.Stderr, codeDir)
	if err != nil {
		return fmt.Errorf("failed to gather nix outputs: %w", err)
	}
	drvs := op.Derivations(architecture, platform)

	log.Info("Building", slog.Any("outputs", drvs))
	for _, ref := range drvs {
		log.Info("Building", slog.String("ref", ref))
		// nix build <ref>
		if err := nixcmd.Build(os.Stdout, os.Stderr, codeDir, ref); err != nil {
			log.Error("failed to build", slog.String("ref", ref), slog.Any("error", err))
			return fmt.Errorf("failed to build %q: %w", ref, err)
		}
	}

	for _, ref := range drvs {
		if !strings.HasPrefix(ref, ".#devShells.") {
			continue
		}
		log.Info("Starting interactive development shell", slog.String("ref", ref), slog.String("command", developCommand))
		// nix develop <ref>
		if err := nixcmd.Develop(ctx, os.Stdout, os.Stderr, codeDir, ref, developCommand); err != nil {
			log.Error("failed to use interactive development shell", slog.String("ref", ref), slog.Any("error", err))
			return fmt.Errorf("failed to use interactive development shell %q: %w", ref, err)
		}
	}

	return nil
}
