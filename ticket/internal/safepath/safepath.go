// Package safepath provides symlink-aware path-containment checks used by the
// ticket runtime to guard sidecar reads and writes.
//
// The defense-in-depth model is: caller-supplied identifiers (ticket IDs, run
// IDs) are already validated for shape elsewhere; safepath catches the harder
// case where the surrounding directory tree contains symlinks that, after
// resolution, escape the intended base directory.
package safepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssertUnderBase verifies that target lives within base after symlink
// resolution on both arguments. base must already exist on disk; if it does
// not, an error is returned.
//
// Use this before reading from or writing to a path whose parent directory is
// expected to be present.
func AssertUnderBase(target, base string) error {
	cleanTarget := filepath.Clean(target)
	cleanBase := filepath.Clean(base)
	resolvedBase, err := resolveBase(cleanBase)
	if err != nil {
		return fmt.Errorf("safepath: resolve base %q: %w", cleanBase, err)
	}
	resolvedTarget, err := resolveTarget(cleanTarget)
	if err != nil {
		return fmt.Errorf("safepath: resolve target %q: %w", cleanTarget, err)
	}
	return assertContained(resolvedBase, resolvedTarget)
}

// AssertUnderIntendedBase verifies that target lives within base after
// symlink resolution, allowing the base directory itself to not yet exist
// (only its parent must resolve). Useful before MkdirAll on the claims
// directory or before atomic writes that create the destination on first use.
func AssertUnderIntendedBase(target, base string) error {
	cleanTarget := filepath.Clean(target)
	cleanBase := filepath.Clean(base)
	resolvedBase, err := resolveIntendedBase(cleanBase)
	if err != nil {
		return fmt.Errorf("safepath: resolve base %q: %w", cleanBase, err)
	}
	resolvedTarget, err := resolveTargetAllowMissingAncestors(cleanTarget)
	if err != nil {
		return fmt.Errorf("safepath: resolve target %q: %w", cleanTarget, err)
	}
	return assertContained(resolvedBase, resolvedTarget)
}

func assertContained(resolvedBase, resolvedTarget string) error {
	rel, err := filepath.Rel(resolvedBase, resolvedTarget)
	if err != nil {
		return fmt.Errorf("safepath: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("safepath: target %q escapes base %q", resolvedTarget, resolvedBase)
	}
	return nil
}

func resolveBase(base string) (string, error) {
	// Symlink-resolve the base itself; this is the strict check that requires
	// the directory to already exist.
	if _, err := filepath.EvalSymlinks(base); err != nil {
		return "", err
	}
	return resolveIntendedBase(base)
}

func resolveIntendedBase(base string) (string, error) {
	// Resolve via the missing-ancestor walker so that a base whose parent
	// does not yet exist (e.g. <repo>/.tickets/.claims before the first
	// claim, when even <repo>/.tickets has not been created) is still
	// handled. The walker resolves symlinks on the closest existing
	// ancestor and grafts the remainder on literally.
	return resolveTargetAllowMissingAncestors(base)
}

func resolveTarget(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	// Fallback for not-yet-existing files: resolve the directory, append the
	// literal basename. This is enough to catch escapes via symlinked parents
	// without requiring the file itself to exist.
	parent, parentErr := filepath.EvalSymlinks(filepath.Dir(path))
	if parentErr != nil {
		return "", parentErr
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}

func resolveTargetAllowMissingAncestors(path string) (string, error) {
	cleaned := filepath.Clean(path)
	var missingErr error
	for current := cleaned; ; current = filepath.Dir(current) {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, evalErr := filepath.EvalSymlinks(current)
			if evalErr != nil {
				return "", evalErr
			}
			rel, relErr := filepath.Rel(current, cleaned)
			if relErr != nil {
				return "", relErr
			}
			if rel == "." {
				return resolved, nil
			}
			return filepath.Join(resolved, rel), nil
		}
		if !isNotExist(err) {
			return "", err
		}
		missingErr = err
		parent := filepath.Dir(current)
		if parent == current {
			return "", missingErr
		}
	}
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	return os.IsNotExist(err) ||
		errors.Is(err, os.ErrNotExist) ||
		strings.Contains(strings.ToLower(err.Error()), "no such file or directory")
}
