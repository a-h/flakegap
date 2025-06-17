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
	log.Info("Exporting Python requirements", slog.String("requirements", requirementsTxtPath))

	// Create output directory.
	outputDirectory := filepath.Join(codePath, "packages/pypi")
	if err := os.MkdirAll(outputDirectory, 0770); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	pythonPlatforms, err := getPythonPlatforms(platform, architecture)
	if err != nil {
		return err
	}
	packagesForPlatform, err := getPackagesForPlatform(ctx, stdout, stderr, codePath, requirementsTxtPath, pythonPlatforms)
	if err != nil {
		return fmt.Errorf("failed to get packages: %w", err)
	}
	files, err := getDownloadInfo(packagesForPlatform, outputDirectory)
	if err != nil {
		return fmt.Errorf("failed to get download info: %w", err)
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

func getPackagesForPlatform(ctx context.Context, stdout, stderr io.Writer, codePath, requirementsTxtPath string, pythonPlatforms []string) (packages []Package, err error) {
	outputFileName := filepath.Join(os.TempDir(), fmt.Sprintf("flakegap-pypi-%d.json", time.Now().Unix()))
	defer os.Remove(outputFileName)

	// Construct command.
	// nix develop --command -- pip install -r requirements.txt --ignore-installed --dry-run --platform manylinux2014_x86_64 --only-binary=:all: --report output.json
	cmd := []string{"pip", "install", "-r", requirementsTxtPath, "--ignore-installed"}
	for _, platform := range pythonPlatforms {
		cmd = append(cmd, "--platform", platform)
	}
	cmd = append(cmd, "--only-binary", ":all:", "--dry-run", "--report", outputFileName)

	if err := nixcmd.Develop(ctx, stdout, stderr, codePath, cmd...); err != nil {
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

const (
	minLibc2Version = 17
	maxLibc2Version = 40
)

func getLinuxPythonPlatforms(architecture string) (platforms []string) {
	platforms = make([]string, maxLibc2Version-minLibc2Version+2)
	for i := maxLibc2Version; i >= minLibc2Version; i-- {
		platforms[maxLibc2Version-i] = fmt.Sprintf("manylinux_2_%d_%s", i, architecture)
	}
	platforms[maxLibc2Version-minLibc2Version+1] = fmt.Sprintf("manylinux2014_%s", architecture)
	return platforms
}

func getMacOSPythonPlatformsX86_64() (platforms []string) {
	platforms = append(platforms, "macosx_11_0_universal2")
	for major := 13; major >= 10; major-- {
		var minMinor int
		if major == 10 {
			minMinor = 9
		} else {
			minMinor = 0
		}
		for minor := 99; minor >= minMinor; minor-- {
			platforms = append(platforms, fmt.Sprintf("macosx_%d_%d_x86_64", major, minor))
		}
	}
	return platforms
}

func getMacOSPythonPlatformsAarch64() (platforms []string) {
	platforms = append(platforms, "macosx_11_0_universal2")
	for major := 14; major >= 11; major-- {
		for minor := 99; minor >= 0; minor-- {
			platforms = append(platforms, fmt.Sprintf("macosx_%d_%d_arm64", major, minor))
		}
	}
	return platforms
}

var platformToArchitectureMap = map[string]map[string][]string{
	"linux": {
		"x86_64":  getLinuxPythonPlatforms("x86_64"),
		"aarch64": getLinuxPythonPlatforms("aarch64"),
	},
	"darwin": {
		"x86_64":  getMacOSPythonPlatformsX86_64(),
		"aarch64": getMacOSPythonPlatformsAarch64(),
	},
}

func getPythonPlatforms(platform, architecture string) ([]string, error) {
	platform = strings.ToLower(platform)
	architecture = strings.ToLower(architecture)
	architectureToPythonPlatformMap, ok := platformToArchitectureMap[platform]
	if !ok {
		return nil, fmt.Errorf("unknown python platform %s (architecture %s)", platform, architecture)
	}
	pythonPlatforms, ok := architectureToPythonPlatformMap[architecture]
	if !ok {
		return nil, fmt.Errorf("unknown python architecture %s for platform %s", architecture, platform)
	}
	return pythonPlatforms, nil
}
