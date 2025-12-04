// Package main provides the entry point for the f5xcctl CLI.
package main

import (
	"fmt"
	"os"

	"github.com/f5/f5xcctl/internal/cmd"
)

// Version information set by build flags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
