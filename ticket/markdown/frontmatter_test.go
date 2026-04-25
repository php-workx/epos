package markdown_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/markdown"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// simpleTicket builds a Ticket that is safe for round-trip tests: only scalar
// and slice-of-string fields, all of which have explicit Present entries.
func simpleTicket() *ticket.Ticket {
	return &ticket.Ticket{
		ID:       "test-1234",
		Title:    "Round-trip ticket",
		Type:     "task",
		Status:   ticket.StatusPending,
		Priority: 3,
		Tags:     []string{"backend", "api"},
		Created:  "2026-01-15T10:00:00Z",
		Present: map[string]bool{
			"id":       true,
			"title":    true,
			"type":     true,
			"status":   true,
			"priority": true,
			"tags":     true,
			"created":  true,
		},
	}
}

// ─── MarshalTicket ────────────────────────────────────────────────────────────

func TestMarshalTicketBasic(t *testing.T) {
	tkt := simpleTicket()

	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket error: %v", err)
	}

	s := string(b)

	if !strings.HasPrefix(s, "---\n") {
		t.Errorf("output should start with ---\\n, got: %q", s[:min(len(s), 10)])
	}
	if !strings.Contains(s, "---\n") || strings.Count(s, "---\n") < 2 {
		t.Error("output should contain at least two ---\\n delimiters")
	}
	for _, want := range []string{"id: test-1234", "title: Round-trip ticket", "type: task", "status: pending"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

func TestMarshalTicketCanonicalOrdering(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:      "ord-0001",
		Title:   "Ordering test",
		Type:    "task",
		Status:  ticket.StatusOpen,
		Created: "2026-01-01T00:00:00Z",
		Present: map[string]bool{
			"id": true, "title": true, "type": true,
			"status": true, "created": true,
		},
	}

	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket error: %v", err)
	}

	s := string(b)
	idPos := strings.Index(s, "id:")
	titlePos := strings.Index(s, "title:")
	typePos := strings.Index(s, "type:")
	statusPos := strings.Index(s, "status:")

	if idPos > titlePos {
		t.Errorf("id: must come before title: (got id@%d, title@%d)", idPos, titlePos)
	}
	if titlePos > typePos {
		t.Errorf("title: must come before type: (got title@%d, type@%d)", titlePos, typePos)
	}
	if typePos > statusPos {
		t.Errorf("type: must come before status: (got type@%d, status@%d)", typePos, statusPos)
	}
}

// ─── UnmarshalTicket ─────────────────────────────────────────────────────────

func TestUnmarshalTicketBasic(t *testing.T) {
	input := "---\nid: abc-1234\ntitle: My ticket\ntype: task\nstatus: open\n---\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket error: %v", err)
	}

	if tkt.ID != "abc-1234" {
		t.Errorf("ID: got %q, want %q", tkt.ID, "abc-1234")
	}
	if tkt.Title != "My ticket" {
		t.Errorf("Title: got %q, want %q", tkt.Title, "My ticket")
	}
	if tkt.Type != "task" {
		t.Errorf("Type: got %q, want %q", tkt.Type, "task")
	}
	if tkt.Status != ticket.StatusOpen {
		t.Errorf("Status: got %q, want %q", tkt.Status, ticket.StatusOpen)
	}
}

func TestUnmarshalTicketPopulatesPresent(t *testing.T) {
	input := "---\nid: abc-1234\ntitle: My ticket\ntype: task\nstatus: open\npriority: 5\n---\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket error: %v", err)
	}

	for _, key := range []string{"id", "title", "type", "status", "priority"} {
		if !tkt.Present[key] {
			t.Errorf("Present[%q] should be true after unmarshal", key)
		}
	}

	// Fields not in the YAML should not be marked as present.
	for _, key := range []string{"deps", "tags", "assignee", "parent"} {
		if tkt.Present[key] {
			t.Errorf("Present[%q] should be false when key was absent from YAML", key)
		}
	}
}

func TestUnmarshalTicketExtractsTitleFromHeading(t *testing.T) {
	input := "---\nid: abc-1234\ntype: task\nstatus: open\n---\n\n# My Heading Title\n\nBody text here.\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket error: %v", err)
	}

	if tkt.Title != "My Heading Title" {
		t.Errorf("Title: got %q, want %q", tkt.Title, "My Heading Title")
	}
	if !tkt.TitleDerived {
		t.Error("TitleDerived should be true when title came from heading")
	}
	if tkt.Present["title"] {
		t.Error("Present[\"title\"] should be false when title was derived from heading")
	}
}

func TestUnmarshalTicketNoBody(t *testing.T) {
	input := "---\nid: abc-1234\ntitle: Direct title\ntype: task\nstatus: open\n---\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket error: %v", err)
	}

	if tkt.TitleDerived {
		t.Error("TitleDerived should be false when title is in frontmatter")
	}
	if tkt.Title != "Direct title" {
		t.Errorf("Title: got %q, want %q", tkt.Title, "Direct title")
	}
}

