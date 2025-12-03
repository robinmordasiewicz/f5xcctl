package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/auth"
	"github.com/f5/f5xcctl/internal/config"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
	Long: `Manage authentication for the f5xcctl CLI.

Subcommands allow you to log in, log out, and check authentication status.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to F5 Distributed Cloud",
	Long: `Log in to F5 Distributed Cloud using browser-based SSO.

This command opens your browser to authenticate with F5XC and stores
the resulting credentials locally.

Examples:
  # Interactive browser login
  f5xcctl auth login

  # Login with API token (non-interactive)
  f5xcctl auth login --api-token YOUR_TOKEN`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from F5 Distributed Cloud",
	Long:  `Remove stored credentials for the current profile.`,
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Display the current authentication status and token validity.`,
	RunE:  runAuthStatus,
}

var (
	authAPIToken string
)

func init() {
	authLoginCmd.Flags().StringVar(&authAPIToken, "api-token", "", "API token for non-interactive login")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("", profile)
	if err != nil {
		return fmt.Errorf("please run 'f5xcctl configure' first: %w", err)
	}

	currentProfile := cfg.GetCurrentProfile()
	if currentProfile == nil {
		return fmt.Errorf("no profile configured")
	}

	if authAPIToken != "" {
		// Non-interactive: use provided token
		creds, err := config.LoadCredentials()
		if err != nil {
			creds = &config.Credentials{
				Profiles: make(map[string]config.ProfileCredentials),
			}
		}

		creds.Profiles[cfg.CurrentProfile] = config.ProfileCredentials{
			APIToken: authAPIToken,
		}

		if err := config.SaveCredentials(creds); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}

		fmt.Println("✓ API token saved successfully")
		return nil
	}

	// Interactive: browser-based SSO
	fmt.Println("Opening browser for authentication...")

	authenticator := auth.NewBrowserAuth(currentProfile.Tenant, currentProfile.APIURL)
	token, err := authenticator.Login()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save the token
	creds, err := config.LoadCredentials()
	if err != nil {
		creds = &config.Credentials{
			Profiles: make(map[string]config.ProfileCredentials),
		}
	}

	creds.Profiles[cfg.CurrentProfile] = config.ProfileCredentials{
		APIToken:  token.AccessToken,
		ExpiresAt: token.ExpiresAt,
	}

	if err := config.SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println("✓ Successfully authenticated")
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("", profile)
	if err != nil {
		return err
	}

	creds, err := config.LoadCredentials()
	if err != nil {
		return fmt.Errorf("no credentials found")
	}

	delete(creds.Profiles, cfg.CurrentProfile)

	if err := config.SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println("✓ Logged out successfully")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("", profile)
	if err != nil {
		fmt.Println("Status: Not configured")
		fmt.Println("\nRun 'f5xcctl configure' to set up the CLI")
		return nil
	}

	creds, err := config.LoadCredentials()
	if err != nil {
		fmt.Printf("Profile: %s\n", cfg.CurrentProfile)
		fmt.Println("Status:  Not authenticated")
		fmt.Println("\nRun 'f5xcctl auth login' to authenticate")
		return nil
	}

	profileCreds, ok := creds.Profiles[cfg.CurrentProfile]
	if !ok || profileCreds.APIToken == "" {
		fmt.Printf("Profile: %s\n", cfg.CurrentProfile)
		fmt.Println("Status:  Not authenticated")
		fmt.Println("\nRun 'f5xcctl auth login' to authenticate")
		return nil
	}

	currentProfile := cfg.GetCurrentProfile()
	fmt.Printf("Profile: %s\n", cfg.CurrentProfile)
	fmt.Printf("Tenant:  %s\n", currentProfile.Tenant)
	fmt.Println("Status:  Authenticated")

	if !profileCreds.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", profileCreds.ExpiresAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}
