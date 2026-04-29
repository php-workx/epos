package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/php-workx/epos/ticket"
	"github.com/spf13/cobra"
)

var (
	editPriority int
	editParent   string
	editDeps     []string
	editBody     string
	editBodyFile string
	editAC       []string
	editNotes    []string
	editAssignee string
	editTags     []string
	editIntent   string
	editStdin    bool
)

// editTicketSpec is the JSON input schema for epos edit --stdin.
type editTicketSpec struct {
	Priority           *int            `json:"priority"`
	Parent             string          `json:"parent"`
	Deps               []string        `json:"deps"`
	Body               string          `json:"body"`
	AcceptanceCriteria []string        `json:"acceptance_criteria"`
	Notes              []string        `json:"notes"`
	Assignee           string          `json:"assignee"`
	Tags               []string        `json:"tags"`
	Intent             string          `json:"intent"`
	Present            map[string]bool `json:"-"`
}

func (s *editTicketSpec) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type editTicketSpecJSON struct {
		Priority           *int     `json:"priority"`
		Parent             string   `json:"parent"`
		Deps               []string `json:"deps"`
		Body               string   `json:"body"`
		AcceptanceCriteria []string `json:"acceptance_criteria"`
		Notes              []string `json:"notes"`
		Assignee           string   `json:"assignee"`
		Tags               []string `json:"tags"`
		Intent             string   `json:"intent"`
	}
	var decoded editTicketSpecJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*s = editTicketSpec{
		Priority:           decoded.Priority,
		Parent:             decoded.Parent,
		Deps:               decoded.Deps,
		Body:               decoded.Body,
		AcceptanceCriteria: decoded.AcceptanceCriteria,
		Notes:              decoded.Notes,
		Assignee:           decoded.Assignee,
		Tags:               decoded.Tags,
		Intent:             decoded.Intent,
		Present:            make(map[string]bool, len(raw)),
	}
	for field := range raw {
		switch field {
		case "priority", "parent", "deps", "body", "acceptance_criteria", "notes", "assignee", "tags", "intent":
			s.Present[field] = true
		}
	}
	return nil
}

func (s *editTicketSpec) markPresent(field string) {
	if s.Present == nil {
		s.Present = make(map[string]bool)
	}
	s.Present[field] = true
}

func (s *editTicketSpec) has(field string) bool {
	return s.Present != nil && s.Present[field]
}

