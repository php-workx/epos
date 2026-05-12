package main

import (
	"encoding/json"
	"fmt"
	"os"

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

		spec := &ticket.TicketPatch{}

		if editStdin {
			if err := json.NewDecoder(os.Stdin).Decode(spec); err != nil {
				return &ticket.ValidationError{Field: "stdin", Message: fmt.Sprintf("invalid JSON: %s", err)}
			}
		}

		// CLI flags override stdin values; only apply flags that were explicitly set.
		if cmd.Flags().Changed("priority") {
			spec.SetPriority(editPriority)
		}
		if cmd.Flags().Changed("parent") {
			spec.SetParent(editParent)
		}
		if cmd.Flags().Changed("deps") {
			spec.SetDeps(editDeps)
		}
		if cmd.Flags().Changed("body") {
			spec.SetDescription(editBody)
		}
		if cmd.Flags().Changed("body-file") {
			data, err := os.ReadFile(editBodyFile) //nolint:gosec // G304: path is the user's explicit --body-file argument
			if err != nil {
				return &ticket.ValidationError{Field: "body-file", Message: err.Error()}
			}
			spec.SetDescription(string(data))
		}
		if cmd.Flags().Changed("ac") {
			spec.SetAcceptanceCriteria(editAC)
		}
		if cmd.Flags().Changed("note") {
			spec.SetNotes(editNotes)
		}
		if cmd.Flags().Changed("assignee") {
			spec.SetAssignee(editAssignee)
		}
		if cmd.Flags().Changed("tags") {
			spec.SetTags(editTags)
		}
		if cmd.Flags().Changed("intent") {
			spec.SetIntent(editIntent)
		}

		if err := spec.Validate(); err != nil {
			return err
		}

		spec.Apply(tk)

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
