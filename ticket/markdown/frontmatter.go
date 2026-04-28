package markdown

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/php-workx/epos/ticket"
	"gopkg.in/yaml.v3"
)

// nodeBuilder accumulates YAML mapping entries and defers error propagation so
// that MarshalYAML can call emit/add without wrapping every call in an if-err
// block. This keeps cognitive complexity well below the project limit.
type nodeBuilder struct {
	t    *ticket.Ticket
	node *yaml.Node
	err  error
}

// emit conditionally appends a key/value pair, respecting Present and TitleDerived.
func (b *nodeBuilder) emit(key string, v interface{}) {
	if b.err != nil || !shouldEmit(b.t, key) {
		return
	}
	b.add(key, v)
}

// add unconditionally appends a key/value pair.
func (b *nodeBuilder) add(key string, v interface{}) {
	if b.err != nil {
		return
	}
	valNode, err := valueToNode(v)
	if err != nil {
		b.err = fmt.Errorf("marshal field %q: %w", key, err)
		return
	}
	b.node.Content = append(b.node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		valNode,
	)
}

// MarshalYAML converts a Ticket to a canonical *yaml.Node suitable for YAML encoding.
//
// The node observes three invariants that together ensure stable, diff-friendly output:
//
//  1. Canonical field ordering: fields are emitted in the order they appear in the
//     Ticket struct definition, with Extra keys sorted alphabetically after all named fields.
//  2. Present-map-aware emission: when t.Present is non-nil, only fields with
//     t.Present[yamlKey] == true are included. Fields absent from the original frontmatter
//     are silently omitted, preventing noisy diffs. When t.Present is nil every field is
//     included unconditionally (useful for manually constructed Ticket values).
//  3. TitleDerived suppression: when t.TitleDerived is true the "title" key is always
//     omitted from the YAML, because the title lives in the Markdown body heading instead.
func MarshalYAML(t *ticket.Ticket) (*yaml.Node, error) {
	b := &nodeBuilder{
		t:    t,
		node: &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
	}

	// ── Core graph ────────────────────────────────────────────────────────────
	b.emit("id", t.ID)
	// title is suppressed when it was derived from a Markdown body heading.
	if !t.TitleDerived {
		b.emit("title", t.Title)
	}
	b.emit("type", t.Type)
	b.emit("status", string(t.Status))
	b.emit("parent", t.Parent)
	b.emit("deps", t.Deps)
	b.emit("priority", t.Priority)
	b.emit("tags", t.Tags)
	b.emit("description", t.Description)
	b.emit("notes", t.Notes)

	// ── Planning facet ────────────────────────────────────────────────────────
	b.emit("requirement_ids", t.RequirementIDs)
	b.emit("source_refs", t.SourceRefs)
	b.emit("lineage_id", t.LineageID)
	b.emit("risk_level", t.RiskLevel)
	b.emit("intent", t.Intent)
	b.emit("constraints", t.Constraints)
	b.emit("warnings", t.Warnings)

	// TaskScope is inline in the struct; its fields are emitted at the top level.
	b.emit("owned_paths", t.Scope.OwnedPaths)
	b.emit("read_only_paths", t.Scope.ReadOnlyPaths)
	b.emit("shared_paths", t.Scope.SharedPaths)
	b.emit("isolation_mode", t.Scope.IsolationMode)

	b.emit("files_likely_touched", t.FilesLikelyTouched)
	b.emit("implementation_detail", t.ImplementationDetail)
	b.emit("learning_context", t.LearningContext)

	// ── Execution facet ───────────────────────────────────────────────────────
	b.emit("acceptance_criteria", t.AcceptanceCriteria)
	b.emit("test_cases", t.TestCases)
	b.emit("validation_commands", t.ValidationCommands)
	b.emit("validation_checks", t.ValidationChecks)
	b.emit("review_threshold", t.ReviewThreshold)
	b.emit("runtime", t.RuntimePreference)
	b.emit("required_evidence", t.RequiredEvidence)
	b.emit("reviewer_guidance", t.ReviewerGuidance)

	// ── Metadata ──────────────────────────────────────────────────────────────
	b.emit("created", t.Created)
	b.emit("updated_at", t.UpdatedAt)
	b.emit("assignee", t.Assignee)
	b.emit("etag", t.ETag)
	b.emit("created_from", t.CreatedFrom)
	b.emit("order", t.Order)

	// ── Grouping ──────────────────────────────────────────────────────────────
	b.emit("grouping_reason", t.GroupingReason)
	b.emit("grouped_requirement_ids", t.GroupedRequirementIDs)

	// ── Links ─────────────────────────────────────────────────────────────────
	b.emit("links", t.Links)

	// ── Extended status ───────────────────────────────────────────────────────
	b.emit("extended_status", t.ExtendedStatus)
	b.emit("status_reason", t.StatusReason)

	// ── Extra (unknown fields) — sorted for determinism ───────────────────────
	if len(t.Extra) > 0 {
		keys := make([]string, 0, len(t.Extra))
		for k := range t.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.add(k, t.Extra[k])
		}
	}

	if b.err != nil {
		return nil, fmt.Errorf("MarshalYAML: %w", b.err)
	}
	return b.node, nil
}

// MarshalTicket serializes t to tk-compatible Markdown with YAML frontmatter.
// The output format is:
//
//	---
//	<yaml frontmatter>
//	---
//
// Fields are emitted in canonical order and controlled by t.Present. Unknown fields
// stored in t.Extra are appended after all named fields, sorted alphabetically.
func MarshalTicket(t *ticket.Ticket) ([]byte, error) {
	node, err := MarshalYAML(t)
	if err != nil {
		return nil, fmt.Errorf("MarshalTicket: %w", err)
	}

	yamlBytes, err := yaml.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("MarshalTicket: marshal node: %w", err)
	}

	return renderFrontmatterBody(yamlBytes, RenderSections(t)), nil
}

