package ticket

// Validate checks a ticket for required fields and valid values.
// It returns a slice of ValidationErrors describing every detected problem.
// An empty slice means the ticket is valid.
func Validate(t Ticket) []ValidationError { //nolint:gocritic // hugeParam: Ticket is the canonical read-only API unit; pointer semantics would require nil guards and change the call-site contract
	var errs []ValidationError

	if t.ID == "" {
		errs = append(errs, ValidationError{Field: "id", Message: "required"})
	} else if idErr := ValidateID(t.ID); idErr != nil {
		errs = append(errs, *idErr)
	}

	if t.Title == "" && !t.TitleDerived {
		errs = append(errs, ValidationError{Field: "title", Message: "required"})
	}

	if t.Type == "" {
		errs = append(errs, ValidationError{Field: "type", Message: "required"})
	} else if !validTicketTypes[t.Type] {
		errs = append(errs, ValidationError{Field: "type", Message: "unknown type: " + t.Type})
	}

	if t.Status != "" && !isValidStatus(t.Status) {
		errs = append(errs, ValidationError{
			Field:   "status",
			Message: "unknown status: " + string(t.Status),
		})
	}

	return errs
}

// ValidateID checks that a ticket ID conforms to the required format:
// lowercase alphanumeric characters and hyphens only, non-empty.
// Returns a *ValidationError if the ID is invalid, or nil if it is valid.
func ValidateID(id string) *ValidationError {
	if id == "" {
		return &ValidationError{Field: "id", Message: "empty"}
	}
	for _, c := range id {
		if !isValidIDChar(c) {
			return &ValidationError{
				Field:   "id",
				Message: "invalid character in ID: must be lowercase alphanumeric with hyphens",
			}
		}
	}
	return nil
}

// ValidateTicketSchedulingFields checks fields used for dependency resolution and scheduling.
// It returns errors for self-references, duplicate dependencies, and parent/dep overlaps.
func ValidateTicketSchedulingFields(t Ticket) []ValidationError { //nolint:gocritic // hugeParam: Ticket is the canonical read-only API unit; pointer semantics would require nil guards and change the call-site contract
	var errs []ValidationError

	seen := make(map[string]bool, len(t.Deps))
	for _, dep := range t.Deps {
		if seen[dep] {
			errs = append(errs, ValidationError{
				Field:   "deps",
				Message: "duplicate dependency: " + dep,
			})
		}
		seen[dep] = true

		if dep == t.ID {
			errs = append(errs, ValidationError{
				Field:   "deps",
				Message: "ticket cannot depend on itself",
			})
		}

		if t.Parent != "" && dep == t.Parent {
			errs = append(errs, ValidationError{
				Field:   "deps",
				Message: "parent ticket should not also be listed as a dependency",
			})
		}
	}

	return errs
}

// isValidStatus reports whether s is a known Status constant.
func isValidStatus(s Status) bool {
	switch s {
	case StatusOpen, StatusReady, StatusInProgress, StatusBlocked, StatusClosed,
		StatusPending, StatusClaimed, StatusImplementing, StatusVerifying,
		StatusUnderReview, StatusRepairPending, StatusHeld, StatusDone, StatusFailed:
		return true
	default:
		return false
	}
}

// isValidIDChar reports whether c is an allowed character in a ticket ID.
func isValidIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}
