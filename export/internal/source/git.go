package source

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
)

// ErrUncommittedChanges is returned by Git.Validate when the repository has uncommitted changes.
var ErrUncommittedChanges = errors.New("source repository has uncommitted changes, so the exported Nix store may not match the exported source; commit the changes, or use -allow-dirty or -use-filesystem-copy")

// Git clones the source repository, so files ignored by .gitignore are not copied.
type Git struct {
	path       string
	allowDirty bool
}

func NewGit(path string, allowDirty bool) Git {
	return Git{path: path, allowDirty: allowDirty}
}

// Validate returns an error if the repository has uncommitted changes, because the clone does not include
// them, so the exported Nix store may not match the exported source.
func (s Git) Validate() (err error) {
	if s.allowDirty {
		return nil
	}
	repo, err := git.PlainOpen(s.path)
	if err != nil {
		return fmt.Errorf("failed to open source directory as git repository: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get git worktree from source repository: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("failed to get worktree status of source repository: %w", err)
	}
	if !status.IsClean() {
		return ErrUncommittedChanges
	}
	return nil
}

// CopyTo clones the repository into dir.
func (s Git) CopyTo(ctx context.Context, dir string) (err error) {
	if _, err = git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{URL: s.path}); err != nil {
		return fmt.Errorf("failed to clone git repository: %w", err)
	}
	return nil
}
