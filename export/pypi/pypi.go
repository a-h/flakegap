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

func Export(ctx context.Context, log *slog.Logger, stdout, stderr io.Writer, platform, architecture, codePath, requirementsTxtPath string) error {
	// Get PyPi packages.
	log.Info("Exporting Python requirements", slog.String("requirementsTxtPath", requirementsTxtPath))

	// Create output directory.
	outputDirectory := filepath.Join(codePath, "packages/pypi")
	if err := os.MkdirAll(outputDirectory, 0770); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	pythonPlatform, err := getPythonPlatform(platform, architecture)
	if err != nil {
		return err
	}
	packagesForPlatform, err := getPackagesForPlatform(ctx, stdout, stderr, codePath, requirementsTxtPath, pythonPlatform)
	if err != nil {
		return fmt.Errorf("failed to get packages for platform %s: %w", pythonPlatform, err)
	}
	files, err := getDownloadInfo(packagesForPlatform, outputDirectory)
	if err != nil {
		return fmt.Errorf("failed to get download info for platform %s: %w", pythonPlatform, err)
	}

	log.Info("Found pip packages", slog.Int("packages", len(files)))

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

func getPackagesForPlatform(ctx context.Context, stdout, stderr io.Writer, codePath, requirementsTxtPath, pythonPlatform string) (packages []Package, err error) {
	outputFileName := filepath.Join(os.TempDir(), fmt.Sprintf("flakegap-pypi-%d.json", time.Now().Unix()))
	defer os.Remove(outputFileName)

	// nix develop --command -- pip install -r requirements.txt --ignore-installed --dry-run --platform manylinux2014_x86_64 --only-binary=:all: --report output.json
	if err := nixcmd.Develop(ctx, stdout, stderr, codePath, "pip", "install", "-r", requirementsTxtPath, "--ignore-installed", "--platform", pythonPlatform, "--only-binary", ":all:", "--dry-run", "--report", outputFileName); err != nil {
		return nil, fmt.Errorf("failed to run pip install: %w", err)
	}
	fileContents, err := os.ReadFile(outputFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read pip install report file: %w", err)
	}
	var output Output
	if err := json.Unmarshal(fileContents, &output); err != nil {
		return nil, fmt.Errorf("failed to parse pip install report file: %w", err)
	}
	return output.Install, nil
}

func getDownloadInfo(packages []Package, outputDirectory string) (files []download.File, err error) {
	files = make([]download.File, len(packages))
	for i, pkg := range packages {
		url := pkg.DownloadInfo.URL
		if !strings.HasSuffix(url, ".whl") && !strings.HasSuffix(url, ".tar.gz") {
			return nil, fmt.Errorf("unexpected type in URL: %s", url)
		}
		fileName := filepath.Join(outputDirectory, path.Base(url))
		if pkg.DownloadInfo.ArchiveInfo.Hashes == nil {
			return nil, fmt.Errorf("no hashes found for package %s version %s", pkg.Metadata.Name, pkg.Metadata.Version)
		}
		expectedSum, sumFound := pkg.DownloadInfo.ArchiveInfo.Hashes["sha256"]
		if !sumFound {
			return nil, fmt.Errorf("sha256 hash not found for package %s version %s", pkg.Metadata.Name, pkg.Metadata.Version)
		}
		files[i] = download.File{
			URL:            pkg.DownloadInfo.URL,
			TargetFileName: fileName,
			Hash:           expectedSum,
		}
	}
	return files, nil
}

var platformToArchitectureMap = map[string]map[string]string{
	"linux": {
		"x86_64":  "manylinux2014_x86_64",
		"aarch64": "manylinux2014_aarch64",
	},
	"darwin": {
		"x86_64":  "macosx_10_9_x86_64",
		"aarch64": "macosx_11_0_arm64",
	},
}

func getPythonPlatform(platform, architecture string) (string, error) {
	platform = strings.ToLower(platform)
	architecture = strings.ToLower(architecture)
	architectureToPythonPlatformMap, ok := platformToArchitectureMap[platform]
	if !ok {
		return "", fmt.Errorf("unknown python platform %s (architecture %s)", platform, architecture)
	}
	pythonPlatform, ok := architectureToPythonPlatformMap[architecture]
	if !ok {
		return "", fmt.Errorf("unknown python architecture %s for platform %s", architecture, platform)
	}
	return pythonPlatform, nil
}
