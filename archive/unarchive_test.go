package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testFile struct {
	content string
	mode    os.FileMode
}

type testEntry struct {
	header  tar.Header
	content string
}

func writeTestArchive(t *testing.T, path string, entries []testEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create archive: %v", err)
	}
	defer f.Close()
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		e.header.Size = int64(len(e.content))
		if err := tw.WriteHeader(&e.header); err != nil {
			t.Fatalf("failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatalf("failed to write content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
}

func TestUnarchive(t *testing.T) {
	tests := []struct {
		name            string
		entries         func(outside string) []testEntry
		expectError     bool
		expectedMetrics Metrics
		expectedFiles   map[string]testFile
	}{
		{
			name: "files, directories, and symlinks within the destination are extracted",
			entries: func(outside string) []testEntry {
				return []testEntry{
					{header: tar.Header{Name: "dir", Typeflag: tar.TypeDir, Mode: 0755}},
					{header: tar.Header{Name: "dir/file.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "hello"},
					{header: tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "dir"}},
					{header: tar.Header{Name: "link/via-link.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "world"},
					{header: tar.Header{Name: "dir/script.sh", Typeflag: tar.TypeReg, Mode: 0755}, content: "#!/bin/sh"},
				}
			},
			expectedMetrics: Metrics{Files: 4, Dirs: 1},
			expectedFiles: map[string]testFile{
				"dir/file.txt":     {content: "hello", mode: 0644},
				"dir/via-link.txt": {content: "world", mode: 0644},
				"dir/script.sh":    {content: "#!/bin/sh", mode: 0755},
			},
		},
		{
			name: "a file listed twice takes the content and mode of the last entry",
			entries: func(outside string) []testEntry {
				return []testEntry{
					{header: tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "first entry"},
					{header: tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Mode: 0700}, content: "second"},
				}
			},
			expectedMetrics: Metrics{Files: 2},
			expectedFiles: map[string]testFile{
				"file.txt": {content: "second", mode: 0700},
			},
		},
		{
			name: "a file written through a symlink that points outside the destination is rejected",
			entries: func(outside string) []testEntry {
				return []testEntry{
					{header: tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: outside}},
					{header: tar.Header{Name: "escape/pwned.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "pwned"},
				}
			},
			expectError:     true,
			expectedMetrics: Metrics{Files: 2},
			expectedFiles:   map[string]testFile{},
		},
		{
			name: "a file written through a relative symlink that traverses out of the destination is rejected",
			entries: func(outside string) []testEntry {
				return []testEntry{
					{header: tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "../outside"}},
					{header: tar.Header{Name: "escape/pwned.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "pwned"},
				}
			},
			expectError:     true,
			expectedMetrics: Metrics{Files: 2},
			expectedFiles:   map[string]testFile{},
		},
		{
			name: "a path that traverses out of the destination is rejected",
			entries: func(outside string) []testEntry {
				return []testEntry{
					{header: tar.Header{Name: "../outside/pwned.txt", Typeflag: tar.TypeReg, Mode: 0644}, content: "pwned"},
				}
			},
			expectError:   true,
			expectedFiles: map[string]testFile{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			if err := os.Mkdir(outside, 0755); err != nil {
				t.Fatalf("failed to create outside directory: %v", err)
			}
			src := filepath.Join(base, "archive.tar.gz")
			writeTestArchive(t, src, tt.entries(outside))
			dst := filepath.Join(base, "dst")
			if err := os.Mkdir(dst, 0755); err != nil {
				t.Fatalf("failed to create destination directory: %v", err)
			}

			m, err := Unarchive(context.Background(), slog.New(slog.DiscardHandler), src, dst)
			if tt.expectError && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expectedMetrics, m); diff != "" {
				t.Errorf("unexpected metrics (-want +got):\n%s", diff)
			}

			outsideEntries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatalf("failed to read outside directory: %v", err)
			}
			if len(outsideEntries) != 0 {
				t.Errorf("expected no files outside the destination, got %v", outsideEntries)
			}
			pathToFile := map[string]testFile{}
			for name := range tt.expectedFiles {
				content, err := os.ReadFile(filepath.Join(dst, name))
				if err != nil {
					t.Fatalf("failed to read %q: %v", name, err)
				}
				info, err := os.Stat(filepath.Join(dst, name))
				if err != nil {
					t.Fatalf("failed to stat %q: %v", name, err)
				}
				pathToFile[name] = testFile{content: string(content), mode: info.Mode().Perm()}
			}
			if diff := cmp.Diff(tt.expectedFiles, pathToFile, cmp.AllowUnexported(testFile{})); diff != "" {
				t.Errorf("unexpected files (-want +got):\n%s", diff)
			}
		})
	}
}
