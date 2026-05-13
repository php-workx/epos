package ticket_test

import (
	"testing"

	"github.com/php-workx/epos/ticket"
)

func TestNewTicketDefaults(t *testing.T) {
	tk := ticket.NewTicket()

	if tk.Present == nil {
		t.Fatal("Present should be non-nil after NewTicket()")
	}
	if len(tk.Present) != 0 {
		t.Errorf("Present should be empty map, got %v", tk.Present)
	}
	if tk.ExtendedStatus != "open" {
		t.Errorf("ExtendedStatus should be %q, got %q", "open", tk.ExtendedStatus)
	}
}

func TestNewTicketSetsPresentMap(t *testing.T) {
	tk := ticket.NewTicket(
		ticket.WithTitle("my ticket"),
		ticket.WithType("task"),
	)

	if !tk.Present["title"] {
		t.Error("Present[\"title\"] should be true after WithTitle")
	}
	if !tk.Present["type"] {
		t.Error("Present[\"type\"] should be true after WithType")
	}
	if tk.Present["status"] {
		t.Error("Present[\"status\"] should be false when WithStatus was not called")
	}
	if tk.Present["parent"] {
		t.Error("Present[\"parent\"] should be false when WithParent was not called")
	}
}

func TestStatusConstants(t *testing.T) {
	if ticket.StatusHeld != "held" {
		t.Errorf("StatusHeld should be %q, got %q", "held", ticket.StatusHeld)
	}
	if ticket.StatusBlocked != "blocked" {
		t.Errorf("StatusBlocked should be %q, got %q", "blocked", ticket.StatusBlocked)
	}

	// Verify no two constants share the same string value.
	statuses := map[string]ticket.Status{
		"StatusOpen":          ticket.StatusOpen,
		"StatusReady":         ticket.StatusReady,
		"StatusInProgress":    ticket.StatusInProgress,
		"StatusBlocked":       ticket.StatusBlocked,
		"StatusClosed":        ticket.StatusClosed,
		"StatusPending":       ticket.StatusPending,
		"StatusClaimed":       ticket.StatusClaimed,
		"StatusImplementing":  ticket.StatusImplementing,
		"StatusVerifying":     ticket.StatusVerifying,
		"StatusUnderReview":   ticket.StatusUnderReview,
		"StatusRepairPending": ticket.StatusRepairPending,
		"StatusHeld":          ticket.StatusHeld,
		"StatusDone":          ticket.StatusDone,
		"StatusFailed":        ticket.StatusFailed,
	}

	seen := make(map[ticket.Status]string, len(statuses))
	for name, s := range statuses {
		if prev, ok := seen[s]; ok {
			t.Errorf("duplicate status value %q: shared by %s and %s", s, prev, name)
		}
		seen[s] = name
	}
}

func TestValidTicketTypes(t *testing.T) {
	// "task" must be accepted.
	errs := ticket.Validate(ticket.Ticket{ID: "abc-1234", Title: "Valid", Type: "task"})
	for _, e := range errs {
		if e.Field == "type" {
			t.Errorf("unexpected type validation error for known type %q: %v", "task", e)
		}
	}

	// "epic" must be accepted.
	errs = ticket.Validate(ticket.Ticket{ID: "abc-1234", Title: "Valid", Type: "epic"})
	for _, e := range errs {
		if e.Field == "type" {
			t.Errorf("unexpected type validation error for known type %q: %v", "epic", e)
		}
	}

	// "unknown" must be rejected.
	errs = ticket.Validate(ticket.Ticket{ID: "abc-1234", Title: "Valid", Type: "unknown"})
	found := false
	for _, e := range errs {
		if e.Field == "type" {
			found = true
		}
	}
	if !found {
		t.Error("expected ValidationError{Field: \"type\"} for Type \"unknown\", got none")
	}
}

