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

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error

	switch os.Args[1] {
	case "export":
		args := export.Args{}
		cmdFlags := flag.NewFlagSet("export", flag.ContinueOnError)
		cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
		cmdFlags.StringVar(&args.ExportFileName, "target-path", "", "Filename to write the output file to - defaults to <source-path>/nix-export.tar.gz")
		cmdFlags.Parse(os.Args[2:])
		if args.ExportFileName == "" {
			args.ExportFileName = filepath.Join(args.Code, "nix-export.tar.gz")
		}
		err = export.Run(ctx, log, args)
	case "validate":
		args := validate.Args{}
		cmdFlags := flag.NewFlagSet("validate", flag.ContinueOnError)
		cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
		cmdFlags.StringVar(&args.ExportFileName, "nix-export-path", "", "Filename of the nix-export.tar.gz file, defaults to <source-path>/nix-export.tar.gz")
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

  flakegap export --source-path <path-to-flake-dir-on-disk>
    - Starts a container that runs required export commands.

  flakegap validate --source-path <path-to-flake-dir-on-disk>
    - Validates that the export worked by running a build in an airgapped container.`)
}