func validateEditSpec(spec *editTicketSpec) error {
	if spec.Priority != nil && *spec.Priority < 0 {
		return &ticket.ValidationError{Field: "priority", Message: "must be >= 0"}
	}
	seen := make(map[string]bool, len(spec.Tags))
	for i, tag := range spec.Tags {
		if strings.TrimSpace(tag) == "" {
			return &ticket.ValidationError{Field: "tags", Message: fmt.Sprintf("item %d is empty", i)}
		}
		if seen[tag] {
			return &ticket.ValidationError{Field: "tags", Message: fmt.Sprintf("duplicate tag %q", tag)}
		}
		seen[tag] = true
	}
	for i, ac := range spec.AcceptanceCriteria {
		if strings.TrimSpace(ac) == "" {
			return &ticket.ValidationError{Field: "acceptance_criteria", Message: fmt.Sprintf("item %d is empty", i)}
		}
	}
	return nil
}

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit fields of an existing ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		_, tk, err := resolveTicketID(s, args[0])
		if err != nil {
			return err
		}

		spec := &editTicketSpec{}

		if editStdin {
			if err := json.NewDecoder(os.Stdin).Decode(spec); err != nil {
				return &ticket.ValidationError{Field: "stdin", Message: fmt.Sprintf("invalid JSON: %s", err)}
			}
		}

		// CLI flags override stdin values; only apply flags that were explicitly set.
		if cmd.Flags().Changed("priority") {
			spec.Priority = &editPriority
			spec.markPresent("priority")
		}
		if cmd.Flags().Changed("parent") {
			spec.Parent = editParent
			spec.markPresent("parent")
		}
		if cmd.Flags().Changed("deps") {
			spec.Deps = editDeps
			spec.markPresent("deps")
		}
		if cmd.Flags().Changed("body") {
			spec.Body = editBody
			spec.markPresent("body")
		}
		if cmd.Flags().Changed("body-file") {
			data, err := os.ReadFile(editBodyFile) //nolint:gosec // G304: path is the user's explicit --body-file argument
			if err != nil {
				return &ticket.ValidationError{Field: "body-file", Message: err.Error()}
			}
			spec.Body = string(data)
			spec.markPresent("body")
		}
		if cmd.Flags().Changed("ac") {
			spec.AcceptanceCriteria = editAC
			spec.markPresent("acceptance_criteria")
		}
		if cmd.Flags().Changed("note") {
			spec.Notes = editNotes
			spec.markPresent("notes")
		}
		if cmd.Flags().Changed("assignee") {
			spec.Assignee = editAssignee
			spec.markPresent("assignee")
		}
		if cmd.Flags().Changed("tags") {
			spec.Tags = editTags
			spec.markPresent("tags")
		}
		if cmd.Flags().Changed("intent") {
			spec.Intent = editIntent
			spec.markPresent("intent")
		}

		if err := validateEditSpec(spec); err != nil {
			return err
		}

		// Apply spec fields to ticket, marking changed fields present.
		if spec.has("priority") && spec.Priority != nil {
			tk.Priority = *spec.Priority
			tk.Present["priority"] = true
		}
		if spec.has("parent") {
			tk.Parent = spec.Parent
			tk.Present["parent"] = true
		}
		if spec.has("deps") {
			tk.Deps = spec.Deps
			tk.Present["deps"] = true
		}
		if spec.has("body") {
			tk.Description = spec.Body
			tk.Present["description"] = true
		}
		if spec.has("acceptance_criteria") {
			tk.AcceptanceCriteria = spec.AcceptanceCriteria
			tk.Present["acceptance_criteria"] = true
		}
		if spec.has("notes") {
			tk.Notes = spec.Notes
			tk.Present["notes"] = true
		}
		if spec.has("assignee") {
			tk.Assignee = spec.Assignee
			tk.Present["assignee"] = true
		}
		if spec.has("tags") {
			tk.Tags = spec.Tags
			tk.Present["tags"] = true
		}
		if spec.has("intent") {
			tk.Intent = spec.Intent
			tk.Present["intent"] = true
		}

		if err := s.Update(tk); err != nil {
			return err
		}

		if jsonFlag {
			return outputJSON(cmd, tk)
		}
		fmt.Fprintln(cmd.OutOrStdout(), tk.ID)
		return nil
	},
}

func init() {
	editCmd.Flags().IntVarP(&editPriority, "priority", "p", 0, "ticket priority (higher = more important)")
	editCmd.Flags().StringVar(&editParent, "parent", "", "parent ticket ID")
	editCmd.Flags().StringSliceVar(&editDeps, "deps", nil, "comma-separated list of dependency ticket IDs")
	editCmd.Flags().StringVar(&editBody, "body", "", "ticket description / narrative body")
	editCmd.Flags().StringVar(&editBodyFile, "body-file", "", "path to file whose content becomes the ticket body")
	editCmd.Flags().StringArrayVar(&editAC, "ac", nil, "acceptance criterion (repeatable)")
	editCmd.Flags().StringArrayVar(&editNotes, "note", nil, "initial note (repeatable)")
	editCmd.Flags().StringVar(&editAssignee, "assignee", "", "assignee name or identifier")
	editCmd.Flags().StringSliceVar(&editTags, "tags", nil, "comma-separated tags")
	editCmd.Flags().StringVar(&editIntent, "intent", "", "high-level intent for the ticket")
	editCmd.Flags().BoolVar(&editStdin, "stdin", false, "read edit spec as JSON from stdin")
	rootCmd.AddCommand(editCmd)
}
