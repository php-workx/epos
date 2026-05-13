package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/php-workx/epos/ticket/store"
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

		if len(args) > 0 && args[0] != "" {
			parentID, _, err := resolveTicketID(s, args[0])
			if err != nil {
				return err
			}
			var children []ticket.Ticket
			if readyIncludeClaimed {
				all, lerr := s.List()
				if lerr != nil {
					return lerr
				}
				children = graph.FilterReadyChildren(all, parentID, nil)
			} else {
				children, err = store.ReadyChildren(s, parentID)
				if err != nil {
					return err
				}
			}
			if jsonFlag {
				return outputJSON(cmd, children)
			}
			for _, t := range children {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", t.ID, t.Title)
			}
			return nil
		}

		var ready []ticket.Ticket
		if readyIncludeClaimed {
			all, lerr := s.List()
			if lerr != nil {
				return lerr
			}
			ready = graph.ReadyFilterUnclaimed(all, nil)
		} else {
			ready, err = store.ReadyTickets(s)
			if err != nil {
				return err
			}
		}
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
