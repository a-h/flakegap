package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/a-h/flakegap/export"
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
		cmdFlags.StringVar(&args.SourcePath, "source-path", ".", "Path to the directory containing the flake.")
		cmdFlags.StringVar(&args.TargetPath, "target-path", ".", "Path to write the output file to.")
		err = export.Run(ctx, log, args)
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

  flakegap export <path-to-flake-on-disk>
    - Starts a container that runs required export commands.`)
}
