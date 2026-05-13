package ticket

import "fmt"

// TicketNotFoundError is returned when a ticket cannot be found by its ID.
type TicketNotFoundError struct { //nolint:revive // stutter is intentional: callers write ticket.TicketNotFoundError in type assertions across packages
	// ID is the ticket ID that was not found.
	ID string
}

func (e *TicketNotFoundError) Error() string {
	return fmt.Sprintf("ticket not found: %q", e.ID)
}

func (e *TicketNotFoundError) ExitCode() int { return 3 }

// AmbiguousIDError is returned when a partial ID matches more than one ticket.
type AmbiguousIDError struct {
	// Partial is the partial ID that was supplied.
	Partial string
	// Matches contains all ticket IDs that matched the partial.
	Matches []string
}

func (e *AmbiguousIDError) Error() string {
	return fmt.Sprintf("ambiguous ticket ID %q: matches %v", e.Partial, e.Matches)
}

func (e *AmbiguousIDError) ExitCode() int { return 4 }

// IDCollisionError is returned when a generated ticket ID already exists in the store.
type IDCollisionError struct {
	// ID is the ID that collided.
	ID string
}

func (e *IDCollisionError) Error() string {
	return fmt.Sprintf("ticket ID collision: %q already exists", e.ID)
}

// CorruptYAMLError is returned when a ticket file cannot be parsed due to invalid YAML.
type CorruptYAMLError struct {
	// Path is the filesystem path of the corrupted file.
	Path string
	// Cause is the underlying parse error.
	Cause error
}

func (e *CorruptYAMLError) Error() string {
	return fmt.Sprintf("corrupt YAML in %q: %v", e.Path, e.Cause)
}

// Unwrap returns the underlying parse error so errors.Is/As can traverse the chain.
func (e *CorruptYAMLError) Unwrap() error {
	return e.Cause
}

// CycleDetectedError is returned when a dependency cycle is detected in the ticket graph.
type CycleDetectedError struct {
	// Cycle contains the ticket IDs that form the cycle, in order.
	Cycle []string
}

func (e *CycleDetectedError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %v", e.Cycle)
}

func (e *CycleDetectedError) ExitCode() int { return 5 }

// AlreadyClaimedError is returned when a ticket is already claimed by a different agent.
type AlreadyClaimedError struct {
	// TicketID is the claimed ticket's ID.
	TicketID string
	// ClaimedBy is the identifier of the agent holding the claim.
	ClaimedBy string
}

func (e *AlreadyClaimedError) Error() string {
	return fmt.Sprintf("ticket %q is already claimed by %q", e.TicketID, e.ClaimedBy)
}

func (e *AlreadyClaimedError) ExitCode() int { return 6 }

// NotClaimedError is returned when an operation requires an active claim that does not exist.
type NotClaimedError struct {
	// TicketID is the ticket that lacks a claim.
	TicketID string
}

func (e *NotClaimedError) Error() string {
	return fmt.Sprintf("ticket %q is not claimed", e.TicketID)
}

func (e *NotClaimedError) ExitCode() int { return 6 }

// NotClaimOwnerError is returned when the caller does not own the claim it is trying to use.
type NotClaimOwnerError struct {
	// TicketID is the ticket whose claim is being contested.
	TicketID string
	// ClaimedBy is the identifier of the agent that holds the claim.
	ClaimedBy string
	// Caller is the identifier of the agent that attempted the operation.
	Caller string
}

func (e *NotClaimOwnerError) Error() string {
	return fmt.Sprintf("ticket %q is claimed by %q, not %q", e.TicketID, e.ClaimedBy, e.Caller)
}

func (e *NotClaimOwnerError) ExitCode() int { return 6 }

// ValidationError is returned when a ticket field fails validation.
type ValidationError struct {
	// Field is the name of the ticket field that failed validation.
	Field string
	// Message describes the validation failure.
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: field %q: %s", e.Field, e.Message)
}

func (e *ValidationError) ExitCode() int { return 2 }

// PartialReadError is returned when a store read operation partially succeeds,
// meaning some tickets were loaded and some were not.
type PartialReadError struct {
	// Succeeded contains the IDs of tickets that were read successfully.
	Succeeded []string
	// Failed maps ticket IDs to the errors that prevented them from loading.
	Failed map[string]error
}

func (e *PartialReadError) Error() string {
	return fmt.Sprintf("partial read: %d succeeded, %d failed", len(e.Succeeded), len(e.Failed))
}
