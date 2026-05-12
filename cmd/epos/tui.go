package main

import (
	"fmt"
	"os"
	"time"

	"github.com/php-workx/epos/internal/tui"
	"github.com/php-workx/epos/ticket"
	"github.com/php-workx/epos/ticket/store"
	"github.com/spf13/cobra"

	tea "charm.land/bubbletea/v2"
)

var (
	tuiOwner   string
	tuiRefresh time.Duration
)

// tuiCmd opens the interactive ticket TUI.
var tuiCmd = &cobra.Command{
	Use:   "tui [parent]",
	Short: "Open interactive ticket TUI",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonFlag {
			return &ticket.ValidationError{Field: "json", Message: "tui is interactive and does not support --json"}
		}
		s, err := store.NewFileStore(dirFlag)
		if err != nil {
			return err
		}
		parent := ""
		if len(args) == 1 {
			parent = args[0]
		}
		owner := resolveTUIOwner(tuiOwner)
		model := tui.NewModel(tui.Config{
			DataSource: tui.NewStoreDataSource(s),
			Parent:     parent,
			Owner:      owner,
			Refresh:    tuiRefresh,
		})
		_, err = tea.NewProgram(model).Run()
		if err != nil {
			return fmt.Errorf("run tui: %w", err)
		}
		return nil
	},
}

func resolveTUIOwner(flag string) string {
	if flag != "" {
		return flag
	}
	for _, key := range []string{"EPOS_OWNER", "USER", "USERNAME"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func init() {
	tuiCmd.Flags().StringVar(&tuiOwner, "owner", "", "owner used for claim and release actions")
	tuiCmd.Flags().DurationVar(&tuiRefresh, "refresh", 5*time.Second, "polling refresh interval (0 disables polling)")
	rootCmd.AddCommand(tuiCmd)
}
