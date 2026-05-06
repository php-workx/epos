package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/spf13/cobra"
)

// readyCmd lists tickets that are ready to be worked.
//
// Tickets with an active (non-expired) claim sidecar are excluded so that
// concurrent agents do not race on the same ticket. Use --include-claimed
// to disable that filter when debugging.
var readyCmd = &cobra.Command{
	Use:   "ready [parent]",
	Short: "List tickets ready to be worked",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		var isClaimed func(string) bool
		if !readyIncludeClaimed {
			claimed, cerr := s.ActiveClaimSet()
			if cerr != nil {
				return cerr
			}
			isClaimed = func(id string) bool { return claimed[id] }
		}

		if len(args) > 0 && args[0] != "" {
			// Filter by parent.
			parentID, _, err := resolveTicketID(s, args[0])
			if err != nil {
				return err
			}
			tickets, err := s.List()
			if err != nil {
				return err
			}
			children := graph.FilterReadyChildren(tickets, parentID, isClaimed)
			if jsonFlag {
				return outputJSON(cmd, children)
			}
			for _, t := range children {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", t.ID, t.Title)
			}
			return nil
		}

		tickets, err := s.List()
		if err != nil {
			return err
		}
		ready := graph.ReadyFilterUnclaimed(tickets, isClaimed)
		if jsonFlag {
			return outputJSON(cmd, ready)
		}
		for _, t := range ready {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", t.ID, t.Title)
		}
		return nil
	},
}

var readyIncludeClaimed bool

// blockedCmd lists tickets that are blocked by open dependencies.
var blockedCmd = &cobra.Command{
	Use:   "blocked [parent]",
	Short: "List tickets blocked by dependencies",
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

		var blocked []ticket.Ticket
		if len(args) > 0 && args[0] != "" {
			parentID, _, err := resolveTicketID(s, args[0])
			if err != nil {
				return err
			}
			for _, t := range graph.BlockedFilter(tickets) {
				if t.Parent == parentID {
					blocked = append(blocked, t)
				}
			}
		} else {
			blocked = graph.BlockedFilter(tickets)
		}

		if jsonFlag {
			return outputJSON(cmd, blocked)
		}

		for _, t := range blocked {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", t.ID, t.Title)
		}
		return nil
	},
}

func init() {
	readyCmd.Flags().BoolVar(&readyIncludeClaimed, "include-claimed", false,
		"include tickets with an active claim sidecar (default: filtered out)")
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(blockedCmd)
}