func TestTicketOptionPattern(t *testing.T) {
	tk := ticket.NewTicket(
		ticket.WithTitle("Test Ticket"),
		ticket.WithType("task"),
		ticket.WithStatus(ticket.StatusOpen),
		ticket.WithParent("parent-abc"),
		ticket.WithDeps("dep-001", "dep-002"),
	)

	if tk.Title != "Test Ticket" {
		t.Errorf("Title: got %q, want %q", tk.Title, "Test Ticket")
	}
	if tk.Type != "task" {
		t.Errorf("Type: got %q, want %q", tk.Type, "task")
	}
	if tk.Status != ticket.StatusOpen {
		t.Errorf("Status: got %q, want %q", tk.Status, ticket.StatusOpen)
	}
	if tk.Parent != "parent-abc" {
		t.Errorf("Parent: got %q, want %q", tk.Parent, "parent-abc")
	}
	if len(tk.Deps) != 2 || tk.Deps[0] != "dep-001" || tk.Deps[1] != "dep-002" {
		t.Errorf("Deps: got %v, want [dep-001 dep-002]", tk.Deps)
	}

	// All set fields must be present.
	for _, field := range []string{"title", "type", "status", "parent", "deps"} {
		if !tk.Present[field] {
			t.Errorf("Present[%q] should be true after option was applied", field)
		}
	}
}

func TestNewTicketOptionFunctions(t *testing.T) {
	tk := ticket.NewTicket(
		ticket.WithDescription("narrative body"),
		ticket.WithAssignee("agent-007"),
		ticket.WithTags("backend", "api"),
		ticket.WithAcceptanceCriteria("ac one", "ac two"),
		ticket.WithNotes("note one", "note two"),
		ticket.WithIntent("the why"),
	)

	if tk.Description != "narrative body" {
		t.Errorf("Description: got %q, want %q", tk.Description, "narrative body")
	}
	if tk.Assignee != "agent-007" {
		t.Errorf("Assignee: got %q, want %q", tk.Assignee, "agent-007")
	}
	if len(tk.Tags) != 2 || tk.Tags[0] != "backend" || tk.Tags[1] != "api" {
		t.Errorf("Tags: got %v, want [backend api]", tk.Tags)
	}
	if len(tk.AcceptanceCriteria) != 2 || tk.AcceptanceCriteria[0] != "ac one" {
		t.Errorf("AcceptanceCriteria: got %v, want [ac one ac two]", tk.AcceptanceCriteria)
	}
	if len(tk.Notes) != 2 || tk.Notes[0] != "note one" {
		t.Errorf("Notes: got %v, want [note one note two]", tk.Notes)
	}
	if tk.Intent != "the why" {
		t.Errorf("Intent: got %q, want %q", tk.Intent, "the why")
	}

	// All set fields must be marked present.
	for _, field := range []string{"description", "assignee", "tags", "acceptance_criteria", "notes", "intent"} {
		if !tk.Present[field] {
			t.Errorf("Present[%q] should be true after option was applied", field)
		}
	}
}

func TestWithTagsVariadic(t *testing.T) {
	tk := ticket.NewTicket(ticket.WithTags("x"))
	if len(tk.Tags) != 1 || tk.Tags[0] != "x" {
		t.Errorf("WithTags single: got %v", tk.Tags)
	}
	tk2 := ticket.NewTicket(ticket.WithTags())
	if len(tk2.Tags) != 0 {
		t.Errorf("WithTags empty: got %v", tk2.Tags)
	}
}

func TestWithAcceptanceCriteriaVariadic(t *testing.T) {
	tk := ticket.NewTicket(ticket.WithAcceptanceCriteria("only one"))
	if len(tk.AcceptanceCriteria) != 1 {
		t.Errorf("WithAcceptanceCriteria single: got %v", tk.AcceptanceCriteria)
	}
}

func TestTicketStructZeroValues(t *testing.T) {
	var tk ticket.Ticket

	if tk.ID != "" {
		t.Errorf("zero-value ID: got %q, want empty string", tk.ID)
	}
	if tk.Present != nil {
		t.Errorf("zero-value Present: got %v, want nil", tk.Present)
	}
	if tk.TitleDerived {
		t.Error("zero-value TitleDerived should be false")
	}
	if tk.Priority != 0 {
		t.Errorf("zero-value Priority: got %d, want 0", tk.Priority)
	}
	if tk.Status != "" {
		t.Errorf("zero-value Status: got %q, want empty string", tk.Status)
	}
}

func TestIsValidType(t *testing.T) {
	for _, v := range []string{"epic", "task", "issue", "feature", "bug", "chore", "spike", "doc"} {
		if !ticket.IsValidType(v) {
			t.Errorf("IsValidType(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"", "TASK", "Task", "unknown", "todo"} {
		if ticket.IsValidType(v) {
			t.Errorf("IsValidType(%q) = true, want false", v)
		}
	}
}
