// Command epos is the CLI entry point for the epos shared ticket system.
package main

import (
	"errors"
	"os"
)

// exitCoder is implemented by domain errors that carry a deterministic exit code.
type exitCoder interface{ ExitCode() int }

// exitCode maps an error to a deterministic exit code per the CLI spec:
//   - nil                         → 0
//   - *ticket.ValidationError     → 2
//   - *ticket.TicketNotFoundError → 3
//   - *ticket.AmbiguousIDError    → 4
//   - *ticket.CycleDetectedError  → 5
//   - claim errors                → 6
//   - any other error             → 1
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ec exitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return 1
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}
