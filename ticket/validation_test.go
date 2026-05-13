package ticket_test

import (
	"errors"
	"testing"

	"github.com/php-workx/epos/ticket"
)

func TestValidateValidTicket(t *testing.T) {
	tk := ticket.Ticket{
		ID:    "abc-1234",
		Title: "Valid Ticket",
		Type:  "task",
	}
	errs := ticket.Validate(tk)
	if len(errs) != 0 {
		t.Errorf("Validate: expected no errors for valid ticket, got: %v", errs)
	}
}

func TestValidateInvalidType(t *testing.T) {
	tk := ticket.Ticket{
		ID:    "abc-1234",
		Title: "Some Ticket",
		Type:  "unknown",
	}
	errs := ticket.Validate(tk)

	var found *ticket.ValidationError
	for i := range errs {
		if errs[i].Field == "type" {
			found = &errs[i]
			break
		}
	}
	if found == nil {
		t.Errorf("Validate: expected ValidationError{Field:\"type\"} for Type=%q, got: %v", "unknown", errs)
	}
}

func TestValidateMissingID(t *testing.T) {
	tk := ticket.Ticket{
		Title: "No ID",
		Type:  "task",
	}
	errs := ticket.Validate(tk)

	found := false
	for _, e := range errs {
		if e.Field == "id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Validate: expected ValidationError{Field:\"id\"} for missing ID, got: %v", errs)
	}
}

func TestValidateInvalidStatus(t *testing.T) {
	tk := ticket.Ticket{
		ID:     "abc-1234",
		Title:  "Some Ticket",
		Type:   "task",
		Status: ticket.Status("bogus-status"),
	}
	errs := ticket.Validate(tk)

	found := false
	for _, e := range errs {
		if e.Field == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Validate: expected ValidationError{Field:\"status\"} for invalid status, got: %v", errs)
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"abc-1234", true},
		{"epo-zols", true},
		{"abc", true},
		{"123", true},
		{"a-b-c-1", true},
		{"", false},
		{"ABC-1234", false},
		{"abc 1234", false},
		{"abc_1234", false},
		{"abc.1234", false},
		{"ABC", false},
	}

	for _, tt := range tests {
		err := ticket.ValidateID(tt.id)
		if tt.valid && err != nil {
			t.Errorf("ValidateID(%q): expected nil, got %v", tt.id, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateID(%q): expected error, got nil", tt.id)
		}
	}
}

func TestValidateNewNilTicket(t *testing.T) {
	err := ticket.ValidateNew(nil)
	if err == nil {
		t.Fatal("ValidateNew(nil): expected error, got nil")
	}
	var ve *ticket.ValidationError
	if errors.As(err, &ve) {
		t.Errorf("ValidateNew(nil): got *ValidationError (exit 2), want plain error (exit 1): %v", err)
	}
}

func TestValidateTagWhitespaceDuplicate(t *testing.T) {
	cases := []struct {
		name string
		tags []string
	}{
		{"space prefix", []string{"foo", " foo"}},
		{"tab prefix", []string{"foo", "\tfoo"}},
		{"both padded", []string{" foo", "\tfoo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tk := ticket.Ticket{ID: "abc-1", Title: "T", Type: "task", Tags: tc.tags}
			errs := ticket.Validate(tk)
			found := false
			for _, e := range errs {
				if e.Field == "tags" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Validate: expected duplicate-tag error for %v, got none", tc.tags)
			}
		})
	}
}

func TestValidateTicketSchedulingFields(t *testing.T) {
	t.Run("clean ticket passes", func(t *testing.T) {
		tk := ticket.Ticket{
			ID:   "abc-1234",
			Deps: []string{"dep-0001", "dep-0002"},
		}
		errs := ticket.ValidateTicketSchedulingFields(tk)
		if len(errs) != 0 {
			t.Errorf("expected no errors for clean ticket, got: %v", errs)
		}
	})

	t.Run("empty deps passes", func(t *testing.T) {
		tk := ticket.Ticket{ID: "abc-1234"}
		errs := ticket.ValidateTicketSchedulingFields(tk)
		if len(errs) != 0 {
			t.Errorf("expected no errors for empty deps, got: %v", errs)
		}
	})

	t.Run("self dependency rejected", func(t *testing.T) {
		tk := ticket.Ticket{
			ID:   "abc-1234",
			Deps: []string{"abc-1234"},
		}
		errs := ticket.ValidateTicketSchedulingFields(tk)
		if len(errs) == 0 {
			t.Error("expected error for self-dependency, got none")
		}
	})

	t.Run("duplicate dependency rejected", func(t *testing.T) {
		tk := ticket.Ticket{
			ID:   "abc-1234",
			Deps: []string{"dep-0001", "dep-0001"},
		}
		errs := ticket.ValidateTicketSchedulingFields(tk)
		if len(errs) == 0 {
			t.Error("expected error for duplicate dependency, got none")
		}
	})

	t.Run("parent listed as dep rejected", func(t *testing.T) {
		tk := ticket.Ticket{
			ID:     "abc-1234",
			Parent: "parent-epic",
			Deps:   []string{"parent-epic"},
		}
		errs := ticket.ValidateTicketSchedulingFields(tk)
		if len(errs) == 0 {
			t.Error("expected error for parent listed as dep, got none")
		}
	})
}
