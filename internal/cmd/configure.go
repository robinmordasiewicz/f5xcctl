package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/f5/f5xcctl/internal/config"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure the f5xcctl CLI",
	Long: `Interactive configuration wizard for the f5xcctl CLI.

This command guides you through setting up your API credentials,
tenant information, and default preferences.

Examples:
  # Run interactive configuration
  f5xcctl configure

  # Configure with API token directly
  f5xcctl configure --api-token YOUR_TOKEN --tenant YOUR_TENANT`,
	RunE: runConfigure,
}

var (
	configAPIToken string
	configTenantID string
)

func init() {
	configureCmd.Flags().StringVar(&configAPIToken, "api-token", "", "API token for authentication")
	configureCmd.Flags().StringVar(&configTenantID, "tenant", "", "F5XC tenant name")
}

// readLine reads a line from stdin and trims whitespace including \r\n.
func readLine(reader *bufio.Reader) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// Trim both \r and \n to handle Windows-style line endings
	return strings.TrimRight(input, "\r\n"), nil
}

// readPassword reads a password from stdin without echoing (shows ******).
func readPassword() (string, error) {
	// Check if stdin is a terminal
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Fall back to regular input if not a terminal
		reader := bufio.NewReader(os.Stdin)
		return readLine(reader)
	}

	password, err := term.ReadPassword(fd)
	if err != nil {
		return "", err
	}
	fmt.Println() // Print newline since ReadPassword doesn't echo
	return string(password), nil
}

func runConfigure(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("F5 Distributed Cloud CLI Configuration")
	fmt.Println("=======================================")
	fmt.Println()

	// Get tenant
	tenantName := configTenantID
	if tenantName == "" {
		fmt.Print("Tenant name: ")
		input, err := readLine(reader)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		tenantName = input
	}

	if tenantName == "" {
		return fmt.Errorf("tenant name is required")
	}

	// Get API token (masked input)
	apiToken := configAPIToken
	if apiToken == "" {
		fmt.Print("API token: ")
		input, err := readPassword()
		if err != nil {
			return fmt.Errorf("failed to read token: %w", err)
		}
		apiToken = input
	}

	if apiToken == "" {
		return fmt.Errorf("API token is required")
	}

	// Get default namespace
	fmt.Print("Default namespace [default]: ")
	defaultNS, _ := readLine(reader)
	if defaultNS == "" {
		defaultNS = "default"
	}

	// Get output format preference
	fmt.Print("Default output format (table/json/yaml) [table]: ")
	outputFormat, _ := readLine(reader)
	if outputFormat == "" {
		outputFormat = "table"
	}

	// Build API URL from tenant (without /api suffix - paths include it)
	apiURL := fmt.Sprintf("https://%s.console.ves.volterra.io", tenantName)

	// Create configuration
	cfg := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]config.Profile{
			"default": {
				Tenant:           tenantName,
				APIURL:           apiURL,
				AuthMethod:       "api-token",
				DefaultNamespace: defaultNS,
				OutputFormat:     outputFormat,
			},
		},
	}

	creds := &config.Credentials{
		Profiles: map[string]config.ProfileCredentials{
			"default": {
				APIToken: apiToken,
			},
		},
	}

	// Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	if err := config.SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration saved successfully!")
	fmt.Printf("  Config file: %s\n", config.DefaultConfigPath())
	fmt.Printf("  Credentials: %s\n", config.DefaultCredentialsPath())
	fmt.Println()
	fmt.Println("You can now use f5xcctl commands. Try:")
	fmt.Println("  f5xcctl namespace list")

	return nil
}
