package main

import (
	"strings"
	"testing"
)

func TestCLITUIHelpIncludesFlags(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "tui", "--help")
	if exitCode != 0 {
		t.Fatalf("tui --help exit = %d; output: %s", exitCode, stdout)
	}
	for _, want := range []string{"Open interactive ticket TUI", "--owner", "--refresh"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("help output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCLITUIRejectsJSON(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "--json", "tui")
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit; output: %s", stdout)
	}
	if !strings.Contains(stdout, "tui is interactive and does not support --json") {
		t.Fatalf("unexpected output: %s", stdout)
	}
}

func TestResolveTUIOwnerPrecedence(t *testing.T) {
	t.Setenv("EPOS_OWNER", "env-owner")
	t.Setenv("USER", "user-owner")
	t.Setenv("USERNAME", "username-owner")

	if got := resolveTUIOwner("flag-owner"); got != "flag-owner" {
		t.Fatalf("resolveTUIOwner flag = %q, want flag-owner", got)
	}
	if got := resolveTUIOwner(""); got != "env-owner" {
		t.Fatalf("resolveTUIOwner EPOS_OWNER = %q, want env-owner", got)
	}
}

func TestResolveTUIOwnerFallsBackToLoginEnv(t *testing.T) {
	t.Setenv("EPOS_OWNER", "")
	t.Setenv("USER", "user-owner")
	t.Setenv("USERNAME", "username-owner")

	if got := resolveTUIOwner(""); got != "user-owner" {
		t.Fatalf("resolveTUIOwner USER = %q, want user-owner", got)
	}

	t.Setenv("USER", "")
	if got := resolveTUIOwner(""); got != "username-owner" {
		t.Fatalf("resolveTUIOwner USERNAME = %q, want username-owner", got)
	}

	t.Setenv("USERNAME", "")
	if got := resolveTUIOwner(""); got != "" {
		t.Fatalf("resolveTUIOwner empty = %q, want empty", got)
	}
}
