package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/export"
	"github.com/a-h/flakegap/importcmd"
	"github.com/a-h/flakegap/sloghandler"
	"github.com/a-h/flakegap/validate"
)

var version string

func main() {
	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printUsage()
		os.Exit(1)
	}
	ctx := context.Background()
	var err error

	switch os.Args[1] {
	case "version":
		fallthrough
	case "-v":
		fallthrough
	case "--version":
		fallthrough
	case "-version":
		fmt.Println(version)
	case "export":
		err = exportCmd(ctx)
	case "import":
		err = importCmd(ctx)
	case "validate":
		err = validateCmd(ctx)
	default:
		fmt.Printf("flakegap: unknown command %q\n", os.Args[1])
		fmt.Println()
		printUsage()
	}

	if err != nil {
		log := newLogger("error", false, os.Stderr)
		log.Error("error", slog.Any("error", err))
		os.Exit(1)
	}
}

func newLogger(logLevel string, verbose bool, stderr io.Writer) *slog.Logger {
	if verbose {
		logLevel = "debug"
	}
	level := slog.LevelInfo.Level()
	switch logLevel {
	case "debug":
		level = slog.LevelDebug.Level()
	case "warn":
		level = slog.LevelWarn.Level()
	case "error":
		level = slog.LevelError.Level()
	}
	log := slog.New(sloghandler.NewHandler(stderr, &slog.HandlerOptions{
		AddSource: logLevel == "debug",
		Level:     level,
	}))

	log = log.With(slog.String("version", version))
	log = log.With(slog.String("flakegap", "client"))
	return log
}

func exportCmd(ctx context.Context) error {
	args := export.Args{}
	var verboseFlag bool
	var logLevelFlag string
	cmdFlags := flag.NewFlagSet("export", flag.ContinueOnError)
	cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
	cmdFlags.StringVar(&args.ExportFileName, "export-filename", "", "Filename to write the output file to - defaults to <source-path>/nix-export.tar.gz")
	cmdFlags.StringVar(&args.Architecture, "architecture", "x86_64", "Architecture to build for, e.g. x86_64, aarch64")
	cmdFlags.StringVar(&args.Platform, "platform", "linux", "Platform to build for, e.g. linux, darwin")
	cmdFlags.BoolVar(&verboseFlag, "v", false, "")
	cmdFlags.StringVar(&logLevelFlag, "log-level", "info", "")
	cmdFlags.StringVar(&args.TemporaryPath, "temporary-path", "", "Directory to write temporary files to")
	cmdFlags.BoolVar(&args.ExportNix, "export-nix", true, "Export the Nix store paths required to build the flake.")
	cmdFlags.BoolVar(&args.Help, "help", false, "Show usage and quit")
	cmdFlags.Parse(os.Args[2:])
	if args.ExportFileName == "" {
		args.ExportFileName = filepath.Join(args.Code, "nix-export.tar.gz")
	}
	if args.Help {
		cmdFlags.PrintDefaults()
		os.Exit(1)
	}
	log := newLogger(logLevelFlag, verboseFlag, os.Stderr)
	return export.Run(ctx, log, args)
}

func importCmd(ctx context.Context) error {
	args := importcmd.Args{}
	var verboseFlag bool
	var logLevelFlag string
	cmdFlags := flag.NewFlagSet("import", flag.ContinueOnError)
	cmdFlags.StringVar(&args.ImportFileName, "import-filename", "nix-export.tar.gz", "Path to the tar.gz file created by the export command.")
	cmdFlags.BoolVar(&verboseFlag, "v", false, "")
	cmdFlags.StringVar(&logLevelFlag, "log-level", "info", "")
	cmdFlags.StringVar(&args.TemporaryPath, "temporary-path", "", "Directory to write temporary files to")
	cmdFlags.BoolVar(&args.Help, "help", false, "Show usage and quit")
	cmdFlags.Parse(os.Args[2:])
	if args.Help {
		cmdFlags.PrintDefaults()
		os.Exit(1)
	}
	log := newLogger(logLevelFlag, verboseFlag, os.Stderr)
	return importcmd.Run(ctx, log, args)
}

func validateCmd(ctx context.Context) error {
	args := validate.Args{}
	var verboseFlag bool
	var logLevelFlag string
	cmdFlags := flag.NewFlagSet("validate", flag.ContinueOnError)
	cmdFlags.StringVar(&args.ExportFileName, "export-filename", "nix-export.tar.gz", "Filename of the nix-export.tar.gz file, defaults to nix-export.tar.gz")
	cmdFlags.StringVar(&args.Platform, "platform", "amd64", "Platform to run the export on, e.g. amd64 / x86_64, arm64 / aarch64")
	cmdFlags.StringVar(&args.Image, "image", "ghcr.io/a-h/flakegap:latest", "Image to run")
	cmdFlags.BoolVar(&verboseFlag, "v", false, "")
	cmdFlags.StringVar(&logLevelFlag, "log-level", "info", "")
	cmdFlags.BoolVar(&args.Help, "help", false, "Show usage and quit")
	cmdFlags.Parse(os.Args[2:])
	if args.Help {
		cmdFlags.PrintDefaults()
		os.Exit(1)
	}
	log := newLogger(logLevelFlag, verboseFlag, os.Stderr)
	return validate.Run(ctx, log, args)
}

func printUsage() {
	fmt.Println(`flakegap

Export Nix packages required to build a flake on an airgapped system.

Usage:

  flakegap export
    - Exports all of the source code, builds, devShells and dependencies.

  flakegap validate
    - Validates that the export worked by running a build in an airgapped container.

  flakegap import
    - Imports the output of the export command into the local Nix store.

  flakegap version
    - Print the version of flakegap.`)
}