// ─── Round-trip idempotency (acceptance criterion) ────────────────────────────

func TestRoundTripIdempotency(t *testing.T) {
	original := simpleTicket()

	b1, err := markdown.MarshalTicket(original)
	if err != nil {
		t.Fatalf("first MarshalTicket: %v", err)
	}

	t2, err := markdown.UnmarshalTicket(b1)
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	b2, err := markdown.MarshalTicket(t2)
	if err != nil {
		t.Fatalf("second MarshalTicket: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Errorf("round-trip produced different bytes:\n--- first marshal ---\n%s\n--- second marshal ---\n%s", b1, b2)
	}
}

// TestRoundTripWithSlices verifies that slice fields (deps, tags) survive a
// marshal → unmarshal → re-marshal cycle byte-for-byte.
func TestRoundTripWithSlices(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:     "dep-0001",
		Title:  "Has deps",
		Type:   "task",
		Status: ticket.StatusPending,
		Deps:   []string{"dep-a", "dep-b"},
		Tags:   []string{"go", "yaml"},
		Present: map[string]bool{
			"id": true, "title": true, "type": true, "status": true,
			"deps": true, "tags": true,
		},
	}

	b1, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket: %v", err)
	}

	t2, err := markdown.UnmarshalTicket(b1)
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	b2, err := markdown.MarshalTicket(t2)
	if err != nil {
		t.Fatalf("second MarshalTicket: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Errorf("round-trip with slices produced different bytes:\n--- first ---\n%s\n--- second ---\n%s", b1, b2)
	}
}

// ─── Deterministic serialization (acceptance criterion) ───────────────────────

func TestDeterministicSerialization(t *testing.T) {
	tkt := simpleTicket()

	b1, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("first MarshalTicket: %v", err)
	}

	b2, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("second MarshalTicket: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Errorf("two calls to MarshalTicket produced different bytes:\n--- first ---\n%s\n--- second ---\n%s", b1, b2)
	}
}

// TestDeterministicExtraFields confirms that tickets with Extra map fields produce
// the same output on repeated calls (Extra keys sorted alphabetically).
func TestDeterministicExtraFields(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:     "extra-0001",
		Title:  "Extra fields",
		Type:   "task",
		Status: ticket.StatusOpen,
		Extra: map[string]interface{}{
			"zebra":  "last",
			"alpha":  "first",
			"middle": "mid",
		},
		Present: map[string]bool{
			"id": true, "title": true, "type": true, "status": true,
		},
	}

	b1, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("first MarshalTicket: %v", err)
	}

	b2, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("second MarshalTicket: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Error("MarshalTicket with Extra fields is not deterministic")
	}

	// Verify alphabetical ordering of Extra keys in output.
	s := string(b1)
	alphaPos := strings.Index(s, "alpha:")
	middlePos := strings.Index(s, "middle:")
	zebraPos := strings.Index(s, "zebra:")

	if alphaPos > middlePos || middlePos > zebraPos {
		t.Errorf("Extra keys not in alphabetical order: alpha@%d middle@%d zebra@%d", alphaPos, middlePos, zebraPos)
	}
}

// ─── Present-map controls field emission (acceptance criterion) ───────────────

func TestPresentMapControlsFieldEmission(t *testing.T) {
	// Priority is set to a non-zero value but NOT included in Present.
	tkt := &ticket.Ticket{
		ID:       "present-test",
		Title:    "Present map test",
		Type:     "task",
		Status:   ticket.StatusOpen,
		Priority: 99, // non-zero value that must NOT appear in output
		Present: map[string]bool{
			"id":     true,
			"title":  true,
			"type":   true,
			"status": true,
			// "priority" deliberately absent → Present["priority"] == false
		},
	}

	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket: %v", err)
	}

	s := string(b)
	if strings.Contains(s, "priority:") {
		t.Errorf("priority: must not appear in output when Present[\"priority\"] is false;\ngot:\n%s", s)
	}
	if !strings.Contains(s, "id: present-test") {
		t.Errorf("id: present-test must appear in output;\ngot:\n%s", s)
	}
}

// TestPresentMapExplicitFalse verifies that explicitly setting Present[key] = false
// suppresses the field, which is the same as the key being absent from the map.
func TestPresentMapExplicitFalse(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:     "false-test",
		Title:  "False present",
		Type:   "task",
		Status: ticket.StatusOpen,
		Tags:   []string{"should-not-appear"},
		Present: map[string]bool{
			"id": true, "title": true, "type": true, "status": true,
			"tags": false, // explicitly false
		},
	}

	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket: %v", err)
	}

	if strings.Contains(string(b), "tags:") {
		t.Errorf("tags: must not appear when Present[\"tags\"] == false")
	}
}

// ─── TitleDerived suppression (acceptance criterion) ─────────────────────────

