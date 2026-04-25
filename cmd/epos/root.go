// Package main implements the epos CLI entry point and command tree.
package main

import (
	"github.com/spf13/cobra"
)

// rootCmd is the base command for the epos CLI.
var rootCmd = &cobra.Command{
	Use:   "epos",
	Short: "Shared ticket system CLI",
	Long:  `epos is the CLI for the shared ticket system, providing commands to create, query, and manage tickets.`,
}

// dirFlag is the filesystem directory containing the .tickets subdirectory.
// Defaults to the current working directory.
var dirFlag string

// jsonFlag controls machine-readable JSON output for every command.
var jsonFlag bool

func init() {
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "dir", "d", ".", "directory containing the .tickets folder")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
}
