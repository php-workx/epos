package ticket

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TicketPatch describes a partial update to a Ticket. Only fields explicitly
// set via a setter (or decoded from JSON) are applied; absent fields are left
// unchanged when Apply is called.
type TicketPatch struct { //nolint:revive // stutter is intentional: TicketPatch is the canonical name used at call sites across packages
	priority           int
	parent             string
	deps               []string
	description        string
	acceptanceCriteria []string
	notes              []string
	assignee           string
	tags               []string
	intent             string
	present            map[string]bool
}

func (p *TicketPatch) mark(field string) {
	if p.present == nil {
		p.present = make(map[string]bool)
	}
	p.present[field] = true
}

// Has reports whether the named field was explicitly set on this patch.
func (p *TicketPatch) Has(field string) bool {
	return p.present != nil && p.present[field]
}

func (p *TicketPatch) SetPriority(v int)       { p.priority = v; p.mark("priority") }
func (p *TicketPatch) SetParent(v string)      { p.parent = v; p.mark("parent") }
func (p *TicketPatch) SetDeps(v []string)      { p.deps = append([]string(nil), v...); p.mark("deps") }
func (p *TicketPatch) SetDescription(v string) { p.description = v; p.mark("description") }
func (p *TicketPatch) SetAcceptanceCriteria(v []string) {
	p.acceptanceCriteria = append([]string(nil), v...)
	p.mark("acceptance_criteria")
}
func (p *TicketPatch) SetNotes(v []string)  { p.notes = append([]string(nil), v...); p.mark("notes") }
func (p *TicketPatch) SetAssignee(v string) { p.assignee = v; p.mark("assignee") }
func (p *TicketPatch) SetTags(v []string)   { p.tags = append([]string(nil), v...); p.mark("tags") }
func (p *TicketPatch) SetIntent(v string)   { p.intent = v; p.mark("intent") }

// Validate checks that all set fields contain valid values. Returns
// *ValidationError on the first invalid field. Fail-fast is deliberate: a patch
// originates from a single user command where reporting the first problem is
// sufficient.
func (p *TicketPatch) Validate() error {
	if p.Has("priority") && p.priority < 0 {
		return &ValidationError{Field: "priority", Message: "must be >= 0"}
	}
	if p.Has("tags") {
		seen := make(map[string]bool, len(p.tags))
		for i, tag := range p.tags {
			if strings.TrimSpace(tag) == "" {
				return &ValidationError{Field: "tags", Message: fmt.Sprintf("item %d is empty", i)}
			}
			if seen[tag] {
				return &ValidationError{Field: "tags", Message: fmt.Sprintf("duplicate tag %q", tag)}
			}
			seen[tag] = true
		}
	}
	if p.Has("acceptance_criteria") {
		for i, ac := range p.acceptanceCriteria {
			if strings.TrimSpace(ac) == "" {
				return &ValidationError{Field: "acceptance_criteria", Message: fmt.Sprintf("item %d is empty", i)}
			}
		}
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler. Only JSON keys that are present
// in the input are marked as set; absent keys are not applied by Apply.
// The JSON key for the description field is "description" (not "body").
//
// Null handling: for the priority field, JSON null is treated as absent (the
// field is not marked set). For string and slice fields, JSON null is
// equivalent to setting the field to its zero value — the field IS marked set
// and Apply will clear it on the ticket.
func (p *TicketPatch) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	type shadow struct {
		Priority           *int     `json:"priority"`
		Parent             string   `json:"parent"`
		Deps               []string `json:"deps"`
		Description        string   `json:"description"`
		AcceptanceCriteria []string `json:"acceptance_criteria"`
		Notes              []string `json:"notes"`
		Assignee           string   `json:"assignee"`
		Tags               []string `json:"tags"`
		Intent             string   `json:"intent"`
	}
	var s shadow
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for key := range raw {
		switch key {
		case "priority":
			if s.Priority != nil {
				p.SetPriority(*s.Priority)
			}
		case "parent":
			p.SetParent(s.Parent)
		case "deps":
			p.SetDeps(s.Deps)
		case "description":
			p.SetDescription(s.Description)
		case "acceptance_criteria":
			p.SetAcceptanceCriteria(s.AcceptanceCriteria)
		case "notes":
			p.SetNotes(s.Notes)
		case "assignee":
			p.SetAssignee(s.Assignee)
		case "tags":
			p.SetTags(s.Tags)
		case "intent":
			p.SetIntent(s.Intent)
		}
	}
	return nil
}

// Apply writes all set fields from the patch onto t, marking each field present
// in t.Present. Unset fields are left unchanged.
func (p *TicketPatch) Apply(t *Ticket) {
	if t.Present == nil {
		t.Present = make(map[string]bool)
	}
	if p.Has("priority") {
		t.Priority = p.priority
		t.Present["priority"] = true
	}
	if p.Has("parent") {
		t.Parent = p.parent
		t.Present["parent"] = true
	}
	if p.Has("deps") {
		t.Deps = append([]string(nil), p.deps...)
		t.Present["deps"] = true
	}
	if p.Has("description") {
		t.Description = p.description
		t.Present["description"] = true
	}
	if p.Has("acceptance_criteria") {
		t.AcceptanceCriteria = append([]string(nil), p.acceptanceCriteria...)
		t.Present["acceptance_criteria"] = true
	}
	if p.Has("notes") {
		t.Notes = append([]string(nil), p.notes...)
		t.Present["notes"] = true
	}
	if p.Has("assignee") {
		t.Assignee = p.assignee
		t.Present["assignee"] = true
	}
	if p.Has("tags") {
		t.Tags = append([]string(nil), p.tags...)
		t.Present["tags"] = true
	}
	if p.Has("intent") {
		t.Intent = p.intent
		t.Present["intent"] = true
	}
}
