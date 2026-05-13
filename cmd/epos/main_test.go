package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/php-workx/epos/ticket/testutil"
)

// binaryPath is set by TestMain when building the epos binary.
var binaryPath string

// findRepoRoot walks up from the current directory to find the go.mod file,
// returning the directory containing go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}

func TestMain(m *testing.M) {
	// Build the epos binary once for all integration tests.
	tmpDir, err := os.MkdirTemp("", "epos-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmpDir, "epos-test")

	// Determine the repo root by looking for go.mod.
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find repo root: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/epos")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build epos binary: %v\n%s", err, out)
		os.Exit(1)
	}

	code := m.Run()

	// Clean up.
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// epos runs the epos CLI binary with the given arguments, using dir as the
// --dir flag. It returns combined stdout/stderr, stderr, and the exit code.
func epos(t *testing.T, dir string, args ...string) (output, stderr string, exitCode int) {
	t.Helper()
	fullArgs := append([]string{"--dir", dir}, args...)
	cmd := exec.Command(binaryPath, fullArgs...)
	combinedOut, err := cmd.CombinedOutput()
	output = string(combinedOut)
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		stderr = ""
	} else if err != nil {
		// Binary failed to start.
		exitCode = -1
		stderr = err.Error()
	}
	return output, stderr, exitCode
}

// --- Integration tests ---

// TestCLINew tests that creating a ticket via the CLI succeeds and returns
// a ticket ID on stdout.
func TestCLINew(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Integration test", "--type", "feature", "--priority", "2")
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)
	if id == "" {
		t.Fatal("expected ticket ID on stdout, got empty string")
	}
	if !strings.HasPrefix(id, "epo-") {
		t.Errorf("expected ID to start with 'epo-', got %q", id)
	}
}

func TestCreateTicketWithGeneratedIDRetriesCollisions(t *testing.T) {
	s := testutil.NewTestStore(t)
	colliding := testutil.NewTestTicket("existing")
	colliding.ID = "epo-collide"
	if err := s.Create(colliding); err != nil {
		t.Fatalf("Create colliding ticket: %v", err)
	}

	ids := []string{"epo-collide", "epo-unique"}
	spec := &newTicketSpec{Title: "new collision test", Type: defaultTicketType}
	tk, err := createTicketWithGeneratedID(s, spec, func(string) string {
		next := ids[0]
		ids = ids[1:]
		return next
	}, 2)
	if err != nil {
		t.Fatalf("createTicketWithGeneratedID: %v", err)
	}
	if tk.ID != "epo-unique" {
		t.Fatalf("created ID = %q, want %q", tk.ID, "epo-unique")
	}
	if _, err := s.Read("epo-unique"); err != nil {
		t.Fatalf("Read created ticket: %v", err)
	}
}

func TestVersionStringIncludesBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := version, gitCommit, buildDate
	t.Cleanup(func() {
		version, gitCommit, buildDate = oldVersion, oldCommit, oldBuildDate
	})

	version = "v1.2.3"
	gitCommit = "abc1234"
	buildDate = "2026-05-08T00:00:00Z"

	got := versionString()
	want := "v1.2.3 (commit abc1234, built 2026-05-08T00:00:00Z)"
	if got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

