package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/php-workx/epos/ticket/testutil"
)

// ─── Create ──────────────────────────────────────────────────────────────────

func TestCreateWritesFile(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("write me")
	if err := s.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}
	path := filepath.Join(s.Dir, store.TicketsDir, tk.ID+".md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected ticket file at %s, got error: %v", path, err)
	}
}

func TestCreateRendersBodySections(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := ticket.NewTicket(
		ticket.WithTitle("Body ticket"),
		ticket.WithType("task"),
		ticket.WithDescription("The narrative."),
		ticket.WithAcceptanceCriteria("first AC", "second AC"),
		ticket.WithNotes("a note"),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true

	if err := s.Create(tk); err != nil {
		t.Fatalf("Create: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.Dir, store.TicketsDir, tk.ID+".md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	body := string(data)

	if !strings.Contains(body, "The narrative.") {
		t.Errorf("body should contain description text:\n%s", body)
	}
	if !strings.Contains(body, "## Acceptance criteria") {
		t.Errorf("body should contain ## Acceptance criteria:\n%s", body)
	}
	if !strings.Contains(body, "- first AC") {
		t.Errorf("body should contain first AC:\n%s", body)
	}
	if !strings.Contains(body, "## Notes") {
		t.Errorf("body should contain ## Notes:\n%s", body)
	}
	if !strings.Contains(body, "- a note") {
		t.Errorf("body should contain note text:\n%s", body)
	}
}

func TestCreateRejectsCollision(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("first")
	testutil.MustCreateTicket(t, s, tk)

	err := s.Create(tk)
	if err == nil {
		t.Fatal("Create: expected IDCollisionError, got nil")
	}
	if _, ok := err.(*ticket.IDCollisionError); !ok {
		t.Errorf("Create: expected *IDCollisionError, got %T: %v", err, err)
	}
}

func TestCreateRejectsNilTicket(t *testing.T) {
	s := testutil.NewTestStore(t)
	requireValidationError(t, s.Create(nil))
}

func TestCreateRejectsEmptyID(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := ticket.NewTicket(ticket.WithTitle("no id"))
	err := s.Create(tk)
	if err == nil {
		t.Fatal("Create: expected ValidationError for empty ID")
	}
}

func TestCRUDRejectsInvalidPathIDs(t *testing.T) {
	s := testutil.NewTestStore(t)
	invalidIDs := []string{"../epo-real", "foo/bar", "foo\\bar"}

	for _, id := range invalidIDs {
		t.Run("Create "+id, func(t *testing.T) {
			tk := testutil.NewTestTicket("invalid create")
			tk.ID = id
			requireValidationError(t, s.Create(tk))
		})

		t.Run("Read "+id, func(t *testing.T) {
			_, err := s.Read(id)
			requireValidationError(t, err)
		})

		t.Run("Update "+id, func(t *testing.T) {
			tk := testutil.NewTestTicket("invalid update")
			tk.ID = id
			requireValidationError(t, s.Update(tk))
		})

		t.Run("Delete "+id, func(t *testing.T) {
			requireValidationError(t, s.Delete(id))
		})
	}
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func TestReadRoundTrip(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := ticket.NewTicket(
		ticket.WithTitle("Round-trip"),
		ticket.WithType("bug"),
		ticket.WithAcceptanceCriteria("criterion"),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true
	testutil.MustCreateTicket(t, s, tk)

	got, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != "Round-trip" {
		t.Errorf("Title: got %q, want %q", got.Title, "Round-trip")
	}
	if got.Type != "bug" {
		t.Errorf("Type: got %q, want %q", got.Type, "bug")
	}
	if len(got.AcceptanceCriteria) != 1 || got.AcceptanceCriteria[0] != "criterion" {
		t.Errorf("AcceptanceCriteria: got %v, want [criterion]", got.AcceptanceCriteria)
	}
}

func TestReadNotFound(t *testing.T) {
	s := testutil.NewTestStore(t)
	_, err := s.Read("epo-does-not-exist")
	if err == nil {
		t.Fatal("Read: expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("Read: expected *TicketNotFoundError, got %T", err)
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestUpdatePreservesMarkdownBody(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := ticket.NewTicket(
		ticket.WithTitle("Preserve body"),
		ticket.WithType("task"),
		ticket.WithDescription("The description."),
		ticket.WithAcceptanceCriteria("must pass tests"),
		ticket.WithNotes("initial note"),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true
	testutil.MustCreateTicket(t, s, tk)

	// Change only status (as epos close does).
	tk.Status = ticket.StatusClosed
	tk.Present["status"] = true
	if err := s.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.Dir, store.TicketsDir, tk.ID+".md"))
	if err != nil {
		t.Fatalf("ReadFile after Update: %v", err)
	}
	body := string(data)

	if !strings.Contains(body, "The description.") {
		t.Errorf("Update dropped description from body:\n%s", body)
	}
	if !strings.Contains(body, "## Acceptance criteria") {
		t.Errorf("Update dropped ## Acceptance criteria from body:\n%s", body)
	}
	if !strings.Contains(body, "- must pass tests") {
		t.Errorf("Update dropped AC item from body:\n%s", body)
	}
	if !strings.Contains(body, "## Notes") {
		t.Errorf("Update dropped ## Notes from body:\n%s", body)
	}
	if !strings.Contains(body, "- initial note") {
		t.Errorf("Update dropped note text from body:\n%s", body)
	}
	if !strings.Contains(body, "status: closed") {
		t.Errorf("Update did not persist new status in frontmatter:\n%s", body)
	}
}

func TestUpdateRegeneratesStructuredBodyWhenRichContentChanges(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := ticket.NewTicket(
		ticket.WithTitle("Refresh body"),
		ticket.WithType("task"),
		ticket.WithDescription("Old description."),
		ticket.WithAcceptanceCriteria("old criterion"),
		ticket.WithNotes("old note"),
	)
	tk.ID = store.GenerateID("epo")
	tk.Present["id"] = true
	tk.ValidationCommands = []string{"go test ./old"}
	tk.Present["validation_commands"] = true
	testutil.MustCreateTicket(t, s, tk)

	tk.Description = "New description."
	tk.AcceptanceCriteria = []string{"new criterion"}
	tk.ValidationCommands = []string{"go test ./..."}
	tk.Notes = []string{"new note"}
	if err := s.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.Dir, store.TicketsDir, tk.ID+".md"))
	if err != nil {
		t.Fatalf("ReadFile after Update: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		"New description.",
		"- new criterion",
		"go test ./...",
		"- new note",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("updated body missing %q:\n%s", want, body)
		}
	}
	for _, stale := range []string{
		"Old description.",
		"- old criterion",
		"go test ./old",
		"- old note",
	} {
		if strings.Contains(body, stale) {
			t.Errorf("updated body retained stale content %q:\n%s", stale, body)
		}
	}
}

func TestUpdateStampsUpdatedAt(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("stamp test")
	testutil.MustCreateTicket(t, s, tk)

	if err := s.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := s.Read(tk.ID)
	if err != nil {
		t.Fatalf("Read after Update: %v", err)
	}
	if got.UpdatedAt == "" {
		t.Error("Update should stamp updated_at on the ticket")
	}
}

func TestUpdateRejectsNilTicket(t *testing.T) {
	s := testutil.NewTestStore(t)
	requireValidationError(t, s.Update(nil))
}

func TestUpdateNotFound(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("ghost")
	err := s.Update(tk) // never created
	if err == nil {
		t.Fatal("Update: expected TicketNotFoundError, got nil")
	}
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("Update: expected *TicketNotFoundError, got %T", err)
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestDeleteRemovesFile(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("delete me")
	testutil.MustCreateTicket(t, s, tk)

	if err := s.Delete(tk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	path := filepath.Join(s.Dir, store.TicketsDir, tk.ID+".md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted", path)
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := testutil.NewTestStore(t)
	err := s.Delete("epo-ghost-xxxx")
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("Delete: expected *TicketNotFoundError, got %T: %v", err, err)
	}
}

// ─── ResolveID ────────────────────────────────────────────────────────────────

func TestResolveIDExact(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("resolve exact")
	testutil.MustCreateTicket(t, s, tk)

	got, err := s.ResolveID(tk.ID)
	if err != nil {
		t.Fatalf("ResolveID exact: %v", err)
	}
	if got != tk.ID {
		t.Errorf("ResolveID exact: got %q, want %q", got, tk.ID)
	}
}

func TestResolveIDPartial(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk := testutil.NewTestTicket("partial match")
	testutil.MustCreateTicket(t, s, tk)

	// Use a 4-char suffix that appears somewhere in the ID.
	partial := tk.ID[len(tk.ID)-4:]
	got, err := s.ResolveID(partial)
	if err != nil {
		t.Fatalf("ResolveID partial %q: %v", partial, err)
	}
	if got != tk.ID {
		t.Errorf("ResolveID partial: got %q, want %q", got, tk.ID)
	}
}

func TestResolveIDNotFound(t *testing.T) {
	s := testutil.NewTestStore(t)
	_, err := s.ResolveID("zzz-nope")
	if _, ok := err.(*ticket.TicketNotFoundError); !ok {
		t.Errorf("ResolveID not found: expected *TicketNotFoundError, got %T", err)
	}
}

func TestResolveIDAmbiguous(t *testing.T) {
	s := testutil.NewTestStore(t)
	tk1 := testutil.NewTestTicket("first")
	tk2 := testutil.NewTestTicket("second")
	testutil.MustCreateTicket(t, s, tk1)
	testutil.MustCreateTicket(t, s, tk2)

	// "epo-" matches all tickets in this store.
	_, err := s.ResolveID("epo-")
	if _, ok := err.(*ticket.AmbiguousIDError); !ok {
		t.Errorf("ResolveID ambiguous: expected *AmbiguousIDError, got %T", err)
	}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestListReturnsAllTickets(t *testing.T) {
	s := testutil.NewTestStore(t)
	for _, title := range []string{"alpha", "beta", "gamma"} {
		testutil.MustCreateTicket(t, s, testutil.NewTestTicket(title))
	}
	all, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("List: got %d tickets, want 3", len(all))
	}
}

func TestListEmptyStore(t *testing.T) {
	s := testutil.NewTestStore(t)
	all, err := s.List()
	if err != nil {
		t.Fatalf("List empty store: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List empty store: got %d tickets, want 0", len(all))
	}
}

func requireValidationError(t *testing.T, err error) {
	t.Helper()
	var validation *ticket.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("expected *ticket.ValidationError, got %T: %v", err, err)
	}
}
