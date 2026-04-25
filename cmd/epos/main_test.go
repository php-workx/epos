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
// --dir flag. It returns stdout, stderr, and the exit code.
func epos(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	fullArgs := append([]string{"--dir", dir}, args...)
	cmd := exec.Command(binaryPath, fullArgs...)
	out, err := cmd.CombinedOutput()
	stdout = string(out)
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		stderr = string(exitErr.Stderr)
	} else if err != nil {
		// Binary failed to start.
		exitCode = -1
		stderr = err.Error()
	}
	return stdout, stderr, exitCode
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
		s := testutil.NewTestStoreInDir(t, dir)
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

// TestCLIReadyAndBlocked tests the ready and blocked filters with dependencies.
func TestCLIReadyAndBlocked(t *testing.T) {
	dir := t.TempDir()

	s := testutil.NewTestStoreInDir(t, dir)

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
