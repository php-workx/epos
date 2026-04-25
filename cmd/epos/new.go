package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/spf13/cobra"
)

var (
	newType     string
	newPriority int
	newParent   string
	newDeps     []string
)

// newCmd creates a new ticket.
var newCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Create a new ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]

		opts := []ticket.TicketOption{
			ticket.WithTitle(title),
		}

		if newType != "" {
			opts = append(opts, ticket.WithType(newType))
		} else {
			opts = append(opts, ticket.WithType("task"))
		}

		if newParent != "" {
			opts = append(opts, ticket.WithParent(newParent))
		}

		if len(newDeps) > 0 {
			opts = append(opts, ticket.WithDeps(newDeps...))
		}

		tk := ticket.NewTicket(opts...)
		tk.Priority = newPriority
		tk.Present["priority"] = true

		// Generate an ID from the title.
		tk.ID = store.GenerateIDWithSuffix(title)
		tk.Present["id"] = true

		s, err := store.NewFileStore(dirFlag)
		if err != nil {
			return err
		}

		if err := s.Create(tk); err != nil {
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
	newCmd.Flags().StringVarP(&newType, "type", "t", "task", "ticket type (epic, task, issue, feature, bug, chore, spike, doc)")
	newCmd.Flags().IntVarP(&newPriority, "priority", "p", 0, "ticket priority (higher = more important)")
	newCmd.Flags().StringVarP(&newParent, "parent", "", "", "parent ticket ID")
	newCmd.Flags().StringSliceVarP(&newDeps, "deps", "", nil, "comma-separated list of dependency ticket IDs")
	rootCmd.AddCommand(newCmd)
}
