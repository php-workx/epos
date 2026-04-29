package markdown

import (
	"time"

	"github.com/php-workx/epos/ticket"
)

// StatusToTK maps an epos status (extended or tk-native) to the tk 3-status model.
//
// Mapping:
//   - pending, repair_pending → ready
//   - claimed, implementing, verifying, under_review → in_progress
//   - done, failed → closed
//   - held → blocked  (operator-initiated blocking)
//   - tk-native statuses (open, ready, in_progress, blocked, closed) pass through unchanged
func StatusToTK(s ticket.Status) ticket.Status {
	switch s {
	case ticket.StatusPending, ticket.StatusRepairPending:
		return ticket.StatusReady
	case ticket.StatusClaimed, ticket.StatusImplementing,
		ticket.StatusVerifying, ticket.StatusUnderReview:
		return ticket.StatusInProgress
	case ticket.StatusDone, ticket.StatusFailed:
		return ticket.StatusClosed
	case ticket.StatusHeld:
		return ticket.StatusBlocked
	// tk-native statuses are returned unchanged.
	case ticket.StatusOpen, ticket.StatusReady,
		ticket.StatusInProgress, ticket.StatusBlocked, ticket.StatusClosed:
		return s
	default:
		return ticket.StatusOpen
	}
}

// StatusFromTK maps a tk-native status to an epos status.
// When extendedStatus is provided and is a recognised epos status string, it is
// returned directly — this preserves the finer-grained extended status across the
// tk round-trip. Otherwise the tk status is mapped to a canonical extended status.
func StatusFromTK(tkStatus ticket.Status, extendedStatus string) ticket.Status {
	if extendedStatus != "" {
		s := ticket.Status(extendedStatus)
		if isValidExtendedStatus(s) {
			return s
		}
	}
	switch tkStatus {
	case ticket.StatusReady, ticket.StatusOpen:
		return ticket.StatusPending
	case ticket.StatusInProgress:
		return ticket.StatusImplementing
	case ticket.StatusClosed:
		return ticket.StatusDone
	case ticket.StatusBlocked:
		return ticket.StatusHeld
	default:
		return ticket.StatusPending
	}
}

// StatusToExtended maps any status to the canonical extended status string.
// Extended statuses are returned as-is; tk-native statuses are mapped to the
// most general extended equivalent.
func StatusToExtended(s ticket.Status) string {
	switch s {
	case ticket.StatusOpen, ticket.StatusReady:
		return string(ticket.StatusPending)
	case ticket.StatusInProgress:
		return string(ticket.StatusImplementing)
	case ticket.StatusClosed:
		return string(ticket.StatusDone)
	case ticket.StatusBlocked:
		return string(ticket.StatusHeld)
	default:
		return string(s)
	}
}

// isValidExtendedStatus reports whether s is a recognised epos status constant.
func isValidExtendedStatus(s ticket.Status) bool {
	switch s {
	case ticket.StatusOpen, ticket.StatusReady, ticket.StatusInProgress,
		ticket.StatusBlocked, ticket.StatusClosed,
		ticket.StatusPending, ticket.StatusClaimed, ticket.StatusImplementing,
		ticket.StatusVerifying, ticket.StatusUnderReview, ticket.StatusRepairPending,
		ticket.StatusHeld, ticket.StatusDone, ticket.StatusFailed:
		return true
	default:
		return false
	}
}

// FormatTime formats t for storage in ticket frontmatter using RFC 3339 UTC.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseTime parses a frontmatter time string in RFC 3339 format.
func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
