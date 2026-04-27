package markdown_test

import (
	"strings"
	"testing"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/markdown"
)

// ─── extractHeadingTitle ──────────────────────────────────────────────────────

func TestExtractHeadingTitle(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantTitle string
		wantOK    bool
	}{
		{
			name:      "simple heading",
			body:      "# My Title\n\nsome body",
			wantTitle: "My Title",
			wantOK:    true,
		},
		{
			name:      "heading with leading blank lines",
			body:      "\n\n# Padded Title\n\nbody",
			wantTitle: "Padded Title",
			wantOK:    true,
		},
		{
			name:      "H2 is not extracted",
			body:      "## Not a title\n\nbody",
			wantTitle: "",
			wantOK:    false,
		},
		{
			name:      "no heading at all",
			body:      "Just some body text.\n",
			wantTitle: "",
			wantOK:    false,
		},
		{
			name:      "H1 with extra whitespace around title",
			body:      "#   Spaced Title   \n",
			wantTitle: "Spaced Title",
			wantOK:    true,
		},
		{
			name:      "H1 pound sign followed by non-space is not a heading",
			body:      "#NoSpace\nrest",
			wantTitle: "",
			wantOK:    false,
		},
		{
			name:      "empty body",
			body:      "",
			wantTitle: "",
			wantOK:    false,
		},
		{
			name:      "first H1 is returned when multiple exist",
			body:      "# First\n\n# Second\n",
			wantTitle: "First",
			wantOK:    true,
		},
		{
			name:      "empty heading text after # is skipped",
			body:      "# \n\n# Real Title\n",
			wantTitle: "Real Title",
			wantOK:    true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := markdown.ExtractHeadingTitle(tc.body)
			if ok != tc.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantTitle {
				t.Errorf("title: got %q, want %q", got, tc.wantTitle)
			}
		})
	}
}

// ─── AddNote ─────────────────────────────────────────────────────────────────

func TestAddNoteCreatesNotesSection(t *testing.T) {
	body := "Some description text.\n"
	result := markdown.AddNote(body, "My first note")

	if !strings.Contains(result, "## Notes") {
		t.Error("AddNote should create a ## Notes section when none exists")
	}
	if !strings.Contains(result, "My first note") {
		t.Error("AddNote should include the note text")
	}
	if !strings.Contains(result, "- ") {
		t.Error("AddNote should format the note as a list item")
	}
}

func TestAddNoteAppendsToExistingSection(t *testing.T) {
	body := "Description.\n\n## Notes\n\n- 2026-01-01T00:00:00Z: First note\n"
	result := markdown.AddNote(body, "Second note")

	if !strings.Contains(result, "First note") {
		t.Error("AddNote should preserve existing notes")
	}
	if !strings.Contains(result, "Second note") {
		t.Error("AddNote should include the new note text")
	}

	// The notes section must appear only once.
	count := strings.Count(result, "## Notes")
	if count != 1 {
		t.Errorf("## Notes section count: got %d, want 1", count)
	}
}

func TestAddNotePreservesBodyBeforeNotes(t *testing.T) {
	body := "## Description\n\nSome description.\n\n## Notes\n\n- old note\n"
	result := markdown.AddNote(body, "new note")

	if !strings.Contains(result, "## Description") {
		t.Error("## Description section must be preserved")
	}
	if !strings.Contains(result, "Some description.") {
		t.Error("description text must be preserved")
	}
	if !strings.Contains(result, "old note") {
		t.Error("old note must be preserved")
	}
	if !strings.Contains(result, "new note") {
		t.Error("new note must appear")
	}
}

func TestAddNotePreservesSectionsAfterNotes(t *testing.T) {
	body := "## Notes\n\n- existing\n\n## Acceptance Criteria\n\n- criterion\n"
	result := markdown.AddNote(body, "new note")

	if !strings.Contains(result, "## Acceptance Criteria") {
		t.Error("section after Notes must be preserved")
	}
	if !strings.Contains(result, "- criterion") {
		t.Error("content after Notes must be preserved")
	}

	// The new note must appear BEFORE the Acceptance Criteria section.
	newNotePos := strings.Index(result, "new note")
	criteriaPos := strings.Index(result, "## Acceptance Criteria")
	if newNotePos >= criteriaPos {
		t.Error("new note must appear before the Acceptance Criteria section")
	}
}

