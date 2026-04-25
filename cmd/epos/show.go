package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// showCmd displays a ticket by its ID (supports partial matching).
var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display a ticket",
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

		if jsonFlag {
			return outputJSON(cmd, tk)
		}

		// Human-readable output.
		fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", tk.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "Title:    %s\n", tk.Title)
		fmt.Fprintf(cmd.OutOrStdout(), "Type:     %s\n", tk.Type)
		fmt.Fprintf(cmd.OutOrStdout(), "Status:   %s\n", tk.Status)
		if tk.Parent != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Parent:   %s\n", tk.Parent)
		}
		if len(tk.Deps) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Deps:     %v\n", tk.Deps)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Priority: %d\n", tk.Priority)
		if len(tk.Tags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Tags:     %v\n", tk.Tags)
		}
		if tk.ExtendedStatus != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "ExtStatus: %s\n", tk.ExtendedStatus)
		}
		if tk.StatusReason != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Reason:   %s\n", tk.StatusReason)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