// UnmarshalTicket parses Markdown bytes with YAML frontmatter into a Ticket.
//
// It performs three key actions beyond basic YAML decoding:
//  1. Populates t.Present with every key found in the frontmatter, so that the
//     marshal layer knows which fields to re-emit on round-trip.
//  2. Sets t.TitleDerived = true when no "title" key is present in the frontmatter
//     but a top-level "# Heading" is found in the body.
//  3. Stores unknown YAML keys in t.Extra via the yaml:",inline" tag.
func UnmarshalTicket(data []byte) (*ticket.Ticket, error) {
	fm, body := splitFrontmatterBody(data)

	// ── Pass 1: collect all top-level keys for the Present map ───────────────

	var rawMap map[string]interface{}
	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &rawMap); err != nil {
			return nil, &ticket.CorruptYAMLError{
				Path:  "<input>",
				Cause: err,
			}
		}
	}

	t := &ticket.Ticket{
		Present: make(map[string]bool, len(rawMap)),
	}
	for k := range rawMap {
		t.Present[k] = true
	}

	// ── Pass 2: decode into the Ticket struct (inline fields handled by yaml.v3) ─

	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), t); err != nil {
			return nil, &ticket.CorruptYAMLError{
				Path:  "<input>",
				Cause: err,
			}
		}
	}

	// ── TitleDerived: extract title from the Markdown body heading if absent ──

	if !t.Present["title"] {
		if heading, ok := ExtractHeadingTitle(body); ok {
			t.Title = heading
			t.TitleDerived = true
		}
	}

	return t, nil
}

// UpdateFrontmatter replaces the YAML frontmatter in existing Markdown bytes
// with a freshly marshaled version of t, preserving the document body verbatim.
// Unknown frontmatter fields stored in t.Extra survive via the round-trip.
func UpdateFrontmatter(existing []byte, t *ticket.Ticket) ([]byte, error) {
	_, body := splitFrontmatterBody(existing)

	node, err := MarshalYAML(t)
	if err != nil {
		return nil, fmt.Errorf("UpdateFrontmatter: %w", err)
	}

	yamlBytes, err := yaml.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("UpdateFrontmatter: marshal node: %w", err)
	}

	return renderFrontmatterBody(yamlBytes, body), nil
}

// UpdateBody returns existing with its Markdown body replaced by transform(body).
// The YAML frontmatter is preserved verbatim. If existing has no frontmatter,
// the entire content is treated as body.
func UpdateBody(existing []byte, transform func(string) string) []byte {
	fm, body := splitFrontmatterBody(existing)
	newBody := transform(body)
	var buf bytes.Buffer
	if fm != "" {
		buf.WriteString("---\n")
		buf.WriteString(fm)
		buf.WriteString("---\n")
	}
	buf.WriteString(newBody)
	return buf.Bytes()
}

// ─── private helpers ──────────────────────────────────────────────────────────

// splitFrontmatterBody splits a Markdown document into its YAML frontmatter and
// body. Frontmatter must be delimited by "---" on its own line at the start and
// end of the block. The returned frontmatter string includes a trailing newline;
// the body string is everything after the closing "---\n" delimiter (may be empty).
//
// If the document does not start with "---\n" the entire content is returned as
// the body and the frontmatter is empty.
func splitFrontmatterBody(data []byte) (frontmatter, body string) {
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}

	// The content after the opening "---\n".
	rest := content[4:]

	// Handle empty frontmatter: the opening "---\n" is immediately followed by the
	// closing delimiter, with no frontmatter content in between.
	if strings.HasPrefix(rest, "---\n") {
		return "", rest[4:]
	}
	if rest == "---" {
		return "", ""
	}

	// Look for the closing delimiter: a newline followed by "---" and another newline
	// (or end of string).
	const closingNL = "\n---\n"

	if idx := strings.Index(rest, closingNL); idx != -1 {
		// frontmatter content ends at idx; +1 keeps the trailing newline.
		return rest[:idx+1], rest[idx+len(closingNL):]
	}

	// Handle "---" at very end of file with no trailing newline.
	const closingEOF = "\n---"
	if idx := strings.Index(rest, closingEOF); idx != -1 && idx+len(closingEOF) == len(rest) {
		return rest[:idx+1], ""
	}

	// No closing delimiter — treat everything as frontmatter.
	return rest, ""
}

// renderFrontmatterBody assembles a complete Markdown document from yamlBytes
// (the raw YAML content, already ending with a newline) and an optional body string.
func renderFrontmatterBody(yamlBytes []byte, body string) []byte {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString(body)
	}
	return buf.Bytes()
}

// shouldEmit reports whether a field with the given YAML key should be included
// in the marshaled output.
//
//   - When t.Present is nil, all fields are included unconditionally.
//   - When t.Present is non-nil (the normal case after NewTicket or UnmarshalTicket),
//     only keys with t.Present[key] == true are included.
//   - "title" is always suppressed when t.TitleDerived is true, regardless of Present.
func shouldEmit(t *ticket.Ticket, key string) bool {
	// TitleDerived overrides Present for the title key.
	if key == "title" && t.TitleDerived {
		return false
	}
	if t.Present == nil {
		return true
	}
	return t.Present[key]
}

// valueToNode converts an arbitrary Go value into a *yaml.Node for embedding in
// a manually constructed YAML mapping. It round-trips through yaml.Marshal /
// yaml.Unmarshal to ensure correct tag and style handling.
func valueToNode(v interface{}) (*yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}
