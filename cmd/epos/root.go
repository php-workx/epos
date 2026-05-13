// Package main implements the epos CLI entry point and command tree.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// These variables are populated by release builds via -ldflags.
var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

// rootCmd is the base command for the epos CLI.
var rootCmd = &cobra.Command{
	Use:     "epos",
	Short:   "Shared ticket system CLI",
	Long:    `epos is the CLI for the shared ticket system, providing commands to create, query, and manage tickets.`,
	Version: versionString(),
}

// dirFlag is the filesystem directory containing the .tickets subdirectory.
// Defaults to the current working directory.
var dirFlag string

// jsonFlag controls machine-readable JSON output for every command.
var jsonFlag bool

func init() {
	rootCmd.Version = versionString()
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "dir", "d", ".", "directory containing the .tickets folder")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "output in JSON format")
}

func versionString() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version, gitCommit, buildDate)
}
