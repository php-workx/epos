package ticket_test

import (
	"testing"

	"github.com/php-workx/epos/ticket"
)

func TestApplySetsPriorityOnTicket(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetPriority(5)
	tk := &ticket.Ticket{}
	p.Apply(tk)
	if tk.Priority != 5 {
		t.Errorf("Priority: got %d, want 5", tk.Priority)
	}
	if !tk.Present["priority"] {
		t.Error("priority not marked present in ticket")
	}
}

func TestApplyLeavesAbsentFieldUnchanged(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetPriority(5) // only priority set
	tk := &ticket.Ticket{Assignee: "alice"}
	p.Apply(tk)
	if tk.Assignee != "alice" {
		t.Errorf("Assignee: got %q, want %q", tk.Assignee, "alice")
	}
}

func TestApplyInitialisesTicketPresent(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetParent("epo-parent-xxxx")
	tk := &ticket.Ticket{} // Present is nil
	p.Apply(tk)
	if tk.Present == nil {
		t.Fatal("Apply did not initialise ticket.Present")
	}
	if !tk.Present["parent"] {
		t.Error("parent not marked present in ticket")
	}
}

func TestApplyAllNineFields(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetPriority(3)
	p.SetParent("epo-parent-xxxx")
	p.SetDeps([]string{"epo-dep-xxxx"})
	p.SetDescription("new description")
	p.SetAcceptanceCriteria([]string{"criterion one"})
	p.SetNotes([]string{"note one"})
	p.SetAssignee("alice")
	p.SetTags([]string{"urgent"})
	p.SetIntent("deliver value")

	tk := &ticket.Ticket{}
	p.Apply(tk)

	if tk.Priority != 3 {
		t.Errorf("Priority: got %d, want 3", tk.Priority)
	}
	if tk.Parent != "epo-parent-xxxx" {
		t.Errorf("Parent: got %q", tk.Parent)
	}
	if len(tk.Deps) != 1 || tk.Deps[0] != "epo-dep-xxxx" {
		t.Errorf("Deps: %v", tk.Deps)
	}
	if tk.Description != "new description" {
		t.Errorf("Description: got %q", tk.Description)
	}
	if len(tk.AcceptanceCriteria) != 1 || tk.AcceptanceCriteria[0] != "criterion one" {
		t.Errorf("AcceptanceCriteria: %v", tk.AcceptanceCriteria)
	}
	if len(tk.Notes) != 1 || tk.Notes[0] != "note one" {
		t.Errorf("Notes: %v", tk.Notes)
	}
	if tk.Assignee != "alice" {
		t.Errorf("Assignee: got %q", tk.Assignee)
	}
	if len(tk.Tags) != 1 || tk.Tags[0] != "urgent" {
		t.Errorf("Tags: %v", tk.Tags)
	}
	if tk.Intent != "deliver value" {
		t.Errorf("Intent: got %q", tk.Intent)
	}
}
