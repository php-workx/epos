package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/spf13/cobra"
)

var (
	newType     string
	newPriority int
	newParent   string
	newDeps     []string
	newBody     string
	newBodyFile string
	newAC       []string
	newNotes    []string
	newAssignee string
	newTags     []string
	newIntent   string
	newStdin    bool
)

// newTicketSpec is the JSON input schema for --stdin.
type newTicketSpec struct {
	Title              string   `json:"title"`
	Type               string   `json:"type"`
	Priority           int      `json:"priority"`
	Parent             string   `json:"parent"`
	Deps               []string `json:"deps"`
	Body               string   `json:"body"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Notes              []string `json:"notes"`
	Assignee           string   `json:"assignee"`
	Tags               []string `json:"tags"`
	Intent             string   `json:"intent"`
}

const (
	defaultTicketType = "task"
	newIDMaxAttempts  = 8
)

func specToTicket(spec *newTicketSpec) *ticket.Ticket {
	opts := []ticket.TicketOption{
		ticket.WithTitle(spec.Title),
		ticket.WithType(spec.Type),
	}
	if spec.Parent != "" {
		opts = append(opts, ticket.WithParent(spec.Parent))
	}
	if len(spec.Deps) > 0 {
		opts = append(opts, ticket.WithDeps(spec.Deps...))
	}
	if spec.Body != "" {
		opts = append(opts, ticket.WithDescription(spec.Body))
	}
	if len(spec.AcceptanceCriteria) > 0 {
		opts = append(opts, ticket.WithAcceptanceCriteria(spec.AcceptanceCriteria...))
	}
	if len(spec.Notes) > 0 {
		opts = append(opts, ticket.WithNotes(spec.Notes...))
	}
	if spec.Assignee != "" {
		opts = append(opts, ticket.WithAssignee(spec.Assignee))
	}
	if len(spec.Tags) > 0 {
		tags := make([]string, len(spec.Tags))
		for i, tag := range spec.Tags {
			tags[i] = strings.TrimSpace(tag)
		}
		opts = append(opts, ticket.WithTags(tags...))
	}
	if spec.Intent != "" {
		opts = append(opts, ticket.WithIntent(spec.Intent))
	}
	opts = append(opts, ticket.WithPriority(spec.Priority))
	return ticket.NewTicket(opts...)
}

func createTicketWithGeneratedID(s store.Store, spec *newTicketSpec, generateID func(string) string, attempts int) (*ticket.Ticket, error) {
	var lastCollision *ticket.IDCollisionError
	for range attempts {
		tk := specToTicket(spec)
		tk.ID = generateID(spec.Title)
		tk.Present["id"] = true

		if err := s.Create(tk); err != nil {
			var collision *ticket.IDCollisionError
			if errors.As(err, &collision) {
				lastCollision = collision
				continue
			}
			return nil, err
		}
		return tk, nil
	}
	if lastCollision != nil {
		return nil, fmt.Errorf("generate unique ticket ID after %d attempts: %w", attempts, lastCollision)
	}
	return nil, fmt.Errorf("could not generate ticket ID after %d attempts", attempts)
}

var newCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Create a new ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec := &newTicketSpec{}

		if newStdin {
			if err := json.NewDecoder(os.Stdin).Decode(spec); err != nil {
				return &ticket.ValidationError{Field: "stdin", Message: fmt.Sprintf("invalid JSON: %s", err)}
			}
		}

		// CLI flags always override stdin values.
		spec.Title = args[0]
		if cmd.Flags().Changed("type") {
			spec.Type = newType
		} else if spec.Type == "" {
			spec.Type = defaultTicketType
		}
		if cmd.Flags().Changed("priority") {
			spec.Priority = newPriority
		}
		if cmd.Flags().Changed("parent") {
			spec.Parent = newParent
		}
		if cmd.Flags().Changed("deps") {
			spec.Deps = newDeps
		}
		if cmd.Flags().Changed("body") {
			spec.Body = newBody
		}
		if cmd.Flags().Changed("body-file") {
			data, err := os.ReadFile(newBodyFile) //nolint:gosec // G304: path is the user's explicit --body-file argument
			if err != nil {
				return &ticket.ValidationError{Field: "body-file", Message: err.Error()}
			}
			spec.Body = string(data)
		}
		if cmd.Flags().Changed("ac") {
			spec.AcceptanceCriteria = newAC
		}
		if cmd.Flags().Changed("note") {
			spec.Notes = newNotes
		}
		if cmd.Flags().Changed("assignee") {
			spec.Assignee = newAssignee
		}
		if cmd.Flags().Changed("tags") {
			spec.Tags = newTags
		}
		if cmd.Flags().Changed("intent") {
			spec.Intent = newIntent
		}

		if err := ticket.ValidateNew(specToTicket(spec)); err != nil {
			return err
		}

		s, err := storeFromFlag()
		if err != nil {
			return err
		}
		tk, err := createTicketWithGeneratedID(s, spec, store.GenerateIDWithSuffix, newIDMaxAttempts)
		if err != nil {
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
	newCmd.Flags().StringVarP(&newType, "type", "t", defaultTicketType, "ticket type (epic, task, issue, feature, bug, chore, spike, doc)")
	newCmd.Flags().IntVarP(&newPriority, "priority", "p", 0, "ticket priority (higher = more important)")
	newCmd.Flags().StringVar(&newParent, "parent", "", "parent ticket ID")
	newCmd.Flags().StringSliceVar(&newDeps, "deps", nil, "comma-separated list of dependency ticket IDs")
	newCmd.Flags().StringVar(&newBody, "body", "", "ticket description / narrative body")
	newCmd.Flags().StringVar(&newBodyFile, "body-file", "", "path to file whose content becomes the ticket body")
	newCmd.Flags().StringArrayVar(&newAC, "ac", nil, "acceptance criterion (repeatable)")
	newCmd.Flags().StringArrayVar(&newNotes, "note", nil, "initial note (repeatable)")
	newCmd.Flags().StringVar(&newAssignee, "assignee", "", "assignee name or identifier")
	newCmd.Flags().StringSliceVar(&newTags, "tags", nil, "comma-separated tags")
	newCmd.Flags().StringVar(&newIntent, "intent", "", "high-level intent for the ticket")
	newCmd.Flags().BoolVar(&newStdin, "stdin", false, "read ticket spec as JSON from stdin")
	rootCmd.AddCommand(newCmd)
}
