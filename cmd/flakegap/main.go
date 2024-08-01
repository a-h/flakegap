package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/flakegap/export"
	"github.com/a-h/flakegap/nixserve"
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
	case "serve":
		err = serveCmd(ctx, log)
	case "export":
		err = exportCmd(ctx, log)
	case "validate":
		err = validateCmd(ctx, log)
	default:
		fmt.Printf("flakegap: unknown command %q\n", os.Args[1])
		fmt.Println()
		printUsage()
	}

	if err != nil {
		log.Error("error", slog.Any("error", err))
		os.Exit(1)
	}
}

func serveCmd(ctx context.Context, log *slog.Logger) error {
	server := &http.Server{}

	cmdFlags := flag.NewFlagSet("serve", flag.ContinueOnError)
	cmdFlags.StringVar(&server.Addr, "addr", "localhost:41805", "Listen address for Nix binary cache")
	cmdFlags.Parse(os.Args[2:])

	// Start the binary cache.
	h, closer, err := nixserve.New(log)
	if err != nil {
		return fmt.Errorf("failed to create nixserve: %w", err)
	}
	server.Handler = h
	defer closer()

	go func() {
		log.Info("Starting server", slog.String("addr", server.Addr))
		<-ctx.Done()
		if err := server.Shutdown(ctx); err != nil {
			log.Error("failed to shutdown server", slog.Any("error", err))
		}
	}()

	return server.ListenAndServe()
}

func exportCmd(ctx context.Context, log *slog.Logger) error {
	args := export.Args{}
	cmdFlags := flag.NewFlagSet("export", flag.ContinueOnError)
	cmdFlags.StringVar(&args.Code, "source-path", ".", "Path to the directory containing the flake.")
	cmdFlags.StringVar(&args.ExportFileName, "export-filename", "", "Filename to write the output file to - defaults to <source-path>/nix-export.tar.gz")
	cmdFlags.StringVar(&args.Image, "image", "ghcr.io/a-h/flakegap:latest", "Image to run")
	cmdFlags.StringVar(&args.BinaryCacheAddr, "binary-cache-addr", "localhost:41805", "Listen address for Nix binary cache")
	cmdFlags.Parse(os.Args[2:])
	if args.ExportFileName == "" {
		args.ExportFileName = filepath.Join(args.Code, "nix-export.tar.gz")
	}
	return export.Run(ctx, log, args)
}

func validateCmd(ctx context.Context, log *slog.Logger) error {
	args := validate.Args{}
	cmdFlags := flag.NewFlagSet("validate", flag.ContinueOnError)
	cmdFlags.StringVar(&args.ExportFileName, "export-filename", "nix-export.tar.gz", "Filename of the nix-export.tar.gz file, defaults to nix-export.tar.gz")
	cmdFlags.StringVar(&args.Image, "image", "ghcr.io/a-h/flakegap:latest", "Image to run")
	cmdFlags.Parse(os.Args[2:])
	return validate.Run(ctx, log, args)
}

func printUsage() {
	fmt.Println(`flakegap

Export Nix packages required to build a flake on an airgapped system.

Usage:

  flakegap export
    - Starts a container that runs required export commands.

  flakegap validate
    - Validates that the export worked by running a build in an airgapped container.

  flakegap serve
    - Serve a local binary cache for the airgapped system to use (started automatically by export).`)
}
