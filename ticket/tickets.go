// Package ticket defines the canonical Ticket type and related core types
// used by the epos shared ticket system.
package ticket

// Status represents a ticket's lifecycle state.
type Status string

const (
	// StatusOpen is the tk-native open state.
	StatusOpen Status = "open"
	// StatusReady is a tk-native synonym for open (ticket is ready to work).
	StatusReady Status = "ready"
	// StatusInProgress is the tk-native state for a ticket being actively worked.
	StatusInProgress Status = "in_progress"
	// StatusBlocked is the tk-native state for a ticket blocked by unresolved dependencies.
	// For operator-initiated blocking, use StatusHeld.
	StatusBlocked Status = "blocked"
	// StatusClosed is the tk-native closed/done state.
	StatusClosed Status = "closed"
)

const (
	// StatusPending is the extended state for a ticket awaiting assignment.
	StatusPending Status = "pending"
	// StatusClaimed is the extended state for a ticket claimed by an agent.
	StatusClaimed Status = "claimed"
	// StatusImplementing is the extended state for a ticket under active implementation.
	StatusImplementing Status = "implementing"
	// StatusVerifying is the extended state for post-implementation verification.
	StatusVerifying Status = "verifying"
	// StatusUnderReview is the extended state for a ticket awaiting human review.
	StatusUnderReview Status = "under_review"
	// StatusRepairPending is the extended state for a ticket that failed review and needs repair.
	StatusRepairPending Status = "repair_pending"
	// StatusHeld is the extended state for operator-initiated blocking.
	// Unlike StatusBlocked (dependency-driven), StatusHeld reflects an explicit operator decision.
	StatusHeld Status = "held"
	// StatusDone is the extended state for a successfully completed ticket.
	StatusDone Status = "done"
	// StatusFailed is the extended state for a ticket that could not be completed.
	StatusFailed Status = "failed"
)

// validTicketTypes is the set of allowed values for Ticket.Type.
var validTicketTypes = map[string]bool{
	"epic":    true,
	"task":    true,
	"issue":   true,
	"feature": true,
	"bug":     true,
	"chore":   true,
	"spike":   true,
	"doc":     true,
}

// TaskScope defines the filesystem paths in scope for a ticket's implementation.
type TaskScope struct {
	OwnedPaths    []string `yaml:"owned_paths"     json:"owned_paths,omitempty"`
	ReadOnlyPaths []string `yaml:"read_only_paths" json:"read_only_paths,omitempty"`
	SharedPaths   []string `yaml:"shared_paths"    json:"shared_paths,omitempty"`
	IsolationMode string   `yaml:"isolation_mode"  json:"isolation_mode,omitempty"`
}

// FileChange describes a single file change associated with a ticket.
type FileChange struct {
	Path   string `yaml:"path"   json:"path"`
	Change string `yaml:"change" json:"change"`
	Reason string `yaml:"reason" json:"reason,omitempty"`
}

// ImplementationDetail holds structured guidance for implementing a ticket.
type ImplementationDetail struct {
	Approach string       `yaml:"approach" json:"approach,omitempty"`
	Files    []FileChange `yaml:"files"    json:"files,omitempty"`
	Notes    string       `yaml:"notes"    json:"notes,omitempty"`
}

