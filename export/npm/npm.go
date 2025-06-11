package npm

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/a-h/flakegap/export/download"
)

type NPMLock struct {
	Name     string             `json:"name"`
	Version  string             `json:"version"`
	Packages map[string]Package `json:"packages"`
}

type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Resolved     string            `json:"resolved"`
	Integrity    string            `json:"integrity"`
	Dependencies map[string]string `json:"dependencies"`
}

func Export(ctx context.Context, log *slog.Logger, stdout, stderr io.Writer, codePath, lockFilePath string) error {
	// Parse the lock file.
	log.Info("Parsing NPM lock file", slog.String("lock-file", lockFilePath))
	lockFile, err := parseLockFile(lockFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse lock file: %w", err)
	}
	log.Info("Found NPM packages")

	// Create output directory.
	outputDirectory := filepath.Join(codePath, "packages/npm")
	if err := os.MkdirAll(outputDirectory, 0770); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create input files from the lock file.
	var files []download.File
	for _, pkg := range lockFile.Packages {
		// If there's no URL, skip.
		if pkg.Resolved == "" {
			continue
		}
		// Get the target file name.
		url := pkg.Resolved
		fileName := filepath.Join(outputDirectory, path.Base(url))
		// Base64 decode the hash.
		decodedHash, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(pkg.Integrity, "sha512-"))
		if err != nil {
			return fmt.Errorf("failed to decode integrity hash for package %s: %w", pkg.Name, err)
		}
		hash := fmt.Sprintf("%x", decodedHash)
		files = append(files, download.File{
			URL:            url,
			TargetFileName: fileName,
			Hash:           hash,
		})
	}

	// Use the downloader to download the packages concurrently.
	downloads := make(chan download.File)
	go func() {
		defer close(downloads)
		for _, file := range files {
			downloads <- file
		}
	}()
	return download.Files(ctx, log, 4, sha512.New, downloads)
}

func parseLockFile(fileName string) (lockFile NPMLock, err error) {
	f, err := os.Open(fileName)
	if err != nil {
		return lockFile, fmt.Errorf("failed to open lock file %s: %w", fileName, err)
	}
	err = json.NewDecoder(f).Decode(&lockFile)
	return lockFile, err
}
