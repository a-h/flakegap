package nixcmd

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestTopoSortPaths(t *testing.T) {
	tests := []struct {
		name     string
		deps     map[string][]string
		validate func(t *testing.T, result []string)
	}{
		{
			name: "paths with no dependencies can be sorted",
			deps: map[string][]string{
				"/nix/store/aaa": {},
				"/nix/store/bbb": {},
			},
			validate: func(t *testing.T, result []string) {
				if len(result) != 2 {
					t.Errorf("expected 2 paths, got %d", len(result))
				}
			},
		},
		{
			name: "dependency comes before dependent",
			deps: map[string][]string{
				"/nix/store/python.drv": {"/nix/store/bash.drv"},
				"/nix/store/bash.drv":   {},
			},
			validate: func(t *testing.T, result []string) {
				bashIdx := slices.Index(result, "/nix/store/bash.drv")
				pythonIdx := slices.Index(result, "/nix/store/python.drv")
				if bashIdx == -1 || pythonIdx == -1 {
					t.Fatalf("missing expected paths in result %v", result)
				}
				if bashIdx >= pythonIdx {
					t.Errorf("bash (index %d) should come before python (index %d)", bashIdx, pythonIdx)
				}
			},
		},
		{
			name: "transitive dependencies are ordered correctly",
			deps: map[string][]string{
				"/nix/store/c": {"/nix/store/b"},
				"/nix/store/b": {"/nix/store/a"},
				"/nix/store/a": {},
			},
			validate: func(t *testing.T, result []string) {
				ai := slices.Index(result, "/nix/store/a")
				bi := slices.Index(result, "/nix/store/b")
				ci := slices.Index(result, "/nix/store/c")
				if ai >= bi || bi >= ci {
					t.Errorf("expected a < b < c, got indices a=%d, b=%d, c=%d", ai, bi, ci)
				}
			},
		},
		{
			name: "independent paths are emitted in alphabetical order",
			deps: map[string][]string{
				"/nix/store/zzz-last":  {},
				"/nix/store/aaa-first": {},
				"/nix/store/mmm-mid":   {},
			},
			validate: func(t *testing.T, result []string) {
				if len(result) != 3 {
					t.Fatalf("expected 3 paths, got %d", len(result))
				}
				expected := []string{
					"/nix/store/aaa-first",
					"/nix/store/mmm-mid",
					"/nix/store/zzz-last",
				}
				for i, want := range expected {
					if result[i] != want {
						t.Errorf("index %d: got %q, want %q", i, result[i], want)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := topoSortPaths(tt.deps)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, result)
		})
	}
}

func TestSortStorePathsByDependency(t *testing.T) {
	dir := t.TempDir()
	writeNarinfo(t, dir, "aaaa.narinfo", "/nix/store/aaaa-python.drv", "bbbb-bash.drv")
	writeNarinfo(t, dir, "bbbb.narinfo", "/nix/store/bbbb-bash.drv", "")

	result, err := sortStorePathsByDependency(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(result), result)
	}
	bashIdx := slices.Index(result, "/nix/store/bbbb-bash.drv")
	pythonIdx := slices.Index(result, "/nix/store/aaaa-python.drv")
	if bashIdx == -1 || pythonIdx == -1 {
		t.Fatalf("missing expected paths in result %v", result)
	}
	if bashIdx >= pythonIdx {
		t.Errorf("bash (index %d) should come before python (index %d)", bashIdx, pythonIdx)
	}
}

func writeNarinfo(t *testing.T, dir, filename, storePath, refs string) {
	t.Helper()
	content := "StorePath: " + storePath + "\nURL: nar/abc.nar.xz\n"
	if refs != "" {
		content += "References: " + refs + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
