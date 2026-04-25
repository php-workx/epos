// Package main provides shared helper functions for the epos CLI commands.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/spf13/cobra"
)

// storeFromFlag creates a FileStore rooted at the value of the --dir flag.
// The .tickets subdirectory is created automatically if it does not exist.
func storeFromFlag() (*store.FileStore, error) {
	return store.NewFileStore(dirFlag)
}

// resolveTicketID resolves a partial ticket ID to a full ID and loads the
// ticket. It returns the resolved full ID, the loaded ticket, and any error.
// Returns *ticket.TicketNotFoundError if no match is found and
// *ticket.AmbiguousIDError if the partial matches more than one ticket.
func resolveTicketID(s *store.FileStore, partial string) (string, *ticket.Ticket, error) {
	id, err := s.ResolveID(partial)
	if err != nil {
		return "", nil, err
	}
	tk, err := s.Read(id)
	if err != nil {
		return "", nil, err
	}
	return id, tk, nil
}

// outputJSON marshals v as pretty-printed JSON and writes it to the command's
// stdout. Used by all commands that support the --json flag.
func outputJSON(cmd *cobra.Command, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
