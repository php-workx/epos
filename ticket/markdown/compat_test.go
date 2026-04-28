package markdown_test

import (
	"testing"
	"time"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/markdown"
)

// ─── StatusToTK ──────────────────────────────────────────────────────────────

// TestStatusMappingAllExtended verifies that all epos extended statuses
// map to the correct tk 3-status values.
func TestStatusMappingAllExtended(t *testing.T) {
	tests := []struct {
		input ticket.Status
		want  ticket.Status
	}{
		{ticket.StatusPending, ticket.StatusReady},
		{ticket.StatusClaimed, ticket.StatusInProgress},
		{ticket.StatusImplementing, ticket.StatusInProgress},
		{ticket.StatusVerifying, ticket.StatusInProgress},
		{ticket.StatusUnderReview, ticket.StatusInProgress},
		{ticket.StatusRepairPending, ticket.StatusReady},
		{ticket.StatusDone, ticket.StatusClosed},
		{ticket.StatusFailed, ticket.StatusClosed},
		{ticket.StatusHeld, ticket.StatusBlocked},
	}

	for _, tc := range tests {
		got := markdown.StatusToTK(tc.input)
		if got != tc.want {
			t.Errorf("StatusToTK(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestStatusToTKNativePassThrough verifies that tk-native statuses pass through
// StatusToTK unchanged.
func TestStatusToTKNativePassThrough(t *testing.T) {
	natives := []ticket.Status{
		ticket.StatusOpen,
		ticket.StatusReady,
		ticket.StatusInProgress,
		ticket.StatusBlocked,
		ticket.StatusClosed,
	}

	for _, s := range natives {
		got := markdown.StatusToTK(s)
		if got != s {
			t.Errorf("StatusToTK(%q) should pass through unchanged, got %q", s, got)
		}
	}
}

// TestStatusToTKUnknownFallback verifies that an unknown status falls back to StatusOpen.
func TestStatusToTKUnknownFallback(t *testing.T) {
	got := markdown.StatusToTK("completely_unknown_status")
	if got != ticket.StatusOpen {
		t.Errorf("StatusToTK(unknown) = %q, want %q", got, ticket.StatusOpen)
	}
}

// ─── StatusFromTK ────────────────────────────────────────────────────────────

func TestStatusFromTK(t *testing.T) {
	tests := []struct {
		name     string
		tkStatus ticket.Status
		extended string
		want     ticket.Status
	}{
		{
			name:     "hint overrides tk status",
			tkStatus: ticket.StatusInProgress,
			extended: "verifying",
			want:     ticket.StatusVerifying,
		},
		{
			name:     "empty hint: in_progress maps to implementing",
			tkStatus: ticket.StatusInProgress,
			extended: "",
			want:     ticket.StatusImplementing,
		},
		{
			name:     "empty hint: closed maps to done",
			tkStatus: ticket.StatusClosed,
			extended: "",
			want:     ticket.StatusDone,
		},
		{
			name:     "empty hint: blocked maps to held",
			tkStatus: ticket.StatusBlocked,
			extended: "",
			want:     ticket.StatusHeld,
		},
		{
			name:     "empty hint: open maps to pending",
			tkStatus: ticket.StatusOpen,
			extended: "",
			want:     ticket.StatusPending,
		},
		{
			name:     "empty hint: ready maps to pending",
			tkStatus: ticket.StatusReady,
			extended: "",
			want:     ticket.StatusPending,
		},
		{
			name:     "invalid hint is ignored",
			tkStatus: ticket.StatusInProgress,
			extended: "not_a_valid_status",
			want:     ticket.StatusImplementing,
		},
		{
			name:     "held hint is valid and accepted",
			tkStatus: ticket.StatusBlocked,
			extended: "held",
			want:     ticket.StatusHeld,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := markdown.StatusFromTK(tc.tkStatus, tc.extended)
			if got != tc.want {
				t.Errorf("StatusFromTK(%q, %q) = %q, want %q", tc.tkStatus, tc.extended, got, tc.want)
			}
		})
	}
}

// ─── StatusToExtended ────────────────────────────────────────────────────────

func TestStatusToExtended(t *testing.T) {
	tests := []struct {
		input ticket.Status
		want  string
	}{
		{ticket.StatusOpen, "pending"},
		{ticket.StatusReady, "pending"},
		{ticket.StatusInProgress, "implementing"},
		{ticket.StatusClosed, "done"},
		{ticket.StatusBlocked, "held"},
		// Extended statuses pass through as-is.
		{ticket.StatusPending, "pending"},
		{ticket.StatusImplementing, "implementing"},
		{ticket.StatusDone, "done"},
		{ticket.StatusFailed, "failed"},
		{ticket.StatusHeld, "held"},
	}

	for _, tc := range tests {
		got := markdown.StatusToExtended(tc.input)
		if got != tc.want {
			t.Errorf("StatusToExtended(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ─── formatTime / parseTime ──────────────────────────────────────────────────

func TestFormatParseTime(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)

	formatted := markdown.FormatTime(now)
	if formatted != "2026-04-24T12:00:00Z" {
		t.Errorf("FormatTime: got %q, want %q", formatted, "2026-04-24T12:00:00Z")
	}

	parsed, err := markdown.ParseTime(formatted)
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	if !parsed.Equal(now) {
		t.Errorf("ParseTime round-trip: got %v, want %v", parsed, now)
	}
}

func TestParseTimeError(t *testing.T) {
	_, err := markdown.ParseTime("not-a-time")
	if err == nil {
		t.Error("ParseTime should return an error for invalid time string")
	}
}
