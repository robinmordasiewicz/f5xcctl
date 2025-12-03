package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit, and build date of the f5xcctl CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "f5xcctl version %s\n", versionInfo.Version)
		fmt.Fprintf(out, "  commit: %s\n", versionInfo.Commit)
		fmt.Fprintf(out, "  built:  %s\n", versionInfo.Date)
		fmt.Fprintf(out, "  go:     %s\n", runtime.Version())
		fmt.Fprintf(out, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}