// ValidationCheck defines an automated check that verifies an acceptance criterion.
type ValidationCheck struct {
	Command     string `yaml:"command"     json:"command"`
	Expected    string `yaml:"expected"    json:"expected,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
}

// LearningRef is a reference to a learning artifact such as a research doc or pattern.
type LearningRef struct {
	ID    string `yaml:"id"    json:"id"`
	Type  string `yaml:"type"  json:"type,omitempty"`
	Title string `yaml:"title" json:"title,omitempty"`
}

// Ticket is the canonical shared ticket type for the epos system.
// It merges the planning, execution, and metadata facets of fabrikk and verk tickets.
//
// The Present map and TitleDerived flag support round-trip idempotency with the
// tk CLI: Present tracks which frontmatter fields were explicitly set, and
// TitleDerived indicates when the title was extracted from a Markdown body heading.
type Ticket struct {
	// Core graph
	ID          string   `yaml:"id"          json:"id"`
	Title       string   `yaml:"title"       json:"title"`
	Type        string   `yaml:"type"        json:"type"`
	Status      Status   `yaml:"status"      json:"status"`
	Parent      string   `yaml:"parent"      json:"parent,omitempty"`
	Deps        []string `yaml:"deps"        json:"deps,omitempty"`
	Priority    int      `yaml:"priority"    json:"priority"`
	Tags        []string `yaml:"tags"        json:"tags,omitempty"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Notes       []string `yaml:"notes"       json:"notes,omitempty"`

	// Planning facet (authored by fabrikk)
	RequirementIDs       []string             `yaml:"requirement_ids"        json:"requirement_ids,omitempty"`
	SourceRefs           []string             `yaml:"source_refs"            json:"source_refs,omitempty"`
	LineageID            string               `yaml:"lineage_id"             json:"lineage_id,omitempty"`
	RiskLevel            string               `yaml:"risk_level"             json:"risk_level,omitempty"`
	Intent               string               `yaml:"intent"                 json:"intent,omitempty"`
	Constraints          []string             `yaml:"constraints"            json:"constraints,omitempty"`
	Warnings             []string             `yaml:"warnings"               json:"warnings,omitempty"`
	Scope                TaskScope            `yaml:",inline"                json:"scope,omitempty"`
	FilesLikelyTouched   []string             `yaml:"files_likely_touched"   json:"files_likely_touched,omitempty"`
	ImplementationDetail ImplementationDetail `yaml:"implementation_detail"  json:"implementation_detail,omitempty"`
	LearningContext      []LearningRef        `yaml:"learning_context"       json:"learning_context,omitempty"`

	// Static execution facet (authored by fabrikk, consumed by verk)
	AcceptanceCriteria []string          `yaml:"acceptance_criteria" json:"acceptance_criteria,omitempty"`
	TestCases          []string          `yaml:"test_cases"          json:"test_cases,omitempty"`
	ValidationCommands []string          `yaml:"validation_commands" json:"validation_commands,omitempty"`
	ValidationChecks   []ValidationCheck `yaml:"validation_checks"   json:"validation_checks,omitempty"`
	ReviewThreshold    string            `yaml:"review_threshold"    json:"review_threshold,omitempty"`
	RuntimePreference  string            `yaml:"runtime"             json:"runtime,omitempty"`
	RequiredEvidence   []string          `yaml:"required_evidence"   json:"required_evidence,omitempty"`
	ReviewerGuidance   string            `yaml:"reviewer_guidance"   json:"reviewer_guidance,omitempty"`

	// Metadata
	Created     string `yaml:"created"      json:"created"`
	UpdatedAt   string `yaml:"updated_at"   json:"updated_at,omitempty"`
	Assignee    string `yaml:"assignee"     json:"assignee,omitempty"`
	ETag        string `yaml:"etag"         json:"etag,omitempty"`
	CreatedFrom string `yaml:"created_from" json:"created_from,omitempty"`
	Order       int    `yaml:"order"        json:"order"`

	// Grouping (fabrikk-specific; may generalize in future phases)
	GroupingReason        string   `yaml:"grouping_reason"         json:"grouping_reason,omitempty"`
	GroupedRequirementIDs []string `yaml:"grouped_requirement_ids" json:"grouped_requirement_ids,omitempty"`

	// Links holds symmetric ticket relationships (fabrikk compatible).
	// Link/Unlink store methods operate on this field.
	// Per D7, links are preserved for data round-trip but excluded from MVP dependency semantics.
	Links []string `yaml:"links,omitempty" json:"links,omitempty"`

	// Extended status maps to the fabrikk execution model (see Status constants).
	ExtendedStatus string `yaml:"extended_status" json:"extended_status,omitempty"`

	// StatusReason is a human-readable explanation for the current status.
	StatusReason string `yaml:"status_reason" json:"status_reason,omitempty"`

	// Extra captures unknown YAML fields for forward compatibility.
	// Fields not matched by any named struct field are stored here and round-tripped.
	Extra map[string]any `yaml:",inline" json:"-"`

	// Present tracks which fields were explicitly set in the ticket's frontmatter.
	// It is critical for round-trip idempotency with the tk CLI: absent fields are not
	// re-emitted during marshal, preventing noisy diffs.
	Present map[string]bool `yaml:"-" json:"-"`

	// TitleDerived is true when the title was extracted from a Markdown heading in the body.
	// When true, MarshalTicket suppresses the title: frontmatter key to avoid duplication.
	TitleDerived bool `yaml:"-" json:"-"`
}