func TestAddNoteEmptyBody(t *testing.T) {
	result := markdown.AddNote("", "Note in empty body")

	if !strings.Contains(result, "## Notes") {
		t.Error("## Notes section must be created for empty body")
	}
	if !strings.Contains(result, "Note in empty body") {
		t.Error("note text must appear")
	}
}

// ─── RenderSections ───────────────────────────────────────────────────────────

func TestRenderSectionsEmpty(t *testing.T) {
	tk := &ticket.Ticket{}
	result := markdown.RenderSections(tk)
	if result != "" {
		t.Errorf("RenderSections on empty ticket should return empty string, got %q", result)
	}
}

func TestRenderSectionsDescriptionOnly(t *testing.T) {
	tk := &ticket.Ticket{Description: "Just a description."}
	result := markdown.RenderSections(tk)
	if !strings.Contains(result, "Just a description.") {
		t.Errorf("RenderSections should include Description, got %q", result)
	}
	if strings.Contains(result, "##") {
		t.Errorf("RenderSections with only Description should produce no headings, got %q", result)
	}
}

func TestRenderSectionsAcceptanceCriteria(t *testing.T) {
	tk := &ticket.Ticket{
		AcceptanceCriteria: []string{"criterion one", "criterion two"},
	}
	result := markdown.RenderSections(tk)
	if !strings.Contains(result, "## Acceptance criteria") {
		t.Errorf("RenderSections should include '## Acceptance criteria' heading, got %q", result)
	}
	if !strings.Contains(result, "- criterion one") {
		t.Errorf("RenderSections should list criterion one, got %q", result)
	}
	if !strings.Contains(result, "- criterion two") {
		t.Errorf("RenderSections should list criterion two, got %q", result)
	}
}

func TestRenderSectionsValidationCommands(t *testing.T) {
	tk := &ticket.Ticket{
		ValidationCommands: []string{"go test ./...", "go vet ./..."},
	}
	result := markdown.RenderSections(tk)
	if !strings.Contains(result, "## Validation") {
		t.Errorf("RenderSections should include '## Validation' heading, got %q", result)
	}
	if !strings.Contains(result, "```bash") {
		t.Errorf("RenderSections should wrap commands in a bash code block, got %q", result)
	}
	if !strings.Contains(result, "go test ./...") {
		t.Errorf("RenderSections should include first command, got %q", result)
	}
}

func TestRenderSectionsNotes(t *testing.T) {
	tk := &ticket.Ticket{Notes: []string{"note alpha", "note beta"}}
	result := markdown.RenderSections(tk)
	if !strings.Contains(result, "## Notes") {
		t.Errorf("RenderSections should include '## Notes' heading, got %q", result)
	}
	if !strings.Contains(result, "- note alpha") {
		t.Errorf("RenderSections should list note alpha, got %q", result)
	}
}

func TestRenderSectionsOrdering(t *testing.T) {
	tk := &ticket.Ticket{
		Description:        "preamble",
		AcceptanceCriteria: []string{"ac item"},
		ValidationCommands: []string{"go test ./..."},
		Notes:              []string{"a note"},
	}
	result := markdown.RenderSections(tk)
	preamblePos := strings.Index(result, "preamble")
	acPos := strings.Index(result, "## Acceptance criteria")
	validationPos := strings.Index(result, "## Validation")
	notesPos := strings.Index(result, "## Notes")

	if preamblePos >= acPos {
		t.Error("Description paragraph should appear before ## Acceptance criteria")
	}
	if acPos >= validationPos {
		t.Error("## Acceptance criteria should appear before ## Validation")
	}
	if validationPos >= notesPos {
		t.Error("## Validation should appear before ## Notes")
	}
}

func TestRenderSectionsNoSpuriousHeadings(t *testing.T) {
	// Only Description set — no headings should appear.
	tk := &ticket.Ticket{Description: "just text"}
	result := markdown.RenderSections(tk)
	if strings.Contains(result, "##") {
		t.Errorf("no headings expected when only Description is set, got %q", result)
	}
}
