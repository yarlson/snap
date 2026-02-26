package snapshot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Snapshotter creates non-disruptive git stash snapshots.
type Snapshotter struct {
	dir string
}

// New creates a Snapshotter for the given directory.
func New(dir string) *Snapshotter {
	return &Snapshotter{dir: dir}
}

// Capture creates a stash snapshot with the given message.
// Returns true if a snapshot was created, false if working tree was clean.
// Includes untracked files in the snapshot.
// Preserves the exact prior index state (staged files remain staged).
func (s *Snapshotter) Capture(ctx context.Context, message string) (bool, error) {
	// Save the current index tree so we can restore it afterwards.
	// This preserves any intentionally-staged files that existed before this call.
	indexTree, err := s.gitOutput(ctx, "write-tree")
	if err != nil {
		return false, fmt.Errorf("save index: %w", err)
	}

	// Stage all files (including untracked) so they're included in the snapshot.
	// git stash create only captures staged+unstaged changes to tracked files,
	// so we must add untracked files to the index first.
	if err := s.git(ctx, "add", "."); err != nil {
		return false, fmt.Errorf("stage: %w", err)
	}

	// Create a stash object without touching the working tree.
	stashID, err := s.gitOutput(ctx, "stash", "create", message)
	if err != nil {
		// Restore index on error using background context (cleanup must succeed).
		//nolint:contextcheck,errcheck // cleanup must succeed even if parent context is cancelled
		_ = s.restoreIndex(context.Background(), indexTree)
		return false, fmt.Errorf("stash create: %w", err)
	}

	// Restore the exact prior index state so untracked files go back to untracked
	// and previously-staged files remain staged. Working tree is untouched.
	if err := s.restoreIndex(ctx, indexTree); err != nil {
		return false, fmt.Errorf("restore index: %w", err)
	}

	// Empty output means working tree was clean — nothing to snapshot.
	if stashID == "" {
		return false, nil
	}

	// Store the stash object in the reflog.
	if err := s.git(ctx, "stash", "store", "-m", message, stashID); err != nil {
		return false, fmt.Errorf("stash store: %w", err)
	}

	return true, nil
}

// restoreIndex restores the git index to a previously-saved tree state.
func (s *Snapshotter) restoreIndex(ctx context.Context, treeID string) error {
	if treeID == "" {
		// Empty tree means index was clean; restore to HEAD.
		return s.git(ctx, "reset")
	}
	// Restore to the saved tree state.
	return s.git(ctx, "read-tree", treeID)
}

func (s *Snapshotter) git(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Snapshotter) gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
