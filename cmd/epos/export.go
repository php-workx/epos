package main

import (
	"github.com/php-workx/epos/ticket/graph"
	"github.com/spf13/cobra"
)

// exportCmd exports tickets as JSON for agent consumption.
var exportCmd = &cobra.Command{
	Use:   "export [parent]",
	Short: "Export tickets as JSON",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		tickets, err := s.List()
		if err != nil {
			return err
		}

		// If a parent ID is specified, filter to children.
		if len(args) > 0 && args[0] != "" {
			parentID, _, err := resolveTicketID(s, args[0])
			if err != nil {
				return err
			}
			tickets = graph.FilterChildren(tickets, parentID)
		}

		// Always output as JSON for export.
		return outputJSON(cmd, tickets)
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