// TicketOption is a functional option for configuring a Ticket via NewTicket.
type TicketOption func(*Ticket) //nolint:revive // stutter is intentional: TicketOption is the canonical name used at call sites across packages

// WithTitle sets the Ticket title and marks the field as present in frontmatter.
func WithTitle(title string) TicketOption {
	return func(t *Ticket) {
		t.Title = title
		t.Present["title"] = true
	}
}

// WithType sets the Ticket type and marks the field as present in frontmatter.
func WithType(typ string) TicketOption {
	return func(t *Ticket) {
		t.Type = typ
		t.Present["type"] = true
	}
}

// WithStatus sets the Ticket status and marks the field as present in frontmatter.
func WithStatus(s Status) TicketOption {
	return func(t *Ticket) {
		t.Status = s
		t.Present["status"] = true
	}
}

// WithPriority sets the Ticket priority and marks the field as present in frontmatter.
func WithPriority(priority int) TicketOption {
	return func(t *Ticket) {
		t.Priority = priority
		t.Present["priority"] = true
	}
}

// WithParent sets the Ticket parent ID and marks the field as present in frontmatter.
func WithParent(parent string) TicketOption {
	return func(t *Ticket) {
		t.Parent = parent
		t.Present["parent"] = true
	}
}

// WithDeps sets the Ticket dependency IDs and marks the field as present in frontmatter.
func WithDeps(deps ...string) TicketOption {
	return func(t *Ticket) {
		t.Deps = deps
		t.Present["deps"] = true
	}
}

// NewTicket creates a Ticket with sensible defaults and an initialized Present map.
// Use TicketOption functions to set fields; each option marks the corresponding field
// as present so the marshal layer knows to emit it.
func NewTicket(opts ...TicketOption) *Ticket {
	t := &Ticket{
		Present:        make(map[string]bool),
		ExtendedStatus: "open",
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// WithDescription sets the Ticket description (narrative preamble) and marks the field as present.
func WithDescription(desc string) TicketOption {
	return func(t *Ticket) {
		t.Description = desc
		t.Present["description"] = true
	}
}

// WithAssignee sets the Ticket assignee and marks the field as present in frontmatter.
func WithAssignee(name string) TicketOption {
	return func(t *Ticket) {
		t.Assignee = name
		t.Present["assignee"] = true
	}
}

// WithTags sets the Ticket tags and marks the field as present in frontmatter.
func WithTags(tags ...string) TicketOption {
	return func(t *Ticket) {
		t.Tags = tags
		t.Present["tags"] = true
	}
}

// WithAcceptanceCriteria sets the Ticket acceptance criteria and marks the field as present.
func WithAcceptanceCriteria(items ...string) TicketOption {
	return func(t *Ticket) {
		t.AcceptanceCriteria = items
		t.Present["acceptance_criteria"] = true
	}
}

// WithNotes sets the Ticket notes and marks the field as present.
func WithNotes(notes ...string) TicketOption {
	return func(t *Ticket) {
		t.Notes = notes
		t.Present["notes"] = true
	}
}

// WithIntent sets the Ticket intent and marks the field as present in frontmatter.
func WithIntent(text string) TicketOption {
	return func(t *Ticket) {
		t.Intent = text
		t.Present["intent"] = true
	}
}
