package nixcmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nixTestVersions lists the Nix versions to test, one per supported NixOS release.
// Update when a new stable NixOS release ships and drop the oldest entry.
var nixTestVersions = []struct {
	name string
	ref  string
}{
	{name: "nixos-25.11", ref: "github:NixOS/nixpkgs/nixos-25.11#nix"},
	{name: "nixos-26.05", ref: "github:NixOS/nixpkgs/nixos-26.05#nix"},
	{name: "nixos-unstable", ref: "github:NixOS/nixpkgs/nixos-unstable#nix"},
}

// getNixBinary returns the path to the nix binary for the given flake reference.
func getNixBinary(t *testing.T, ref string) (bin string) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Nix version download in short mode")
	}
	cmd := exec.Command("nix", "build", ref, "--no-link", "--print-out-paths")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix build %s: %v\n%s", ref, err, string(out))
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "-man") || strings.HasSuffix(line, "-dev") {
			continue
		}
		p := filepath.Join(line, "bin", "nix")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("no nix binary found in build output for %s", ref)
	return ""
}

func TestFlakeShowOutputCanBeParsed(t *testing.T) {
	for _, v := range nixTestVersions {
		t.Run(v.name, func(t *testing.T) {
			bin := getNixBinary(t, v.ref)
			t.Setenv("PATH", filepath.Dir(bin)+":"+os.Getenv("PATH"))

			var stdout, stderr bytes.Buffer
			op, err := FlakeShow(&stdout, &stderr, "..")
			if err != nil {
				t.Fatalf("FlakeShow failed: %v\nstderr: %s", err, stderr.String())
			}
			drvs := op.Derivations("x86_64", "linux")
			if len(drvs) == 0 {
				t.Error("no x86_64-linux derivations found in flake show output")
			}
		})
	}
}

func TestPathInfoOutputCanBeParsed(t *testing.T) {
	for _, v := range nixTestVersions {
		t.Run(v.name, func(t *testing.T) {
			bin := getNixBinary(t, v.ref)
			// Use the nix store path of the binary itself as a known, already-fetched path.
			storePath := filepath.Dir(filepath.Dir(bin))
			t.Setenv("PATH", filepath.Dir(bin)+":"+os.Getenv("PATH"))

			var stdout, stderr bytes.Buffer
			paths, err := PathInfo(&stdout, &stderr, "..", false, false, storePath)
			if err != nil {
				t.Fatalf("PathInfo failed: %v\nstderr: %s", err, stderr.String())
			}
			if len(paths) == 0 {
				t.Errorf("no paths returned for store path %s", storePath)
			}
			if strings.Contains(stderr.String(), "--json-format") {
				t.Errorf("unexpected --json-format deprecation warning in stderr: %s", stderr.String())
			}
		})
	}
}
