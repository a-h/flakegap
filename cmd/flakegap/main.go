package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/export"
	"github.com/a-h/flakegap/validate"
)

var version string

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	log = log.With(slog.String("version", version))
	log = log.With(slog.String("flakegap", "client"))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "export":
		args := export.Args{}
		cmdFlags := flag.NewFlagSet("export", flag.ContinueOnError)
		cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
		cmdFlags.StringVar(&args.ExportFileName, "export-filename", "", "Filename to write the output file to - defaults to <source-path>/nix-export.tar.gz")
		cmdFlags.StringVar(&args.ExportManifestFileName, "manifest-filename", "", "Filename to write the manifest to - defaults to <source-path>/nix-export.txt")
		cmdFlags.StringVar(&args.Image, "image", "ghcr.io/a-h/flakegap:main", "Image to run")
		cmdFlags.Parse(os.Args[2:])
		if args.ExportFileName == "" {
			args.ExportFileName = filepath.Join(args.Code, "nix-export.tar.gz")
		}
		if args.ExportManifestFileName == "" {
			args.ExportManifestFileName = filepath.Join(args.Code, "nix-export.txt")
		}
		err = export.Run(ctx, log, args)
	case "validate":
		args := validate.Args{}
		cmdFlags := flag.NewFlagSet("validate", flag.ContinueOnError)
		cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
		cmdFlags.StringVar(&args.ExportFileName, "export-filename", "", "Filename of the nix-export.tar.gz file, defaults to <source-path>/nix-export.tar.gz")
		cmdFlags.StringVar(&args.Image, "image", "ghcr.io/a-h/flakegap:main", "Image to run")
		cmdFlags.Parse(os.Args[2:])
		if args.ExportFileName == "" {
			args.ExportFileName = filepath.Join(args.Code, "nix-export.tar.gz")
		}
		err = validate.Run(ctx, log, args)
	default:
		fmt.Println("flakegap: unknown command")
		fmt.Println()
		printUsage()
	}

	if err != nil {
		log.Error("error", slog.Any("error", err))
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`flakegap

Export Nix packages required to build a flake on an airgapped system.

Usage:

  flakegap export
    - Starts a container that runs required export commands.

  flakegap validate
    - Validates that the export worked by running a build in an airgapped container.`)
}
