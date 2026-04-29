package main

import (
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/graph"
	"github.com/spf13/cobra"
)

// validateCmd validates a single ticket by ID.
var validateCmd = &cobra.Command{
	Use:   "validate <id>",
	Short: "Validate a ticket by ID",
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

		errs := ticket.Validate(*tk)
		if jsonFlag {
			return outputJSON(cmd, errs)
		}

		if len(errs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "valid")
			return nil
		}

		for _, e := range errs {
			fmt.Fprintf(cmd.OutOrStdout(), "error: %s: %s\n", e.Field, e.Message)
		}
		return fmt.Errorf("ticket %s has %d validation error(s)", args[0], len(errs))
	},
}

// lintCmd validates all tickets in the store.
var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Validate all tickets in the store",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := storeFromFlag()
		if err != nil {
			return err
		}

		tickets, err := s.List()
		if err != nil {
			return err
		}

		var allErrors []struct {
			ID     string                   `json:"id"`
			Errors []ticket.ValidationError `json:"errors"`
		}

		var errorCount int
		hasErrors := false
		for _, tk := range tickets {
			errs := ticket.Validate(tk)
			if len(errs) > 0 {
				hasErrors = true
				errorCount++
				allErrors = append(allErrors, struct {
					ID     string                   `json:"id"`
					Errors []ticket.ValidationError `json:"errors"`
				}{ID: tk.ID, Errors: errs})
			}
		}

		// Also check for dependency cycles.
		cycles := graph.DetectCycles(tickets)
		if len(cycles) > 0 {
			hasErrors = true
		}

		if jsonFlag {
			return outputJSON(cmd, map[string]interface{}{
				"tickets": allErrors,
				"cycles":  cycles,
			})
		}

		for _, entry := range allErrors {
			for _, e := range entry.Errors {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s: %s\n", entry.ID, e.Field, e.Message)
			}
		}

		for _, cycle := range cycles {
			fmt.Fprintf(cmd.OutOrStdout(), "cycle: %v\n", cycle)
		}

		if !hasErrors {
			fmt.Fprintln(cmd.OutOrStdout(), "all tickets valid")
			return nil
		}
		return fmt.Errorf("lint found %d ticket(s) with errors, %d cycle(s)", errorCount, len(cycles))
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(lintCmd)
}
