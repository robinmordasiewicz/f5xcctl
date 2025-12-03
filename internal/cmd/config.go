package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long: `Manage f5xcctl CLI configuration settings.

Use subcommands to get, set, and list configuration values.`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}

		key := args[0]
		value, err := cfg.Get(key)
		if err != nil {
			return err
		}

		fmt.Println(value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			// Create new config if none exists
			cfg = config.NewDefault()
		}

		key := args[0]
		value := args[1]

		if err := cfg.Set(key, value); err != nil {
			return err
		}

		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}

		fmt.Printf("Current profile: %s\n\n", cfg.CurrentProfile)

		profile, ok := cfg.Profiles[cfg.CurrentProfile]
		if !ok {
			return fmt.Errorf("profile %q not found", cfg.CurrentProfile)
		}

		fmt.Printf("tenant:            %s\n", profile.Tenant)
		fmt.Printf("api-url:           %s\n", profile.APIURL)
		fmt.Printf("auth-method:       %s\n", profile.AuthMethod)
		fmt.Printf("default-namespace: %s\n", profile.DefaultNamespace)
		fmt.Printf("output-format:     %s\n", profile.OutputFormat)

		return nil
	},
}

var configProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage configuration profiles",
}

var configProfilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}

		for name := range cfg.Profiles {
			if name == cfg.CurrentProfile {
				fmt.Printf("* %s (current)\n", name)
			} else {
				fmt.Printf("  %s\n", name)
			}
		}

		return nil
	},
}

var configProfilesUseCmd = &cobra.Command{
	Use:   "use <profile>",
	Short: "Switch to a different profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("", "")
		if err != nil {
			return err
		}

		profileName := args[0]
		if _, ok := cfg.Profiles[profileName]; !ok {
			return fmt.Errorf("profile %q not found", profileName)
		}

		cfg.CurrentProfile = profileName
		if err := config.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Switched to profile %q\n", profileName)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configProfilesCmd)

	configProfilesCmd.AddCommand(configProfilesListCmd)
	configProfilesCmd.AddCommand(configProfilesUseCmd)
}
