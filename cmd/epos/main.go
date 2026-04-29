// Command epos is the CLI entry point for the epos shared ticket system.
package main

import (
	"errors"
	"os"

	"github.com/php-workx/epos/ticket"
)

// exitCode maps an error to a deterministic exit code per the CLI spec:
//   - *ticket.TicketNotFoundError → 3
//   - *ticket.AmbiguousIDError  → 4
//   - *ticket.ValidationError    → 2
//   - nil                       → 0
//   - any other error           → 1
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var notFound *ticket.TicketNotFoundError
	if errors.As(err, &notFound) {
		return 3
	}
	var ambiguous *ticket.AmbiguousIDError
	if errors.As(err, &ambiguous) {
		return 4
	}
	var validation *ticket.ValidationError
	if errors.As(err, &validation) {
		return 2
	}
	return 1
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}