func TestTitleDerivedNotWrittenToFrontmatter(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:           "derived-title",
		Title:        "Derived from heading",
		TitleDerived: true, // must suppress title: in frontmatter
		Present: map[string]bool{
			"id":    true,
			"title": true, // present, but TitleDerived overrides
		},
	}

	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket: %v", err)
	}

	s := string(b)
	if strings.Contains(s, "title:") {
		t.Errorf("title: must not appear in frontmatter when TitleDerived is true;\ngot:\n%s", s)
	}
	if !strings.Contains(s, "id: derived-title") {
		t.Errorf("id: must still be emitted;\ngot:\n%s", s)
	}
}

// ─── Unknown YAML fields (acceptance criterion) ───────────────────────────────

func TestPreserveUnknownYAMLFields(t *testing.T) {
	input := "---\nid: extra-abc\ntitle: Has extras\ntype: task\nstatus: open\ncustom_field: hello\nanother_extra: 42\n---\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	// Extra fields should be in the Extra map.
	if tkt.Extra["custom_field"] != "hello" {
		t.Errorf("Extra[\"custom_field\"]: got %v, want %q", tkt.Extra["custom_field"], "hello")
	}
	if tkt.Extra["another_extra"] != 42 {
		t.Errorf("Extra[\"another_extra\"]: got %v, want 42", tkt.Extra["another_extra"])
	}

	// Re-marshal must include the extra fields.
	b, err := markdown.MarshalTicket(tkt)
	if err != nil {
		t.Fatalf("MarshalTicket: %v", err)
	}
	s := string(b)

	if !strings.Contains(s, "custom_field: hello") {
		t.Errorf("custom_field not preserved in marshal output:\n%s", s)
	}
	if !strings.Contains(s, "another_extra: 42") {
		t.Errorf("another_extra not preserved in marshal output:\n%s", s)
	}

	// Second round-trip.
	tkt2, err := markdown.UnmarshalTicket(b)
	if err != nil {
		t.Fatalf("second UnmarshalTicket: %v", err)
	}
	if tkt2.Extra["custom_field"] != tkt.Extra["custom_field"] {
		t.Errorf("custom_field changed after second round-trip: got %v", tkt2.Extra["custom_field"])
	}
}

// ─── UpdateFrontmatter ────────────────────────────────────────────────────────

func TestUpdateFrontmatter(t *testing.T) {
	original := "---\nid: upd-0001\ntitle: Original\ntype: task\nstatus: open\n---\n\n## Notes\n\n- An existing note\n"

	tkt, err := markdown.UnmarshalTicket([]byte(original))
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	// Mutate the status.
	tkt.Status = ticket.StatusInProgress
	tkt.Present["status"] = true

	updated, err := markdown.UpdateFrontmatter([]byte(original), tkt)
	if err != nil {
		t.Fatalf("UpdateFrontmatter: %v", err)
	}

	s := string(updated)

	// Updated status must appear.
	if !strings.Contains(s, "status: in_progress") {
		t.Errorf("updated status not found in output:\n%s", s)
	}

	// Body must be preserved.
	if !strings.Contains(s, "An existing note") {
		t.Errorf("body not preserved by UpdateFrontmatter:\n%s", s)
	}
}

// ─── splitFrontmatterBody (indirect test via UnmarshalTicket) ─────────────────

func TestSplitFrontmatterBodyWithBody(t *testing.T) {
	input := "---\nid: split-test\ntitle: Test\ntype: task\nstatus: open\n---\n\nbody text here\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	if tkt.ID != "split-test" {
		t.Errorf("ID not parsed correctly: got %q", tkt.ID)
	}
}

func TestSplitFrontmatterBodyNoFrontmatter(t *testing.T) {
	input := "Just plain body text without any frontmatter."

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}

	// All fields default to zero values.
	if tkt.ID != "" {
		t.Errorf("ID should be empty for no-frontmatter input, got %q", tkt.ID)
	}
}

func TestSplitFrontmatterBodyEmptyFrontmatter(t *testing.T) {
	input := "---\n---\nbody only\n"

	tkt, err := markdown.UnmarshalTicket([]byte(input))
	if err != nil {
		t.Fatalf("UnmarshalTicket: %v", err)
	}
	if len(tkt.Present) != 0 {
		t.Errorf("Present should be empty for empty frontmatter, got %v", tkt.Present)
	}
}

// ─── MarshalYAML (direct) ─────────────────────────────────────────────────────

func TestMarshalYAMLReturnsMappingNode(t *testing.T) {
	tkt := &ticket.Ticket{
		ID:     "node-test",
		Title:  "Node test",
		Type:   "task",
		Status: ticket.StatusOpen,
		Present: map[string]bool{
			"id": true, "title": true, "type": true, "status": true,
		},
	}

	node, err := markdown.MarshalYAML(tkt)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	if node == nil {
		t.Fatal("MarshalYAML returned nil node")
	}
	if node.Kind != 4 { // yaml.MappingNode == 4
		t.Errorf("expected MappingNode (kind 4), got kind %d", node.Kind)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
