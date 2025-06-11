package pypi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/flakegap/export/download"
	"github.com/a-h/flakegap/nixcmd"
)

// pip install -r requirements.txt --dry-run --report output.json
type Output struct {
	Install []Package `json:"install"`
}

type Package struct {
	DownloadInfo DownloadInfo `json:"download_info"`
	Metadata     Metadata     `json:"metadata"`
}

type DownloadInfo struct {
	URL         string      `json:"url"`
	ArchiveInfo ArchiveInfo `json:"archive_info"`
}

type ArchiveInfo struct {
	// Hashes is a map of hash names to their values, e.g. sha256: <hex-encoded-hash>
	Hashes map[string]string `json:"hashes"`
}

type Metadata struct {
	Version string `json:"version"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

func Export(ctx context.Context, log *slog.Logger, stdout, stderr io.Writer, codePath, requirementsTxtPath string) error {
	// Get PyPi packages.
	log.Info("Exporting Python requirements", slog.String("requirementsTxtPath", requirementsTxtPath))
	outputFileName := filepath.Join(os.TempDir(), fmt.Sprintf("flakegap-pypi-%d.json", time.Now().Unix()))
	defer os.Remove(outputFileName)
	// pip install -r requirements.txt --dry-run --report output.json
	if err := nixcmd.Develop(ctx, stdout, stderr, codePath, "pip", "install", "-r", requirementsTxtPath, "--dry-run", "--report", outputFileName); err != nil {
		return fmt.Errorf("failed to run pip install: %w", err)
	}
	fileContents, err := os.ReadFile(outputFileName)
	if err != nil {
		return fmt.Errorf("failed to read pip install report file: %w", err)
	}
	var output Output
	if err := json.Unmarshal(fileContents, &output); err != nil {
		return fmt.Errorf("failed to parse pip install report file: %w", err)
	}
	downloadsTotal := len(output.Install)
	log.Info("Found pip packages", slog.Int("packages", downloadsTotal))

	// Create output directory.
	outputDirectory := filepath.Join(codePath, "packages/pypi")
	if err := os.MkdirAll(outputDirectory, 0770); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create input files from the pip install report.
	files := make([]download.File, downloadsTotal)
	for i, pkg := range output.Install {
		url := pkg.DownloadInfo.URL
		if !strings.HasSuffix(url, ".whl") && !strings.HasSuffix(url, ".tar.gz") {
			return fmt.Errorf("unexpected type in URL: %s", url)
		}
		fileName := filepath.Join(outputDirectory, path.Base(url))
		if pkg.DownloadInfo.ArchiveInfo.Hashes == nil {
			return fmt.Errorf("no hashes found for package %s version %s", pkg.Metadata.Name, pkg.Metadata.Version)
		}
		expectedSum, sumFound := pkg.DownloadInfo.ArchiveInfo.Hashes["sha256"]
		if !sumFound {
			return fmt.Errorf("sha256 hash not found for package %s version %s", pkg.Metadata.Name, pkg.Metadata.Version)
		}
		files[i] = download.File{
			URL:            pkg.DownloadInfo.URL,
			TargetFileName: fileName,
			Hash:           expectedSum,
		}
	}

	// Use the downloader to download the packages concurrently.
	downloads := make(chan download.File)
	go func() {
		defer close(downloads)
		for _, file := range files {
			downloads <- file
		}
	}()
	return download.Files(ctx, log, 4, sha256.New, downloads)
}
