// Package cmd provides the CLI commands for f5xcctl.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/f5/f5xcctl/internal/config"
	"github.com/f5/f5xcctl/internal/output"
)

var (
	cfgFile     string
	profile     string
	outputFmt   string
	namespace   string
	tenant      string
	apiURL      string
	debug       bool
	verbosity   int
	versionInfo VersionInfo
)

// VersionInfo holds version metadata.
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

// SetVersionInfo sets the version information for the CLI.
func SetVersionInfo(version, commit, date string) {
	versionInfo = VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "f5xcctl",
	Short: "F5 Distributed Cloud Control - kubectl-style CLI for F5XC",
	Long: `f5xcctl is a command-line interface for managing F5 Distributed Cloud resources.

It provides kubectl-style commands to create, read, update, and delete resources
such as load balancers, origin pools, firewalls, certificates, and more.

When run without arguments, f5xcctl starts in interactive mode with auto-completion.

Configuration:
  Configure the CLI using 'f5xcctl configure' or set environment variables:
    F5XC_API_TOKEN    API token for authentication
    F5XC_TENANT       Tenant name
    F5XC_NAMESPACE    Default namespace

Examples:
  # Start interactive mode (default when no args)
  f5xcctl

  # List resources (kubectl-style)
  f5xcctl get namespace
  f5xcctl get httplb -n production
  f5xcctl get httplb -A              # all namespaces

  # Create/apply resources
  f5xcctl apply -f loadbalancer.yaml
  f5xcctl create namespace my-ns

  # Delete resources
  f5xcctl delete httplb my-lb -n production

  # Describe resources
  f5xcctl describe httplb my-lb -n production

  # List all resource types
  f5xcctl api-resources

Documentation:
  https://docs.cloud.f5.com/docs/reference/api`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Set default Run to start interactive mode when no subcommand is provided
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		runInteractive(cmd, args)
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.f5xcctl/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "configuration profile to use")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "output format: table, json, yaml, text")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "namespace for the operation")
	rootCmd.PersistentFlags().StringVar(&tenant, "tenant", "", "F5XC tenant name")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "F5XC API URL")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug output")
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbosity level (use -v, -vv, -vvv, or -v=N)")

	// Bind flags to viper
	_ = viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	_ = viper.BindPFlag("tenant", rootCmd.PersistentFlags().Lookup("tenant"))
	_ = viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))

	// Add subcommands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(namespaceCmd)
	rootCmd.AddCommand(newLBCmd())
	rootCmd.AddCommand(newOriginCmd())
	rootCmd.AddCommand(newSecurityCmd())
	rootCmd.AddCommand(newCertCmd())
	rootCmd.AddCommand(newDNSCmd())
	rootCmd.AddCommand(newMonitorCmd())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() error {
	cfg, err := config.Load(cfgFile, profile)
	if err != nil {
		// Config file is optional for some commands
		if debug {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	// Store config in context for subcommands
	if cfg != nil {
		profile := cfg.GetCurrentProfile()
		if profile != nil {
			// Apply config values as defaults (flags take precedence)
			if namespace == "" && profile.DefaultNamespace != "" {
				namespace = profile.DefaultNamespace
			}
			if tenant == "" && profile.Tenant != "" {
				tenant = profile.Tenant
			}
			if apiURL == "" && profile.APIURL != "" {
				apiURL = profile.APIURL
			}
			if outputFmt == "table" && profile.OutputFormat != "" {
				outputFmt = profile.OutputFormat
			}
		}
	}

	return nil
}

// GetOutputFormatter returns the appropriate output formatter based on flags.
func GetOutputFormatter() output.Formatter {
	return output.NewFormatter(outputFmt)
}

// GetNamespace returns the namespace to use for operations.
func GetNamespace() string {
	if namespace != "" {
		return namespace
	}
	return "default"
}

// GetTenant returns the tenant name.
func GetTenant() string {
	return tenant
}

// GetAPIURL returns the API URL.
func GetAPIURL() string {
	return apiURL
}

// IsDebug returns whether debug mode is enabled.
func IsDebug() bool {
	return debug || verbosity >= 4
}

// GetVerbosity returns the current verbosity level
// Verbosity levels:
//
//	0: Normal output (default)
//	1: Show request URLs
//	2: Show request/response headers
//	3: Show request bodies
//	4+: Full debug output
func GetVerbosity() int {
	return verbosity
}

// IsVerbose returns true if verbosity is at least the given level.
func IsVerbose(level int) bool {
	return verbosity >= level
}
