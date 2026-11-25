package source

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-cmp/cmp"
)

func TestGitValidate(t *testing.T) {
	tests := []struct {
		name          string
		isRepo        bool
		uncommitted   map[string]string
		allowDirty    bool
		expectedError error
	}{
		{
			name:   "clean repositories are valid",
			isRepo: true,
		},
		{
			name:          "repositories with uncommitted changes are rejected",
			isRepo:        true,
			uncommitted:   map[string]string{"untracked.txt": "b"},
			expectedError: ErrUncommittedChanges,
		},
		{
			name:        "repositories with uncommitted changes are valid when allowed",
			isRepo:      true,
			uncommitted: map[string]string{"untracked.txt": "b"},
			allowDirty:  true,
		},
		{
			name:          "directories that are not repositories are rejected",
			isRepo:        false,
			expectedError: git.ErrRepositoryNotExists,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.isRepo {
				dir = createCommittedRepo(t, map[string]string{"a.txt": "a"})
			}
			writeFiles(t, dir, tt.uncommitted)

			err := NewGit(dir, tt.allowDirty).Validate()
			if !errors.Is(err, tt.expectedError) {
				t.Errorf("got error %v, expected %v", err, tt.expectedError)
			}
		})
	}
}

func TestGitCopyTo(t *testing.T) {
	tests := []struct {
		name        string
		committed   map[string]string
		uncommitted map[string]string
		expected    []string
	}{
		{
			name:      "committed files are copied",
			committed: map[string]string{"a.txt": "a", "b.txt": "b"},
			expected:  []string{"a.txt", "b.txt"},
		},
		{
			name:        "files ignored by .gitignore are not copied",
			committed:   map[string]string{".gitignore": "ignored.txt\n", "a.txt": "a"},
			uncommitted: map[string]string{"ignored.txt": "x"},
			expected:    []string{".gitignore", "a.txt"},
		},
		{
			name:        "uncommitted files are not copied",
			committed:   map[string]string{"a.txt": "a"},
			uncommitted: map[string]string{"untracked.txt": "b"},
			expected:    []string{"a.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := createCommittedRepo(t, tt.committed)
			writeFiles(t, dir, tt.uncommitted)
			dst := filepath.Join(t.TempDir(), "source")

			if err := NewGit(dir, true).CopyTo(t.Context(), dst); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.expected, listFiles(t, dst)); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func createCommittedRepo(t *testing.T, nameToContent map[string]string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	writeFiles(t, dir, nameToContent)
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}
	if err = worktree.AddGlob("."); err != nil {
		t.Fatalf("failed to add files: %v", err)
	}
	_, err = worktree.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	return dir
}
