package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/runtime"
	"github.com/spf13/cobra"
)

var (
	claimOwner   string
	releaseOwner string
)

// claimCmd claims a ticket for an agent.
var claimCmd = &cobra.Command{
	Use:   "claim <id>",
	Short: "Claim a ticket for an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		if claimOwner == "" {
			return &ticket.ValidationError{Field: "owner", Message: "required: --owner"}
		}

		fullID, _, err := resolveTicketID(s, args[0])
		if err != nil {
			return err
		}

		if err := runtime.Claim(dirFlag, fullID, claimOwner, "epos-cli", ticket.DefaultLeaseDuration); err != nil {
			return err
		}

		if jsonFlag {
			return outputJSON(cmd, map[string]string{"id": fullID, "claimed_by": claimOwner, "status": "claimed"})
		}

		fmt.Fprintf(cmd.OutOrStdout(), "claimed %s by %s\n", fullID, claimOwner)
		return nil
	},
}

// releaseCmd releases a ticket claim.
var releaseCmd = &cobra.Command{
	Use:   "release <id>",
	Short: "Release a ticket claim",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		if releaseOwner == "" {
			return &ticket.ValidationError{Field: "owner", Message: "required: --owner"}
		}

		fullID, _, err := resolveTicketID(s, args[0])
		if err != nil {
			return err
		}

		// Release the claim and set status back to pending.
		if err := runtime.Release(dirFlag, fullID, releaseOwner, ticket.StatusPending, ""); err != nil {
			return err
		}

		if jsonFlag {
			return outputJSON(cmd, map[string]string{"id": fullID, "status": string(ticket.StatusPending)})
		}

		fmt.Fprintf(cmd.OutOrStdout(), "released %s\n", fullID)
		return nil
	},
}

func init() {
	claimCmd.Flags().StringVarP(&claimOwner, "owner", "o", "", "agent ID claiming the ticket")
	releaseCmd.Flags().StringVarP(&releaseOwner, "owner", "o", "", "agent ID releasing the ticket")
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(releaseCmd)
}