// TestCLIFullWorkflow exercises the full end-to-end workflow:
//
//	create → ready → blocked → claim → release → close → export
func TestCLIFullWorkflow(t *testing.T) {
	dir := t.TempDir()

	// Step 1: Create a ticket.
	stdout, _, exitCode := epos(t, dir, "new", "Integration test", "--type", "feature", "--priority", "2")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	ticketID := strings.TrimSpace(stdout)

	// Step 2: ready — should list the new ticket (status open, no deps).
	stdout, _, exitCode = epos(t, dir, "ready")
	if exitCode != 0 {
		t.Fatalf("ready: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Step 3: blocked — should list nothing (no deps).
	stdout, _, exitCode = epos(t, dir, "blocked")
	if exitCode != 0 {
		t.Fatalf("blocked: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Step 4: Claim the ticket.
	stdout, _, exitCode = epos(t, dir, "claim", ticketID, "--owner", "agent-1")
	if exitCode != 0 {
		t.Fatalf("claim: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Step 5: Release the ticket.
	stdout, _, exitCode = epos(t, dir, "release", ticketID, "--owner", "agent-1")
	if exitCode != 0 {
		t.Fatalf("release: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Step 6: Close the ticket.
	stdout, _, exitCode = epos(t, dir, "close", ticketID)
	if exitCode != 0 {
		t.Fatalf("close: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Step 7: Export as JSON and verify the closed ticket is present.
	stdout, _, exitCode = epos(t, dir, "export", "--json")
	if exitCode != 0 {
		t.Fatalf("export --json: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var tickets []ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tickets); err != nil {
		t.Fatalf("export --json: failed to parse JSON: %v\noutput: %s", err, stdout)
	}

	found := false
	for _, tk := range tickets {
		if tk.ID == ticketID && tk.Status == ticket.StatusClosed {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("export --json: expected ticket %q with status %q, not found in output", ticketID, ticket.StatusClosed)
	}
}

// TestCLIShow tests that the show command displays a ticket.
func TestCLIShow(t *testing.T) {
	dir := t.TempDir()

	// Create a ticket.
	stdout, _, exitCode := epos(t, dir, "new", "Show test ticket", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	ticketID := strings.TrimSpace(stdout)

	// Show the ticket.
	stdout, _, exitCode = epos(t, dir, "show", ticketID)
	if exitCode != 0 {
		t.Fatalf("show: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "Show test ticket") {
		t.Errorf("show: expected output to contain title, got: %s", stdout)
	}
}

// TestCLIValidate tests that the validate command checks a ticket.
func TestCLIValidate(t *testing.T) {
	dir := t.TempDir()

	// Create a ticket.
	stdout, _, exitCode := epos(t, dir, "new", "Validate test", "--type", "bug")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	ticketID := strings.TrimSpace(stdout)

	// Validate the ticket — should be valid since it was created with required fields.
	stdout, _, exitCode = epos(t, dir, "validate", ticketID)
	if exitCode != 0 {
		t.Fatalf("validate: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "valid") {
		t.Errorf("validate: expected output to contain 'valid', got: %s", stdout)
	}
}

// TestCLILint tests that the lint command checks all tickets.
func TestCLILint(t *testing.T) {
	dir := t.TempDir()

	// Create a ticket.
	stdout, _, exitCode := epos(t, dir, "new", "Lint test", "--type", "chore")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Lint all tickets.
	stdout, _, exitCode = epos(t, dir, "lint")
	if exitCode != 0 {
		t.Fatalf("lint: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "valid") {
		t.Errorf("lint: expected output to contain 'valid', got: %s", stdout)
	}
}

// TestCLIExportJSON tests that export --json outputs valid JSON.
func TestCLIExportJSON(t *testing.T) {
	dir := t.TempDir()

	// Create a ticket.
	stdout, _, exitCode := epos(t, dir, "new", "Export JSON test", "--type", "feature", "--priority", "5")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Export as JSON.
	stdout, _, exitCode = epos(t, dir, "export", "--json")
	if exitCode != 0 {
		t.Fatalf("export --json: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var tickets []ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tickets); err != nil {
		t.Fatalf("export --json: failed to parse: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(tickets))
	}
	if tickets[0].Title != "Export JSON test" {
		t.Errorf("expected title %q, got %q", "Export JSON test", tickets[0].Title)
	}
}

// TestCLINewDefaultType tests that creating a ticket without --type defaults to "task".
func TestCLINewDefaultType(t *testing.T) {
	dir := t.TempDir()

	stdout, _, exitCode := epos(t, dir, "new", "Default type test")
	if exitCode != 0 {
		t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	ticketID := strings.TrimSpace(stdout)

	// Show the ticket and verify it has type "task".
	stdout, _, exitCode = epos(t, dir, "show", ticketID, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if tk.Type != "task" {
		t.Errorf("expected type 'task', got %q", tk.Type)
	}
}

// TestCustomErrorTypes verifies that the CLI maps custom error types to the
// correct exit codes:
//   - TicketNotFoundError → exit code 3
//   - AmbiguousIDError → exit code 4
//   - ValidationError → exit code 2
func TestCustomErrorTypes(t *testing.T) {
	t.Run("TicketNotFoundError_exit3", func(t *testing.T) {
		dir := t.TempDir()

		// Attempting to show a nonexistent ticket should produce TicketNotFoundError,
		// which the CLI maps to exit code 3.
		_, _, exitCode := epos(t, dir, "show", "nonexistent-id")
		if exitCode != 3 {
			t.Errorf("expected exit code 3 for TicketNotFoundError, got %d", exitCode)
		}
	})

	t.Run("ValidationError_exit2", func(t *testing.T) {
		dir := t.TempDir()

		// Claiming without --owner should trigger a ValidationError (field "owner" is required),
		// which the CLI maps to exit code 2.
		// First, create a ticket so the ID exists.
		stdout, _, exitCode := epos(t, dir, "new", "Validation test ticket")
		if exitCode != 0 {
			t.Fatalf("new: expected exit code 0, got %d; output: %s", exitCode, stdout)
		}
		ticketID := strings.TrimSpace(stdout)

		// Try to claim without --owner.
		_, _, exitCode = epos(t, dir, "claim", ticketID)
		if exitCode != 2 {
			t.Errorf("expected exit code 2 for ValidationError, got %d", exitCode)
		}
	})

	t.Run("AmbiguousIDError_exit4", func(t *testing.T) {
		dir := t.TempDir()

		// Create two tickets with IDs that share a common prefix.
		s := testutil.NewStoreInDir(t, dir)
		tk1 := testutil.NewTestTicket("First ambiguous ticket")
		tk2 := testutil.NewTestTicket("Second ambiguous ticket")
		testutil.MustCreateTicket(t, s, tk1)
		testutil.MustCreateTicket(t, s, tk2)

		// Use a prefix that matches both tickets — the "epo" prefix will match all
		// tickets starting with "epo-", causing an AmbiguousIDError → exit code 4.
		_, _, exitCode := epos(t, dir, "show", "epo-")
		if exitCode != 4 {
			t.Errorf("expected exit code 4 for AmbiguousIDError, got %d", exitCode)
		}
	})
}

// TestCLIReadyExcludesActivelyClaimedTickets verifies the user-facing
// behavior of the sidecar-aware ready filter: an actively claimed ticket
// disappears from "epos ready" output but reappears with --include-claimed.
func TestCLIReadyExcludesActivelyClaimedTickets(t *testing.T) {
	dir := t.TempDir()

	stdout, _, exitCode := epos(t, dir, "new", "Will be claimed", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	if _, _, exitCode = epos(t, dir, "claim", id, "--owner", "agent-1"); exitCode != 0 {
		t.Fatalf("claim: exit %d", exitCode)
	}

	// Default behaviour: claimed ticket must not appear in ready output.
	stdout, _, exitCode = epos(t, dir, "ready")
	if exitCode != 0 {
		t.Fatalf("ready: exit %d: %s", exitCode, stdout)
	}
	if strings.Contains(stdout, id) {
		t.Errorf("ready: claimed ticket %q leaked into output:\n%s", id, stdout)
	}

	// Escape hatch: --include-claimed surfaces it again for debugging.
	stdout, _, exitCode = epos(t, dir, "ready", "--include-claimed")
	if exitCode != 0 {
		t.Fatalf("ready --include-claimed: exit %d: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, id) {
		t.Errorf("ready --include-claimed: expected claimed ticket %q in output:\n%s", id, stdout)
	}
}

// TestCLIReadyAndBlocked tests the ready and blocked filters with dependencies.
func TestCLIReadyAndBlocked(t *testing.T) {
	dir := t.TempDir()

	s := testutil.NewStoreInDir(t, dir)

	// Create a parent ticket.
	parent := testutil.NewTestTicket("Parent ticket")
	parent.Type = "epic"
	parent = testutil.MustCreateTicket(t, s, parent)

	// Create a child ticket with the parent as a dependency.
	child := testutil.NewTestTicket("Child ticket")
	child.Parent = parent.ID
	child.Deps = []string{parent.ID}
	child.Status = ticket.StatusPending
	child.Present["deps"] = true
	child.Present["parent"] = true
	child.Present["status"] = true
	testutil.MustCreateTicket(t, s, child)

	// The child should appear in "blocked" (parent is open, not closed).
	stdout, _, exitCode := epos(t, dir, "blocked")
	if exitCode != 0 {
		t.Fatalf("blocked: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, child.ID) {
		t.Errorf("blocked: expected child ticket %q in output, got: %s", child.ID, stdout)
	}

	// Close the parent to unblock the child.
	stdout, _, exitCode = epos(t, dir, "close", parent.ID)
	if exitCode != 0 {
		t.Fatalf("close parent: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}

	// Now the child should be in "ready" (parent is closed, all deps satisfied).
	// Note: the child has status "pending" which is a ready-eligible status.
	stdout, _, exitCode = epos(t, dir, "ready")
	if exitCode != 0 {
		t.Fatalf("ready: expected exit code 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, child.ID) {
		t.Errorf("ready: expected child ticket %q in output, got: %s", child.ID, stdout)
	}
}

// --- Unit-level tests for exitCode ---

func TestExitCodeNil(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Errorf("exitCode(nil) = %d, want 0", got)
	}
}

func TestExitCodeTicketNotFound(t *testing.T) {
	err := &ticket.TicketNotFoundError{ID: "xyz"}
	if got := exitCode(err); got != 3 {
		t.Errorf("exitCode(TicketNotFoundError) = %d, want 3", got)
	}
}

func TestExitCodeAmbiguousID(t *testing.T) {
	err := &ticket.AmbiguousIDError{Partial: "epo", Matches: []string{"epo-a", "epo-b"}}
	if got := exitCode(err); got != 4 {
		t.Errorf("exitCode(AmbiguousIDError) = %d, want 4", got)
	}
}

func TestExitCodeValidation(t *testing.T) {
	err := &ticket.ValidationError{Field: "owner", Message: "required"}
	if got := exitCode(err); got != 2 {
		t.Errorf("exitCode(ValidationError) = %d, want 2", got)
	}
}

func TestExitCodeGeneric(t *testing.T) {
	err := fmt.Errorf("some generic error")
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode(generic) = %d, want 1", got)
	}
}

func TestExitCodeCycleDetected(t *testing.T) {
	err := &ticket.CycleDetectedError{Cycle: []string{"a", "b", "a"}}
	if got := exitCode(err); got != 5 {
		t.Errorf("exitCode(CycleDetectedError) = %d, want 5", got)
	}
}

func TestExitCodeAlreadyClaimed(t *testing.T) {
	err := &ticket.AlreadyClaimedError{TicketID: "t-1", ClaimedBy: "agent-x"}
	if got := exitCode(err); got != 6 {
		t.Errorf("exitCode(AlreadyClaimedError) = %d, want 6", got)
	}
}

func TestExitCodeNotClaimed(t *testing.T) {
	err := &ticket.NotClaimedError{TicketID: "t-1"}
	if got := exitCode(err); got != 6 {
		t.Errorf("exitCode(NotClaimedError) = %d, want 6", got)
	}
}

func TestExitCodeNotClaimOwner(t *testing.T) {
	err := &ticket.NotClaimOwnerError{TicketID: "t-1", ClaimedBy: "agent-x", Caller: "agent-y"}
	if got := exitCode(err); got != 6 {
		t.Errorf("exitCode(NotClaimOwnerError) = %d, want 6", got)
	}
}

func TestExitCodeWrapped(t *testing.T) {
	inner := &ticket.TicketNotFoundError{ID: "xyz"}
	err := fmt.Errorf("wrapped: %w", inner)
	if got := exitCode(err); got != 3 {
		t.Errorf("exitCode(wrapped TicketNotFoundError) = %d, want 3", got)
	}
}

func TestExitCodeWrappedClaim(t *testing.T) {
	inner := &ticket.AlreadyClaimedError{TicketID: "t-1", ClaimedBy: "agent-x"}
	err := fmt.Errorf("wrapped: %w", inner)
	if got := exitCode(err); got != 6 {
		t.Errorf("exitCode(wrapped AlreadyClaimedError) = %d, want 6", got)
	}
}

// --- Verify testutil helpers are usable from cmd/epos tests ---

// TestTestutilNewTestStore verifies that the testutil helpers work correctly.
func TestTestutilNewTestStore(t *testing.T) {
	s := testutil.NewTestStore(t)
	if s == nil {
		t.Fatal("NewTestStore returned nil")
	}
	if s.Dir == "" {
		t.Fatal("NewTestStore FileStore has empty Dir")
	}
	// Verify the .tickets directory was created.
	ticketsDir := filepath.Join(s.Dir, store.TicketsDir)
	if _, err := os.Stat(ticketsDir); os.IsNotExist(err) {
		t.Errorf("NewTestStore did not create .tickets directory at %s", ticketsDir)
	}
}

func TestTestutilNewTestTicket(t *testing.T) {
	tk := testutil.NewTestTicket("test ticket")
	if tk.Title != "test ticket" {
		t.Errorf("NewTestTicket title = %q, want %q", tk.Title, "test ticket")
	}
	if tk.Type != "task" {
		t.Errorf("NewTestTicket type = %q, want %q", tk.Type, "task")
	}
	if tk.ID == "" {
		t.Error("NewTestTicket ID must not be empty")
	}
}

func TestTestutilNewTestTicketWithStatus(t *testing.T) {
	tk := testutil.NewTestTicketWithStatus("closed ticket", ticket.StatusClosed)
	if tk.Status != ticket.StatusClosed {
		t.Errorf("NewTestTicketWithStatus status = %q, want %q", tk.Status, ticket.StatusClosed)
	}
	if tk.Title != "closed ticket" {
		t.Errorf("NewTestTicketWithStatus title = %q, want %q", tk.Title, "closed ticket")
	}
}

// ─── epos new: rich content flags ─────────────────────────────────────────────

func TestCLINewWithBody(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Body ticket",
		"--body", "This is the narrative.",
		"--type", "task",
	)
	if exitCode != 0 {
		t.Fatalf("new --body: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "This is the narrative." {
		t.Errorf("Description: got %q, want %q", tk.Description, "This is the narrative.")
	}
}

func TestCLINewWithAcceptanceCriteria(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "AC ticket",
		"--ac", "first criterion",
		"--ac", "second criterion",
	)
	if exitCode != 0 {
		t.Fatalf("new --ac: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(tk.AcceptanceCriteria) != 2 {
		t.Fatalf("AcceptanceCriteria: got %d items, want 2", len(tk.AcceptanceCriteria))
	}
	if tk.AcceptanceCriteria[0] != "first criterion" {
		t.Errorf("AcceptanceCriteria[0]: got %q, want %q", tk.AcceptanceCriteria[0], "first criterion")
	}
}

func TestCLINewWithNotes(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Notes ticket",
		"--note", "initial note",
	)
	if exitCode != 0 {
		t.Fatalf("new --note: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(tk.Notes) != 1 || tk.Notes[0] != "initial note" {
		t.Errorf("Notes: got %v, want [initial note]", tk.Notes)
	}
}

func TestCLINewWithAllRichFlags(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Rich ticket",
		"--type", "feature",
		"--priority", "5",
		"--body", "narrative text",
		"--ac", "criterion one",
		"--ac", "criterion two",
		"--note", "note alpha",
		"--assignee", "dev-1",
		"--tags", "backend,api",
		"--intent", "improve throughput",
	)
	if exitCode != 0 {
		t.Fatalf("new all flags: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "narrative text" {
		t.Errorf("Description: got %q", tk.Description)
	}
	if len(tk.AcceptanceCriteria) != 2 {
		t.Errorf("AcceptanceCriteria count: got %d", len(tk.AcceptanceCriteria))
	}
	if tk.Assignee != "dev-1" {
		t.Errorf("Assignee: got %q", tk.Assignee)
	}
	if tk.Intent != "improve throughput" {
		t.Errorf("Intent: got %q", tk.Intent)
	}
	if len(tk.Tags) != 2 {
		t.Errorf("Tags count: got %d", len(tk.Tags))
	}
}

func TestCLINewBodyRenderedInFile(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "File body test",
		"--body", "my description",
		"--ac", "check one",
		"--note", "note text",
	)
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	data, err := os.ReadFile(filepath.Join(dir, ".tickets", id+".md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "my description") {
		t.Errorf("ticket file missing description body paragraph:\n%s", body)
	}
	if !strings.Contains(body, "## Acceptance criteria") {
		t.Errorf("ticket file missing ## Acceptance criteria section:\n%s", body)
	}
	if !strings.Contains(body, "## Notes") {
		t.Errorf("ticket file missing ## Notes section:\n%s", body)
	}
}

// ─── epos new: JSON stdin ─────────────────────────────────────────────────────

func eposStdin(t *testing.T, dir, input string, args ...string) (stdout string, exitCode int) {
	t.Helper()
	fullArgs := append([]string{"--dir", dir}, args...)
	cmd := exec.Command(binaryPath, fullArgs...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	stdout = string(out)
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}
	return stdout, exitCode
}

func TestCLINewStdin(t *testing.T) {
	dir := t.TempDir()
	input := `{
		"type": "task",
		"body": "from stdin",
		"acceptance_criteria": ["ac from stdin"]
	}`
	stdout, exitCode := eposStdin(t, dir, input, "new", "Stdin ticket", "--stdin")
	if exitCode != 0 {
		t.Fatalf("new --stdin: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout2, _, exitCode2 := epos(t, dir, "show", id, "--json")
	if exitCode2 != 0 {
		t.Fatalf("show --json: exit %d: %s", exitCode2, stdout2)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout2), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "from stdin" {
		t.Errorf("Description from stdin: got %q", tk.Description)
	}
	if len(tk.AcceptanceCriteria) != 1 || tk.AcceptanceCriteria[0] != "ac from stdin" {
		t.Errorf("AcceptanceCriteria from stdin: got %v", tk.AcceptanceCriteria)
	}
}

func TestCLINewStdinFlagsOverride(t *testing.T) {
	dir := t.TempDir()
	input := `{"type": "epic", "body": "from stdin body"}`
	// --type flag should override JSON type; --body flag should override JSON body
	stdout, exitCode := eposStdin(t, dir, input,
		"new", "Override test", "--stdin",
		"--type", "task",
		"--body", "flag body wins",
	)
	if exitCode != 0 {
		t.Fatalf("new --stdin with overrides: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout2, _, _ := epos(t, dir, "show", id, "--json")
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout2), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Type != "task" {
		t.Errorf("Type: flag should override stdin, got %q", tk.Type)
	}
	if tk.Description != "flag body wins" {
		t.Errorf("Description: flag should override stdin, got %q", tk.Description)
	}
}

// TestCLINewStdinUsesBodyKey pins the contract that new --stdin accepts "body"
// (not "description") for the description field. This diverges from edit --stdin
// which uses "description" — see docs/skill-contract.md.
func TestCLINewStdinUsesBodyKey(t *testing.T) {
	dir := t.TempDir()
	input := `{"title":"stdin ticket","type":"task","body":"body from stdin"}`
	stdout, exitCode := eposStdin(t, dir, input, "new", "stdin ticket", "--stdin")
	if exitCode != 0 {
		t.Fatalf("new --stdin: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	showOut, _, exitCode2 := epos(t, dir, "show", id, "--json")
	if exitCode2 != 0 {
		t.Fatalf("show: exit %d: %s", exitCode2, showOut)
	}
	var tk map[string]any
	if err := json.Unmarshal([]byte(showOut), &tk); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if tk["description"] != "body from stdin" {
		t.Errorf("description: got %v, want %q", tk["description"], "body from stdin")
	}
}

func TestCLINewStdinInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	stdout, exitCode := eposStdin(t, dir, "not valid json", "new", "Bad stdin", "--stdin")
	if exitCode == 0 {
		t.Fatalf("new --stdin invalid JSON: expected non-zero exit, got 0; output: %s", stdout)
	}
}

// ─── epos new: validation ─────────────────────────────────────────────────────

func TestCLINewInvalidType(t *testing.T) {
	dir := t.TempDir()
	_, _, exitCode := epos(t, dir, "new", "Bad type", "--type", "invalid-type")
	if exitCode != 2 {
		t.Errorf("new --type invalid: expected exit 2 (ValidationError), got %d", exitCode)
	}
}

func TestCLINewEmptyACItem(t *testing.T) {
	dir := t.TempDir()
	_, _, exitCode := epos(t, dir, "new", "Empty AC", "--ac", "")
	if exitCode != 2 {
		t.Errorf("new --ac empty: expected exit 2 (ValidationError), got %d", exitCode)
	}
}

func TestCLINewDuplicateTags(t *testing.T) {
	dir := t.TempDir()
	_, _, exitCode := epos(t, dir, "new", "Dup tags", "--tags", "foo,foo")
	if exitCode != 2 {
		t.Errorf("new --tags duplicate: expected exit 2 (ValidationError), got %d", exitCode)
	}
}

func TestCLINewStdinInvalidType(t *testing.T) {
	dir := t.TempDir()
	input := `{"type": "not-a-type"}`
	stdout, exitCode := eposStdin(t, dir, input, "new", "Stdin bad type", "--stdin")
	if exitCode != 2 {
		t.Errorf("new --stdin invalid type: expected exit 2, got %d; output: %s", exitCode, stdout)
	}
}

func TestCLINewStdinEmptyAC(t *testing.T) {
	dir := t.TempDir()
	input := `{"acceptance_criteria": ["valid", ""]}`
	stdout, exitCode := eposStdin(t, dir, input, "new", "Stdin empty AC", "--stdin")
	if exitCode != 2 {
		t.Errorf("new --stdin empty AC item: expected exit 2, got %d; output: %s", exitCode, stdout)
	}
}

func TestCLINewBodyFile(t *testing.T) {
	dir := t.TempDir()
	bodyFile := filepath.Join(dir, "body.txt")
	if err := os.WriteFile(bodyFile, []byte("from a file"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	stdout, _, exitCode := epos(t, dir, "new", "Body file ticket", "--body-file", bodyFile)
	if exitCode != 0 {
		t.Fatalf("new --body-file: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, _ = epos(t, dir, "show", id, "--json")
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "from a file" {
		t.Errorf("Description from --body-file: got %q", tk.Description)
	}
}

// ─── epos edit ────────────────────────────────────────────────────────────────

func TestCLIEdit(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit me", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	_, _, exitCode = epos(t, dir, "edit", id,
		"--body", "updated description",
		"--ac", "new criterion",
	)
	if exitCode != 0 {
		t.Fatalf("edit: exit %d", exitCode)
	}

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json after edit: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "updated description" {
		t.Errorf("Description after edit: got %q", tk.Description)
	}
	if len(tk.AcceptanceCriteria) != 1 || tk.AcceptanceCriteria[0] != "new criterion" {
		t.Errorf("AcceptanceCriteria after edit: got %v", tk.AcceptanceCriteria)
	}
}

func TestCLIEditPreservesBody(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Body preserved",
		"--body", "original description",
		"--ac", "original AC",
	)
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	// Edit only priority — body sections should be untouched.
	_, _, exitCode = epos(t, dir, "edit", id, "--priority", "7")
	if exitCode != 0 {
		t.Fatalf("edit --priority: exit %d", exitCode)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".tickets", id+".md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "original description") {
		t.Errorf("edit dropped description from body:\n%s", body)
	}
	if !strings.Contains(body, "## Acceptance criteria") {
		t.Errorf("edit dropped ## Acceptance criteria from body:\n%s", body)
	}
}

func TestCLIEditStdin(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit via stdin", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	input := `{"description": "stdin body", "acceptance_criteria": ["stdin ac"]}`
	stdout2, exitCode2 := eposStdin(t, dir, input, "edit", id, "--stdin")
	if exitCode2 != 0 {
		t.Fatalf("edit --stdin: exit %d: %s", exitCode2, stdout2)
	}

	stdout3, _, _ := epos(t, dir, "show", id, "--json")
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout3), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "stdin body" {
		t.Errorf("edit --stdin Description: got %q", tk.Description)
	}
}

func TestCLIEditStdinBodyKeyIsIgnored(t *testing.T) {
	// edit --stdin uses "description", not "body". Sending "body" must be a no-op.
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "body key test", "--type", "task", "--body", "original description")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	input := `{"body": "should be ignored"}`
	stdout2, exitCode2 := eposStdin(t, dir, input, "edit", id, "--stdin")
	if exitCode2 != 0 {
		t.Fatalf("edit --stdin: exit %d: %s", exitCode2, stdout2)
	}

	stdout3, _, _ := epos(t, dir, "show", id, "--json")
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout3), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "original description" {
		t.Errorf("edit --stdin with 'body' key mutated description: got %q, want %q", tk.Description, "original description")
	}
}

func TestCLIEditStdinCanClearFields(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Clear via stdin",
		"--priority", "5",
		"--body", "body to clear",
		"--ac", "criterion to clear",
		"--assignee", "alice",
		"--tags", "one,two",
		"--intent", "intent to clear",
	)
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	input := `{"priority": 0, "description": "", "acceptance_criteria": [], "assignee": "", "tags": [], "intent": ""}`
	stdout, exitCode = eposStdin(t, dir, input, "edit", id, "--stdin")
	if exitCode != 0 {
		t.Fatalf("edit --stdin clear fields: exit %d: %s", exitCode, stdout)
	}

	stdout, _, exitCode = epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("show --json after clear: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Priority != 0 {
		t.Errorf("Priority after clear edit: got %d, want 0", tk.Priority)
	}
	if tk.Description != "" || len(tk.AcceptanceCriteria) != 0 || tk.Assignee != "" || len(tk.Tags) != 0 || tk.Intent != "" {
		t.Errorf("edit --stdin did not clear fields: %+v", tk)
	}
}

func TestCLIEditNotFound(t *testing.T) {
	dir := t.TempDir()
	_, _, exitCode := epos(t, dir, "edit", "nonexistent-id", "--priority", "1")
	if exitCode != 3 {
		t.Errorf("edit nonexistent: expected exit 3 (TicketNotFoundError), got %d", exitCode)
	}
}

func TestCLIEditInvalidPriority(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit validation")
	if exitCode != 0 {
		t.Fatalf("new: exit %d", exitCode)
	}
	id := strings.TrimSpace(stdout)

	input := `{"priority": -1}`
	stdout2, exitCode2 := eposStdin(t, dir, input, "edit", id, "--stdin")
	if exitCode2 != 2 {
		t.Errorf("edit --stdin negative priority: expected exit 2, got %d; output: %s", exitCode2, stdout2)
	}
}

func TestCLIEditDuplicateTags(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Tag dup test")
	if exitCode != 0 {
		t.Fatalf("new: exit %d", exitCode)
	}
	id := strings.TrimSpace(stdout)

	_, _, exitCode = epos(t, dir, "edit", id, "--tags", "foo,foo")
	if exitCode != 2 {
		t.Errorf("edit --tags duplicate: expected exit 2, got %d", exitCode)
	}
}

func TestCLIEditStdinInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit stdin bad JSON", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	out, code := eposStdin(t, dir, "not valid json", "edit", id, "--stdin")
	if code == 0 {
		t.Errorf("edit --stdin invalid JSON: expected non-zero exit, got 0; output: %s", out)
	}
}

func TestCLIEditBodyFile(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit body file", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	bodyFile := filepath.Join(dir, "body.txt")
	if err := os.WriteFile(bodyFile, []byte("from body file"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}

	out, _, code := epos(t, dir, "edit", id, "--body-file", bodyFile)
	if code != 0 {
		t.Fatalf("edit --body-file: exit %d: %s", code, out)
	}

	stdout, _, _ = epos(t, dir, "show", id, "--json")
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Description != "from body file" {
		t.Errorf("Description after edit --body-file: got %q, want %q", tk.Description, "from body file")
	}
}

func TestCLIEditBodyFileNotFound(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Edit body file missing", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	out, _, code := epos(t, dir, "edit", id, "--body-file", filepath.Join(dir, "nonexistent.txt"))
	if code == 0 {
		t.Errorf("edit --body-file nonexistent: expected non-zero exit, got 0; output: %s", out)
	}
}

// ─── epos validate / lint: non-zero exit on errors ───────────────────────────

func TestCLIValidateExitsNonZeroOnInvalidTicket(t *testing.T) {
	dir := t.TempDir()
	s := testutil.NewStoreInDir(t, dir)

	// Create a ticket with an invalid type via the store (bypasses CLI validation).
	tk := testutil.NewTestTicket("Invalid type ticket")
	tk.Type = "not-a-valid-type"
	testutil.MustCreateTicket(t, s, tk)

	stdout, _, exitCode := epos(t, dir, "validate", tk.ID)
	if exitCode != 1 {
		t.Errorf("validate on invalid ticket: expected exit 1, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "error:") {
		t.Errorf("validate: expected error lines in output, got: %s", stdout)
	}
}

func TestCLIValidateExitsZeroOnValidTicket(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Valid ticket", "--type", "task")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "validate", id)
	if exitCode != 0 {
		t.Errorf("validate on valid ticket: expected exit 0, got %d; output: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "valid") {
		t.Errorf("validate: expected 'valid' in output, got: %s", stdout)
	}
}

func TestCLILintExitsNonZeroOnInvalidTicket(t *testing.T) {
	dir := t.TempDir()
	s := testutil.NewStoreInDir(t, dir)

	tk := testutil.NewTestTicket("Invalid type for lint")
	tk.Type = "not-a-valid-type"
	testutil.MustCreateTicket(t, s, tk)

	stdout, _, exitCode := epos(t, dir, "lint")
	if exitCode != 1 {
		t.Errorf("lint with invalid ticket: expected exit 1, got %d; output: %s", exitCode, stdout)
	}
}

func TestCLILintJSONOmitsValidTickets(t *testing.T) {
	dir := t.TempDir()
	s := testutil.NewStoreInDir(t, dir)

	valid := testutil.MustCreateTicket(t, s, testutil.NewTestTicket("Valid lint ticket"))
	invalid := testutil.NewTestTicket("Invalid lint ticket")
	invalid.Type = "not-a-valid-type"
	invalid = testutil.MustCreateTicket(t, s, invalid)

	stdout, _, exitCode := epos(t, dir, "lint", "--json")
	if exitCode != 0 {
		t.Fatalf("lint --json: exit %d; output: %s", exitCode, stdout)
	}

	var result struct {
		Tickets []struct {
			ID string `json:"id"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse lint JSON: %v\n%s", err, stdout)
	}
	if len(result.Tickets) != 1 || result.Tickets[0].ID != invalid.ID {
		t.Fatalf("lint --json tickets = %+v, want only %s", result.Tickets, invalid.ID)
	}
	for _, entry := range result.Tickets {
		if entry.ID == valid.ID {
			t.Fatalf("lint --json included valid ticket %s", valid.ID)
		}
	}
}

func TestCLILintExitsNonZeroOnCycle(t *testing.T) {
	dir := t.TempDir()
	s := testutil.NewStoreInDir(t, dir)

	// Two tickets with a mutual dep cycle: A → B, B → A.
	tk1 := testutil.NewTestTicket("Cycle ticket A")
	tk2 := testutil.NewTestTicket("Cycle ticket B")
	tk1.Deps = []string{tk2.ID}
	tk1.Present["deps"] = true
	tk2.Deps = []string{tk1.ID}
	tk2.Present["deps"] = true
	testutil.MustCreateTicket(t, s, tk1)
	testutil.MustCreateTicket(t, s, tk2)

	stdout, _, exitCode := epos(t, dir, "lint")
	if exitCode != 1 {
		t.Errorf("lint with dep cycle: expected exit 1, got %d; output: %s", exitCode, stdout)
	}
}

// ─── epos show: rich content fields ──────────────────────────────────────────

func TestCLIShowDisplaysDescription(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Desc show test", "--body", "body text here")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id)
	if exitCode != 0 {
		t.Fatalf("show: exit %d: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "body text here") {
		t.Errorf("show: expected description in output, got:\n%s", stdout)
	}
}

func TestCLIShowDisplaysAcceptanceCriteria(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "AC show test", "--ac", "my criterion")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id)
	if exitCode != 0 {
		t.Fatalf("show: exit %d: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "my criterion") {
		t.Errorf("show: expected AC in output, got:\n%s", stdout)
	}
}

func TestCLIShowDisplaysNotes(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Notes show test", "--note", "my note text")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id)
	if exitCode != 0 {
		t.Fatalf("show: exit %d: %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, "my note text") {
		t.Errorf("show: expected note in output, got:\n%s", stdout)
	}
}

func TestCLIShowOmitsEmptyContentFields(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "No content ticket")
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	stdout, _, exitCode = epos(t, dir, "show", id)
	if exitCode != 0 {
		t.Fatalf("show: exit %d: %s", exitCode, stdout)
	}
	if strings.Contains(stdout, "Description:") {
		t.Errorf("show: empty Description should not appear in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "Acceptance criteria:") {
		t.Errorf("show: empty AC should not appear in output, got:\n%s", stdout)
	}
}

// ─── cross-tool round-trip ────────────────────────────────────────────────────

func TestCLIRoundTripEposToTk(t *testing.T) {
	if _, err := exec.LookPath("tk"); err != nil {
		t.Skip("tk not in PATH")
	}
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Round trip ticket",
		"--body", "round trip body",
		"--ac", "round trip criterion",
	)
	if exitCode != 0 {
		t.Fatalf("epos new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	cmd := exec.Command("tk", "show", id)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tk show: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "round trip body") {
		t.Errorf("tk show: expected body in output, got:\n%s", output)
	}
	if !strings.Contains(output, "round trip criterion") {
		t.Errorf("tk show: expected AC in output, got:\n%s", output)
	}
}

func TestCLIRoundTripTkToEpos(t *testing.T) {
	if _, err := exec.LookPath("tk"); err != nil {
		t.Skip("tk not in PATH")
	}
	dir := t.TempDir()

	cmd := exec.Command("tk", "create", "TK created ticket")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tk create: %v\n%s", err, out)
	}
	id := strings.TrimSpace(string(out))

	stdout, _, exitCode := epos(t, dir, "show", id, "--json")
	if exitCode != 0 {
		t.Fatalf("epos show --json: exit %d: %s", exitCode, stdout)
	}
	var tk ticket.Ticket
	if err := json.Unmarshal([]byte(stdout), &tk); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if tk.Title != "TK created ticket" {
		t.Errorf("round trip title: got %q, want %q", tk.Title, "TK created ticket")
	}
}

// ─── close preserves body ─────────────────────────────────────────────────────

func TestCLIClosePreservesBody(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exitCode := epos(t, dir, "new", "Close preserve",
		"--body", "must survive close",
		"--ac", "check one",
	)
	if exitCode != 0 {
		t.Fatalf("new: exit %d: %s", exitCode, stdout)
	}
	id := strings.TrimSpace(stdout)

	_, _, exitCode = epos(t, dir, "close", id)
	if exitCode != 0 {
		t.Fatalf("close: exit %d", exitCode)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".tickets", id+".md"))
	if err != nil {
		t.Fatalf("ReadFile after close: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "must survive close") {
		t.Errorf("close dropped description from body:\n%s", body)
	}
	if !strings.Contains(body, "## Acceptance criteria") {
		t.Errorf("close dropped ## Acceptance criteria from body:\n%s", body)
	}
}
