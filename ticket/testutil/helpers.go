// Package testutil provides shared test helpers for epos test suites.
//
// NewTestStore creates a temporary FileStore rooted in a temp directory that is
// automatically cleaned up when the test finishes. NewTestTicket and
// NewTestTicketWithStatus create Ticket values with sensible defaults for use
// in unit and integration tests across all epos packages.
package testutil

import (
	"path/filepath"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
)

// NewTestStore creates a FileStore rooted in a temp directory. The directory
// is removed automatically when the calling test (or subtest) finishes.
// The returned FileStore is ready for Create/Read/Update/Delete operations.
func NewTestStore(t *testing.T) *store.FileStore {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewTestStore: %v", err)
	}
	return s
}

// NewStoreInDir creates a FileStore rooted at the given directory.
// It is the caller's responsibility to clean up the directory.
// This is useful when a test needs to control the directory path
// (e.g. for CLI integration tests that pass --dir).
func NewStoreInDir(t *testing.T, dir string) *store.FileStore {
	t.Helper()
	s, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewStoreInDir: %v", err)
	}
	return s
}

// NewTestTicket returns a *ticket.Ticket with sensible defaults for testing.
// The ID is generated using the store package's ID generator, the status is
// set to StatusOpen, and the type is set to "task".
func NewTestTicket(title string) *ticket.Ticket {
	tk := ticket.NewTicket(
		ticket.WithTitle(title),
		ticket.WithType("task"),
		ticket.WithStatus(ticket.StatusOpen),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true
	return tk
}

// NewTestTicketWithStatus returns a *ticket.Ticket with the given status.
// It is identical to NewTestTicket except that the status field is set to the
// provided value, making it convenient for tests that need tickets in specific
// lifecycle states (e.g. closed, blocked, claimed).
func NewTestTicketWithStatus(title string, status ticket.Status) *ticket.Ticket {
	tk := ticket.NewTicket(
		ticket.WithTitle(title),
		ticket.WithType("task"),
		ticket.WithStatus(status),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true
	return tk
}

// MustCreateTicket creates a ticket in the given store, failing the test if
// the create operation returns an error. It returns the created ticket for
// immediate use in assertions.
func MustCreateTicket(t *testing.T, s store.Store, tk *ticket.Ticket) *ticket.Ticket {
	t.Helper()
	if err := s.Create(tk); err != nil {
		t.Fatalf("MustCreateTicket %q: %v", tk.ID, err)
	}
	return tk
}

// MustCreateTestTicket creates a new test ticket with the given title and
// stores it in s. It returns the created ticket.
func MustCreateTestTicket(t *testing.T, s store.Store, title string) *ticket.Ticket {
	t.Helper()
	tk := NewTestTicket(title)
	return MustCreateTicket(t, s, tk)
}

// TicketsDir returns the path to the .tickets subdirectory for a given store root.
// Useful for verifying file-system-level state in tests.
func TicketsDir(storeDir string) string {
	return filepath.Join(storeDir, store.TicketsDir)
}
