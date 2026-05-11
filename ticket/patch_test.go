package ticket_test

import (
	"encoding/json"
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

func TestValidatePriorityNegative(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetPriority(-1)
	err := p.Validate()
	if err == nil {
		t.Fatal("expected ValidationError, got nil")
	}
	if _, ok := err.(*ticket.ValidationError); !ok {
		t.Errorf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestValidatePriorityZeroIsValid(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetPriority(0)
	if err := p.Validate(); err != nil {
		t.Errorf("priority=0 should be valid, got %v", err)
	}
}

func TestValidateTagEmpty(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetTags([]string{""})
	if err := p.Validate(); err == nil {
		t.Fatal("expected ValidationError for empty tag, got nil")
	}
}

func TestValidateTagDuplicate(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetTags([]string{"urgent", "urgent"})
	if err := p.Validate(); err == nil {
		t.Fatal("expected ValidationError for duplicate tag, got nil")
	}
}

func TestValidateACItemEmpty(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetAcceptanceCriteria([]string{""})
	if err := p.Validate(); err == nil {
		t.Fatal("expected ValidationError for empty AC item, got nil")
	}
}

func TestValidateEmptyPatchIsValid(t *testing.T) {
	p := &ticket.TicketPatch{} // nothing set
	if err := p.Validate(); err != nil {
		t.Errorf("empty patch Validate: got %v, want nil", err)
	}
}

func TestValidateUnsetFieldsSkipChecks(t *testing.T) {
	p := &ticket.TicketPatch{}
	p.SetAssignee("alice") // something set, but not priority/tags/ac
	if err := p.Validate(); err != nil {
		t.Errorf("patch with only assignee set: got %v, want nil", err)
	}
}

func TestUnmarshalJSONPartialInput(t *testing.T) {
	var p ticket.TicketPatch
	if err := json.Unmarshal([]byte(`{"assignee":"bob"}`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	tk := &ticket.Ticket{Priority: 7}
	p.Apply(tk)
	if tk.Assignee != "bob" {
		t.Errorf("Assignee: got %q, want %q", tk.Assignee, "bob")
	}
	// Priority was not in the JSON — must remain untouched.
	if tk.Priority != 7 {
		t.Errorf("Priority mutated by partial patch: got %d, want 7", tk.Priority)
	}
}

func TestUnmarshalJSONPriorityZeroIsSet(t *testing.T) {
	var p ticket.TicketPatch
	if err := json.Unmarshal([]byte(`{"priority":0}`), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !p.Has("priority") {
		t.Error("priority=0 in JSON should be treated as set, not absent")
	}
	tk := &ticket.Ticket{Priority: 5}
	p.Apply(tk)
	if tk.Priority != 0 {
		t.Errorf("Priority: got %d, want 0", tk.Priority)
	}
}

func TestUnmarshalJSONUnknownKeysIgnored(t *testing.T) {
	var p ticket.TicketPatch
	err := json.Unmarshal([]byte(`{"unknown_field":"value","assignee":"alice"}`), &p)
	if err != nil {
		t.Fatalf("Unmarshal with unknown key: %v", err)
	}
	if !p.Has("assignee") {
		t.Error("known field 'assignee' should be set")
	}
}

func TestUnmarshalJSONInvalidJSON(t *testing.T) {
	var p ticket.TicketPatch
	if err := json.Unmarshal([]byte(`{invalid}`), &p); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestUnmarshalJSONThenApply(t *testing.T) {
	var p ticket.TicketPatch
	input := `{"priority":3,"tags":["urgent","review"],"description":"new desc"}`
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	tk := &ticket.Ticket{Assignee: "existing"}
	p.Apply(tk)
	if tk.Priority != 3 {
		t.Errorf("Priority: got %d, want 3", tk.Priority)
	}
	if len(tk.Tags) != 2 {
		t.Errorf("Tags: got %v, want [urgent review]", tk.Tags)
	}
	if tk.Description != "new desc" {
		t.Errorf("Description: got %q, want %q", tk.Description, "new desc")
	}
	// Assignee was not in JSON — must remain untouched.
	if tk.Assignee != "existing" {
		t.Errorf("Assignee mutated: got %q, want %q", tk.Assignee, "existing")
	}
}
