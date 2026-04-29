package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/spf13/cobra"
)

var (
	closeReason  string
	reopenReason string
)

// closeCmd transitions a ticket to closed status.
var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		fullID, tk, err := resolveTicketID(s, args[0])
		if err != nil {
			return err
		}

		tk.Status = ticket.StatusClosed
		tk.Present["status"] = true
		if closeReason != "" {
			tk.StatusReason = closeReason
			tk.Present["status_reason"] = true
		}

		if err := s.Update(tk); err != nil {
			return err
		}

		if jsonFlag {
			return outputJSON(cmd, map[string]string{"id": fullID, "status": string(ticket.StatusClosed)})
		}

		fmt.Fprintf(cmd.OutOrStdout(), "closed %s\n", fullID)
		return nil
	},
}

// reopenCmd transitions a ticket back to open status.
var reopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a closed ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		fullID, tk, err := resolveTicketID(s, args[0])
		if err != nil {
			return err
		}

		tk.Status = ticket.StatusOpen
		tk.Present["status"] = true
		if reopenReason != "" {
			tk.StatusReason = reopenReason
			tk.Present["status_reason"] = true
		}

		if err := s.Update(tk); err != nil {
			return err
		}

		if jsonFlag {
			return outputJSON(cmd, map[string]string{"id": fullID, "status": string(ticket.StatusOpen)})
		}

		fmt.Fprintf(cmd.OutOrStdout(), "reopened %s\n", fullID)
		return nil
	},
}

func init() {
	closeCmd.Flags().StringVarP(&closeReason, "reason", "r", "", "reason for the status change")
	reopenCmd.Flags().StringVarP(&reopenReason, "reason", "r", "", "reason for the status change")
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(reopenCmd)
}
