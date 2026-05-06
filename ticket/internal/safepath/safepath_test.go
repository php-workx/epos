package safepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAssertUnderBaseAcceptsContainedPath confirms the happy path: a file that
// genuinely lives under base passes the check.
func TestAssertUnderBaseAcceptsContainedPath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "child", "file.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := AssertUnderBase(target, base); err != nil {
		t.Fatalf("expected contained path to pass, got %v", err)
	}
}

// TestAssertUnderBaseRejectsLiteralEscape confirms the trivial escape via ".."
// in the target path string is rejected even before symlink resolution.
func TestAssertUnderBaseRejectsLiteralEscape(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.json")
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := AssertUnderBase(target, base); err == nil {
		t.Fatal("expected error when target lives outside base")
	}
}

// TestAssertUnderBaseRejectsSymlinkEscape is the core defense check: the
// claims directory itself is a symlink to an attacker-controlled location.
// The naive filepath.Join string check would accept it; symlink resolution
// must catch it.
func TestAssertUnderBaseRejectsSymlinkEscape(t *testing.T) {
	intendedBase := t.TempDir()
	attackerDir := t.TempDir()

	// Replace the contents of intendedBase with a symlink that resolves into
	// attackerDir. Anything written under intendedBase actually lands in
	// attackerDir.
	claims := filepath.Join(intendedBase, ".claims")
	if err := os.Symlink(attackerDir, claims); err != nil {
		t.Skipf("os.Symlink unsupported: %v", err)
	}

	// Create a file that, by string concatenation, looks like it's under
	// claims but actually lives in attackerDir.
	smuggled := filepath.Join(claims, "evil.json")
	if err := os.WriteFile(smuggled, []byte("malicious"), 0o600); err != nil {
		t.Fatalf("WriteFile via symlink: %v", err)
	}

	if err := AssertUnderBase(smuggled, intendedBase); err == nil {
		t.Fatal("expected symlink-escape detection")
	} else if !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("unexpected error %v", err)
	}
}

// TestAssertUnderIntendedBaseAllowsMissingTarget verifies that a not-yet-existing
// target file under an existing parent is accepted. This covers the
// first-write-of-sidecar case where the JSON file does not exist yet.
func TestAssertUnderIntendedBaseAllowsMissingTarget(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "sidecar.json") // does not exist
	if err := AssertUnderIntendedBase(target, base); err != nil {
		t.Fatalf("expected missing-target to be accepted, got %v", err)
	}
}

// TestAssertUnderIntendedBaseRejectsSymlinkEscapeWithMissingTarget verifies
// that the missing-target relaxation does not weaken the symlink check on
// existing ancestors.
func TestAssertUnderIntendedBaseRejectsSymlinkEscapeWithMissingTarget(t *testing.T) {
	intendedBase := t.TempDir()
	attackerDir := t.TempDir()

	claims := filepath.Join(intendedBase, ".claims")
	if err := os.Symlink(attackerDir, claims); err != nil {
		t.Skipf("os.Symlink unsupported: %v", err)
	}

	// File does not exist yet; safepath must still notice the parent symlink
	// escapes the base.
	missing := filepath.Join(claims, "future.json")
	if err := AssertUnderIntendedBase(missing, intendedBase); err == nil {
		t.Fatal("expected symlink-escape detection on missing target")
	}
}
