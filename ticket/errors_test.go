package ticket_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
)

func TestTicketNotFoundError(t *testing.T) {
	err := &ticket.TicketNotFoundError{ID: "abc-1234"}

	if err.Error() == "" {
		t.Error("TicketNotFoundError.Error() must not return empty string")
	}
	if !strings.Contains(err.Error(), "abc-1234") {
		t.Errorf("TicketNotFoundError.Error() should contain the ID, got: %q", err.Error())
	}
}

func TestAmbiguousIDError(t *testing.T) {
	err := &ticket.AmbiguousIDError{
		Partial: "abc",
		Matches: []string{"abc-001", "abc-002"},
	}

	if err.Error() == "" {
		t.Error("AmbiguousIDError.Error() must not return empty string")
	}
	if !strings.Contains(err.Error(), "abc") {
		t.Errorf("AmbiguousIDError.Error() should contain the partial ID, got: %q", err.Error())
	}
}

func TestCorruptYAMLError(t *testing.T) {
	cause := errors.New("yaml: unmarshal error")
	err := &ticket.CorruptYAMLError{Path: "/some/path.md", Cause: cause}

	if err.Error() == "" {
		t.Error("CorruptYAMLError.Error() must not return empty string")
	}
	if !strings.Contains(err.Error(), "/some/path.md") {
		t.Errorf("CorruptYAMLError.Error() should contain the path, got: %q", err.Error())
	}
	// Unwrap must expose the underlying cause.
	if !errors.Is(err, cause) {
		t.Error("errors.Is(corruptYAMLError, cause) should return true via Unwrap()")
	}
}

func TestIDCollisionError(t *testing.T) {
	err := &ticket.IDCollisionError{ID: "abc-1234"}

	if err.Error() == "" {
		t.Error("IDCollisionError.Error() must not return empty string")
	}
	if !strings.Contains(err.Error(), "abc-1234") {
		t.Errorf("IDCollisionError.Error() should contain the ID, got: %q", err.Error())
	}
}

func TestCustomErrorTypeAssertions(t *testing.T) {
	// Simulate CLI dispatch: errors are returned as the error interface and
	// the caller performs type assertions to select exit codes.

	var notFound error = &ticket.TicketNotFoundError{ID: "xyz-9999"}
	var ambiguous error = &ticket.AmbiguousIDError{Partial: "xyz", Matches: []string{"xyz-0001", "xyz-0002"}}

	// *TicketNotFoundError type assertion must succeed.
	var nfe *ticket.TicketNotFoundError
	if !errors.As(notFound, &nfe) {
		t.Fatal("errors.As(*TicketNotFoundError) should succeed")
	}
	if nfe.ID != "xyz-9999" {
		t.Errorf("TicketNotFoundError.ID: got %q, want %q", nfe.ID, "xyz-9999")
	}

	// *AmbiguousIDError type assertion must succeed.
	var ambig *ticket.AmbiguousIDError
	if !errors.As(ambiguous, &ambig) {
		t.Fatal("errors.As(*AmbiguousIDError) should succeed")
	}
	if ambig.Partial != "xyz" {
		t.Errorf("AmbiguousIDError.Partial: got %q, want %q", ambig.Partial, "xyz")
	}
	if len(ambig.Matches) != 2 {
		t.Errorf("AmbiguousIDError.Matches: got %v, want 2 entries", ambig.Matches)
	}

	// Ensure the error types do NOT cross-assert.
	var wrongType *ticket.IDCollisionError
	if errors.As(notFound, &wrongType) {
		t.Error("errors.As(*IDCollisionError) should NOT succeed for a TicketNotFoundError")
	}
}
