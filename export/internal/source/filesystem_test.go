package source

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFilesystemCopyTo(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected []string
	}{
		{
			name:     "all files are copied",
			files:    map[string]string{"a.txt": "a", "sub/b.txt": "b"},
			expected: []string{"a.txt", "sub/b.txt"},
		},
		{
			name:     "files ignored by .gitignore are copied",
			files:    map[string]string{".gitignore": "ignored.txt\n", "ignored.txt": "x"},
			expected: []string{".gitignore", "ignored.txt"},
		},
		{
			name:     "build outputs and previous exports are not copied",
			files:    map[string]string{"a.txt": "a", "coverage.out": "c", "nix-export.tar.gz": "n", ".direnv/env": "e"},
			expected: []string{"a.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			dst := filepath.Join(t.TempDir(), "source")

			if err := NewFilesystem(dir).CopyTo(t.Context(), dst); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.expected, listFiles(t, dst)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func writeFiles(t *testing.T, dir string, nameToContent map[string]string) {
	t.Helper()
	for name, content := range nameToContent {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
}

func listFiles(t *testing.T, dir string) (names []string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		names = append(names, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	return names
}
