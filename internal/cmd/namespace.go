package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/config"
	"github.com/f5/f5xcctl/internal/output"
	"github.com/f5/f5xcctl/internal/runtime"
)

var namespaceCmd = &cobra.Command{
	Use:     "namespace",
	Aliases: []string{"ns"},
	Short:   "Manage namespaces",
	Long: `Manage F5 Distributed Cloud namespaces.

Namespaces are the primary organizational unit in F5XC, providing isolation
for resources and access control boundaries.

Examples:
  # List all namespaces
  f5xcctl namespace list

  # Get a specific namespace
  f5xcctl namespace get my-namespace

  # Create a new namespace
  f5xcctl namespace create my-namespace --description "My namespace"

  # Delete a namespace
  f5xcctl namespace delete my-namespace`,
}

var namespaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List namespaces",
	Long:  `List all namespaces in the tenant.`,
	RunE:  runNamespaceList,
}

var namespaceGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get namespace details",
	Long:  `Get detailed information about a specific namespace.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceGet,
}

var namespaceCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a namespace",
	Long:  `Create a new namespace in the tenant.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceCreate,
}

var namespaceDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a namespace",
	Long:  `Delete a namespace from the tenant.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runNamespaceDelete,
}

var (
	nsDescription string
	nsLabels      map[string]string
	nsLabelFilter string
	nsForce       bool
)

func init() {
	// List flags
	namespaceListCmd.Flags().StringVar(&nsLabelFilter, "label-filter", "", "Filter namespaces by labels (e.g., 'env=prod,team=platform')")

	// Create flags
	namespaceCreateCmd.Flags().StringVar(&nsDescription, "description", "", "Description for the namespace")
	namespaceCreateCmd.Flags().StringToStringVar(&nsLabels, "labels", nil, "Labels for the namespace (key=value,...)")

	// Delete flags
	namespaceDeleteCmd.Flags().BoolVar(&nsForce, "yes", false, "Skip confirmation prompt")

	namespaceCmd.AddCommand(namespaceListCmd)
	namespaceCmd.AddCommand(namespaceGetCmd)
	namespaceCmd.AddCommand(namespaceCreateCmd)
	namespaceCmd.AddCommand(namespaceDeleteCmd)
}

// NamespaceResponse represents a namespace from the list API
// The F5XC list API returns a flat structure with fields at the top level.
type NamespaceResponse struct {
	Tenant      string            `json:"tenant,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Name        string            `json:"name"`
	UID         string            `json:"uid,omitempty"`
	Description string            `json:"description,omitempty"`
	Disabled    bool              `json:"disabled,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// NamespaceGetResponse represents a single namespace from the GET API
// The GET API returns a nested structure with metadata and system_metadata.
type NamespaceGetResponse struct {
	Metadata       NamespaceGetMetadata       `json:"metadata"`
	SystemMetadata NamespaceGetSystemMetadata `json:"system_metadata"`
	Spec           interface{}                `json:"spec,omitempty"`
}

// NamespaceGetMetadata represents metadata from a single namespace GET.
type NamespaceGetMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Description string            `json:"description,omitempty"`
	Disable     bool              `json:"disable,omitempty"`
}

// NamespaceGetSystemMetadata represents system metadata from a single namespace GET.
type NamespaceGetSystemMetadata struct {
	UID                   string    `json:"uid,omitempty"`
	Tenant                string    `json:"tenant,omitempty"`
	CreationTimestamp     time.Time `json:"creation_timestamp,omitempty"`
	ModificationTimestamp time.Time `json:"modification_timestamp,omitempty"`
	CreatorClass          string    `json:"creator_class,omitempty"`
	CreatorID             string    `json:"creator_id,omitempty"`
}

// NamespaceListResponse represents the list namespaces response.
type NamespaceListResponse struct {
	Items  []NamespaceResponse `json:"items"`
	Errors []interface{}       `json:"errors,omitempty"`
}

// NamespaceCreateMetadata represents metadata for namespace creation.
type NamespaceCreateMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Description string            `json:"description,omitempty"`
}

// NamespaceCreateSpec represents spec for namespace creation.
type NamespaceCreateSpec struct {
	// Add spec fields as needed for creation
}

// NamespaceCreateRequest represents a namespace create request.
type NamespaceCreateRequest struct {
	Metadata NamespaceCreateMetadata `json:"metadata"`
	Spec     NamespaceCreateSpec     `json:"spec,omitempty"`
}

// NamespaceTableOutput for table display.
type NamespaceTableOutput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func getClient() (*runtime.Client, error) {
	// First, try to create client from environment variables
	// This supports F5XC_API_URL, F5XC_API_P12_FILE, F5XC_P12_PASSWORD,
	// F5XC_CERT_FILE, F5XC_KEY_FILE, and F5XC_API_TOKEN
	if os.Getenv("F5XC_API_URL") != "" {
		client, err := runtime.NewClientFromEnv(runtime.WithDebug(debug))
		if err == nil {
			return client, nil
		}
		// If env-based client creation failed, fall back to config file
		if debug {
			fmt.Fprintf(os.Stderr, "Warning: Failed to create client from environment: %v\n", err)
		}
	}

	// Fall back to config file
	cfg, err := config.Load(cfgFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w\n\nRun 'f5xcctl configure' to set up the CLI, or set environment variables:\n  F5XC_API_URL, F5XC_API_TOKEN (or F5XC_API_P12_FILE + F5XC_P12_PASSWORD)", err)
	}

	creds, err := config.LoadCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w\n\nRun 'f5xcctl auth login' to authenticate", err)
	}

	return runtime.NewClient(cfg, creds)
}

func runNamespaceList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Build query parameters
	query := url.Values{}
	if nsLabelFilter != "" {
		query.Set("label_filter", nsLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/api/web/namespaces", query)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var listResp NamespaceListResponse
	if err := resp.DecodeJSON(&listResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Format output
	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, listResp.Items)
	}

	// Table output
	if len(listResp.Items) == 0 {
		output.Infof("No namespaces found")
		return nil
	}

	tableData := make([]NamespaceTableOutput, 0, len(listResp.Items))
	for _, ns := range listResp.Items {
		tableData = append(tableData, NamespaceTableOutput{
			Name:        ns.Name,
			Description: ns.Description,
		})
	}

	return output.Print("table", tableData)
}

func runNamespaceGet(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/web/namespaces/%s", name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var nsResp NamespaceGetResponse
	if err := resp.DecodeJSON(&nsResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Format output
	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, nsResp)
	}

	// Detailed table output for single namespace
	fmt.Printf("Name:        %s\n", nsResp.Metadata.Name)
	fmt.Printf("Description: %s\n", nsResp.Metadata.Description)
	fmt.Printf("Tenant:      %s\n", nsResp.SystemMetadata.Tenant)
	fmt.Printf("UID:         %s\n", nsResp.SystemMetadata.UID)
	if !nsResp.SystemMetadata.CreationTimestamp.IsZero() {
		fmt.Printf("Created:     %s\n", nsResp.SystemMetadata.CreationTimestamp.Format(time.RFC3339))
	}
	if nsResp.Metadata.Disable {
		fmt.Printf("Status:      disabled\n")
	}
	if len(nsResp.Metadata.Labels) > 0 {
		fmt.Println("Labels:")
		for k, v := range nsResp.Metadata.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}

func runNamespaceCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]

	createReq := NamespaceCreateRequest{
		Metadata: NamespaceCreateMetadata{
			Name:        name,
			Namespace:   "", // Must be empty for namespace creation
			Description: nsDescription,
			Labels:      nsLabels,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Post(ctx, "/api/web/namespaces", createReq)
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var nsResp NamespaceResponse
	if err := resp.DecodeJSON(&nsResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, nsResp)
	}

	output.Successf("Namespace %q created successfully", name)
	return nil
}

func runNamespaceDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]

	// Confirmation prompt
	if !nsForce {
		fmt.Printf("Are you sure you want to delete namespace %q? [y/N]: ", name)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// F5XC uses cascade_delete endpoint for namespace deletion
	path := fmt.Sprintf("/api/web/namespaces/%s/cascade_delete", name)
	resp, err := client.Post(ctx, path, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("Namespace %q deleted successfully", name)
	return nil
}
