//go:build tools

// Package tools pins build-time and future direct dependencies so that
// "go mod tidy" does not remove them before they are imported by real code.
package tools

import (
	_ "github.com/gofrs/flock"
	_ "github.com/spf13/cobra"
	_ "gopkg.in/yaml.v3"
)
