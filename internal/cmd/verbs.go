package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/f5/f5xcctl/internal/output"
	"github.com/f5/f5xcctl/internal/runtime"
)

// Common flags for verb commands.
var (
	allNamespaces bool
	labelSelector string
	fieldSelector string
	filename      string
	recursive     bool
	dryRun        bool
	force         bool
	gracePeriod   int
	wait          bool
	watchFlag     bool
	showLabels    bool
	// Pagination flags.
	limit     int
	pageToken string
)

// ============================================================================
// GET Command
// ============================================================================

var getCmd = &cobra.Command{
	Use:   "get <resource-type> [name] [flags]",
	Short: "Display one or more resources",
	Long: `Display one or more resources.

Prints a table of the most important information about the specified resources.
You can filter the list using a label selector or get a specific resource by name.

Examples:
  # List all HTTP load balancers in the default namespace
  f5xcctl get http_loadbalancer

  # List all HTTP load balancers in a specific namespace
  f5xcctl get httplb -n production

  # Get a specific load balancer
  f5xcctl get httplb my-lb -n production

  # List all load balancers in all namespaces
  f5xcctl get httplb -A

  # Get output in YAML format
  f5xcctl get httplb my-lb -o yaml

  # List with labels shown
  f5xcctl get httplb --show-labels

  # List namespaces
  f5xcctl get namespace

  # Use short names
  f5xcctl get ns        # namespaces
  f5xcctl get op        # origin_pools
  f5xcctl get af        # app_firewalls
  f5xcctl get cert      # certificates`,
	Args:              cobra.MinimumNArgs(1),
	RunE:              runGet,
	ValidArgsFunction: completeResourceTypes,
}

// ============================================================================
// CREATE Command
// ============================================================================

var createCmd = &cobra.Command{
	Use:   "create <resource-type> <name> [flags]",
	Short: "Create a resource from a file or stdin",
	Long: `Create a resource from a file or from stdin.

JSON and YAML formats are accepted.

Examples:
  # Create a load balancer from a YAML file
  f5xcctl create -f loadbalancer.yaml

  # Create from stdin
  cat lb.yaml | f5xcctl create -f -

  # Create with explicit type and name (uses defaults)
  f5xcctl create http_loadbalancer my-lb -n production`,
	RunE: runCreate,
}

// ============================================================================
// DELETE Command
// ============================================================================

var deleteCmd = &cobra.Command{
	Use:   "delete <resource-type> <name> [flags]",
	Short: "Delete resources by name or from file",
	Long: `Delete resources by resource type and name, or from files.

Examples:
  # Delete a load balancer
  f5xcctl delete http_loadbalancer my-lb -n production

  # Delete using short name
  f5xcctl delete httplb my-lb -n production

  # Delete from a file
  f5xcctl delete -f loadbalancer.yaml

  # Delete multiple resources
  f5xcctl delete httplb lb1 lb2 lb3 -n production

  # Force delete without confirmation
  f5xcctl delete httplb my-lb -n production --force`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDelete,
}

// ============================================================================
// APPLY Command
// ============================================================================

var applyCmd = &cobra.Command{
	Use:   "apply -f <filename> [flags]",
	Short: "Apply a configuration to a resource by file",
	Long: `Apply a configuration to a resource by filename.

The resource will be created if it doesn't exist, or updated if it does.
JSON and YAML formats are accepted.

Examples:
  # Apply a configuration from a file
  f5xcctl apply -f loadbalancer.yaml

  # Apply from stdin
  cat lb.yaml | f5xcctl apply -f -

  # Apply all YAML files in a directory
  f5xcctl apply -f ./configs/ -R

  # Dry run - show what would be applied
  f5xcctl apply -f loadbalancer.yaml --dry-run`,
	RunE: runApply,
}

// ============================================================================
// REPLACE Command
// ============================================================================

var replaceCmd = &cobra.Command{
	Use:   "replace -f <filename> [flags]",
	Short: "Replace a resource by filename",
	Long: `Replace a resource by filename.

The resource must exist. JSON and YAML formats are accepted.

Examples:
  # Replace a load balancer configuration
  f5xcctl replace -f loadbalancer.yaml

  # Replace from stdin
  cat lb.yaml | f5xcctl replace -f -`,
	RunE: runReplace,
}

// ============================================================================
// DESCRIBE Command
// ============================================================================

var describeCmd = &cobra.Command{
	Use:   "describe <resource-type> <name> [flags]",
	Short: "Show details of a specific resource",
	Long: `Show detailed information about a specific resource.

Prints a detailed description of the selected resource, including
related resources, events, and status conditions.

Examples:
  # Describe a load balancer
  f5xcctl describe http_loadbalancer my-lb -n production

  # Describe using short name
  f5xcctl describe httplb my-lb -n production`,
	Args: cobra.ExactArgs(2),
	RunE: runDescribe,
}

// ============================================================================
// LABEL Command
// ============================================================================

var labelCmd = &cobra.Command{
	Use:   "label <resource-type> <name> <key>=<value>... [flags]",
	Short: "Update labels on a resource",
	Long: `Update the labels on a resource.

A label key and value must be specified. To remove a label, use the key with
a trailing dash (e.g., 'key-').

Examples:
  # Add labels to a load balancer
  f5xcctl label httplb my-lb env=production team=platform -n production

  # Remove a label
  f5xcctl label httplb my-lb env- -n production

  # Overwrite existing labels
  f5xcctl label httplb my-lb env=staging --overwrite -n production`,
	Args: cobra.MinimumNArgs(3),
	RunE: runLabel,
}

// ============================================================================
// API-RESOURCES Command
// ============================================================================

var apiResourcesCmd = &cobra.Command{
	Use:   "api-resources",
	Short: "Print the supported API resources",
	Long: `Print the supported API resources on the server.

Examples:
  # List all resources
  f5xcctl api-resources

  # List resources with short names
  f5xcctl api-resources --short

  # List resources in a specific group
  f5xcctl api-resources --group security`,
	RunE: runAPIResources,
}

var apiResourcesGroup string
var apiResourcesShort bool

func init() {
	// GET flags
	getCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "List resources across all namespaces")
	getCmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "Label selector (e.g., 'env=prod,team=platform')")
	getCmd.Flags().StringVar(&fieldSelector, "field-selector", "", "Field selector (e.g., 'metadata.name=my-lb')")
	getCmd.Flags().BoolVar(&showLabels, "show-labels", false, "Show labels in output")
	getCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch for changes")
	getCmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of resources to list (0 = unlimited)")
	getCmd.Flags().StringVar(&pageToken, "page-token", "", "Token for paginated results (provided in previous response)")

	// CREATE flags
	createCmd.Flags().StringVarP(&filename, "filename", "f", "", "Filename, directory, or URL to files to create")
	createCmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Process the directory recursively")
	createCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be created")

	// DELETE flags
	deleteCmd.Flags().StringVarP(&filename, "filename", "f", "", "Filename to delete resources from")
	deleteCmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	deleteCmd.Flags().IntVar(&gracePeriod, "grace-period", -1, "Seconds to wait before force deletion")
	deleteCmd.Flags().BoolVar(&wait, "wait", false, "Wait for deletion to complete")

	// APPLY flags
	applyCmd.Flags().StringVarP(&filename, "filename", "f", "", "Filename, directory, or URL to files (required)")
	applyCmd.Flags().BoolVarP(&recursive, "recursive", "R", false, "Process the directory recursively")
	applyCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be applied")
	_ = applyCmd.MarkFlagRequired("filename")

	// REPLACE flags
	replaceCmd.Flags().StringVarP(&filename, "filename", "f", "", "Filename to replace resource from (required)")
	replaceCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be replaced")
	_ = replaceCmd.MarkFlagRequired("filename")

	// LABEL flags
	labelCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be changed")
	labelCmd.Flags().BoolVar(&force, "overwrite", false, "Overwrite existing label values")

	// API-RESOURCES flags
	apiResourcesCmd.Flags().StringVar(&apiResourcesGroup, "group", "", "Filter by resource group")
	apiResourcesCmd.Flags().BoolVar(&apiResourcesShort, "short", false, "Show short names only")

	// Add commands to root
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(replaceCmd)
	rootCmd.AddCommand(describeCmd)
	rootCmd.AddCommand(labelCmd)
	rootCmd.AddCommand(apiResourcesCmd)
}

// ============================================================================
// Command Implementations
// ============================================================================

func runGet(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	var resourceName string
	if len(args) > 1 {
		resourceName = args[1]
	}

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s\n\nUse 'f5xcctl api-resources' to list available resource types", resourceType)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ns := namespace
	if rt.Namespaced && ns == "" && !allNamespaces {
		ns = "default"
	}

	// If getting a specific resource
	if resourceName != "" {
		path := rt.GetItemPath(ns, resourceName)
		resp, err := client.Get(ctx, path, nil)
		if err != nil {
			return fmt.Errorf("failed to get %s %q: %w", rt.Name, resourceName, err)
		}
		if err := resp.Error(); err != nil {
			return err
		}

		var result map[string]interface{}
		if err := resp.DecodeJSON(&result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		return printResource(result, rt)
	}

	// Watch mode
	if watchFlag {
		return watchResources(client, rt, ns, resourceName)
	}

	// List resources
	if allNamespaces && rt.Namespaced {
		return listAllNamespaces(ctx, client, rt)
	}

	path := rt.GetAPIPath(ns)

	// Build query params
	params := make(map[string]string)

	// Label selector
	if labelSelector != "" {
		params["label_filter"] = convertLabelSelector(labelSelector)
	}

	// Pagination parameters
	if limit > 0 {
		params["page_size"] = fmt.Sprintf("%d", limit)
	}
	if pageToken != "" {
		params["page_token"] = pageToken
	}

	resp, err := client.Get(ctx, path, convertToURLValues(params))
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", rt.Plural, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	var result map[string]interface{}
	if err := resp.DecodeJSON(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Apply client-side label filtering if needed (fallback)
	if labelSelector != "" {
		result = filterByLabelSelector(result, labelSelector)
	}

	// Apply client-side field selector filtering
	if fieldSelector != "" {
		result = filterByFieldSelector(result, fieldSelector)
	}

	if err := printResourceList(result, rt, ns); err != nil {
		return err
	}

	// Display pagination info if there's more data
	if nextToken, ok := result["next_page_token"].(string); ok && nextToken != "" {
		fmt.Printf("\n--- More results available. Use: --page-token=%s ---\n", nextToken)
	}

	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	if filename != "" {
		return createFromFile(filename)
	}

	if len(args) < 2 {
		return fmt.Errorf("resource type and name required when not using -f flag\n\nUsage: f5xcctl create <resource-type> <name> [flags]")
	}

	resourceType := args[0]
	resourceName := args[1]

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	ns := namespace
	if rt.Namespaced && ns == "" {
		ns = "default"
	}

	// Create minimal resource
	resource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      resourceName,
			"namespace": ns,
		},
		"spec": map[string]interface{}{},
	}

	if dryRun {
		fmt.Printf("Would create %s %q in namespace %q\n", rt.Name, resourceName, ns)
		return output.Print("yaml", resource)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := rt.GetAPIPath(ns)
	resp, err := client.Post(ctx, path, resource)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", rt.Name, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s created", rt.Name, resourceName)
	return nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	if filename != "" {
		return deleteFromFile(filename)
	}

	if len(args) < 2 {
		return fmt.Errorf("resource type and name required\n\nUsage: f5xcctl delete <resource-type> <name>... [flags]")
	}

	resourceType := args[0]
	resourceNames := args[1:]

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	ns := namespace
	if rt.Namespaced && ns == "" {
		ns = "default"
	}

	// Confirmation
	if !force {
		fmt.Printf("You are about to delete the following %s:\n", rt.Plural)
		for _, name := range resourceNames {
			fmt.Printf("  - %s/%s (namespace: %s)\n", rt.Name, name, ns)
		}
		fmt.Print("\nAre you sure? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle grace period
	if gracePeriod > 0 {
		output.Infof("Grace period: %d seconds before deletion", gracePeriod)
		for i := gracePeriod; i > 0; i-- {
			fmt.Printf("\rDeleting in %d seconds (Ctrl+C to cancel)...", i)
			time.Sleep(1 * time.Second)
		}
		fmt.Println()
	} else if gracePeriod == 0 {
		output.Infof("Immediate deletion (grace-period=0)")
	}

	var errors []string
	for _, name := range resourceNames {
		path := rt.GetItemPath(ns, name)

		// Special handling for namespaces - use cascade_delete
		if rt.Name == "namespace" {
			path = fmt.Sprintf("/api/web/namespaces/%s/cascade_delete", name)
			resp, err := client.Post(ctx, path, map[string]interface{}{})
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			if err := resp.Error(); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
		} else {
			resp, err := client.Delete(ctx, path)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			if err := resp.Error(); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
		}
		output.Successf("%s/%s deleted", rt.Name, name)

		// Wait for deletion if --wait flag is set
		if wait {
			if err := waitForResourceDeletion(client, rt, ns, name, 5*time.Minute); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some resources failed to delete:\n  %s", strings.Join(errors, "\n  "))
	}
	return nil
}

// waitForResourceDeletion waits until the resource no longer exists.
func waitForResourceDeletion(client *runtime.Client, rt *ResourceType, ns, name string, timeout time.Duration) error {
	output.Infof("Waiting for %s/%s to be fully deleted...", rt.Name, name)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for deletion")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		path := rt.GetItemPath(ns, name)
		resp, err := client.Get(ctx, path, nil)
		cancel()

		// If we get an error or the resource doesn't exist, it's deleted
		if err != nil || !resp.IsSuccess() {
			output.Successf("%s/%s fully deleted", rt.Name, name)
			return nil
		}

		<-ticker.C
	}
}

func runApply(cmd *cobra.Command, args []string) error {
	if filename == "" {
		return fmt.Errorf("filename is required\n\nUsage: f5xcctl apply -f <filename>")
	}
	return applyFromFile(filename, false)
}

func runReplace(cmd *cobra.Command, args []string) error {
	if filename == "" {
		return fmt.Errorf("filename is required\n\nUsage: f5xcctl replace -f <filename>")
	}
	return applyFromFile(filename, true)
}

func runDescribe(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	resourceName := args[1]

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if rt.Namespaced && ns == "" {
		ns = "default"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := rt.GetItemPath(ns, resourceName)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s %q: %w", rt.Name, resourceName, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	var result map[string]interface{}
	if err := resp.DecodeJSON(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return printDescribe(result, rt, resourceName)
}

func runLabel(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	resourceName := args[1]
	labelArgs := args[2:]

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	// Parse labels
	addLabels := make(map[string]string)
	removeLabels := []string{}

	for _, arg := range labelArgs {
		if strings.HasSuffix(arg, "-") {
			// Remove label
			removeLabels = append(removeLabels, strings.TrimSuffix(arg, "-"))
		} else if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			addLabels[parts[0]] = parts[1]
		} else {
			return fmt.Errorf("invalid label format: %s (expected key=value or key-)", arg)
		}
	}

	if dryRun {
		fmt.Printf("Would update labels on %s/%s:\n", rt.Name, resourceName)
		if len(addLabels) > 0 {
			fmt.Println("  Add:")
			for k, v := range addLabels {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(removeLabels) > 0 {
			fmt.Println("  Remove:")
			for _, k := range removeLabels {
				fmt.Printf("    %s\n", k)
			}
		}
		return nil
	}

	// TODO: Implement label update via PATCH
	output.Warningf("Label command implementation pending - use 'apply' with updated YAML for now")
	return nil
}

func runAPIResources(cmd *cobra.Command, args []string) error {
	groups := GetResourceGroups()

	if apiResourcesShort {
		fmt.Println("SHORT\tNAME")
		for _, rt := range ResourceRegistry {
			if apiResourcesGroup != "" && rt.Group != apiResourcesGroup {
				continue
			}
			short := rt.Short
			if short == "" {
				short = "-"
			}
			fmt.Printf("%s\t%s\n", short, rt.Name)
		}
		return nil
	}

	fmt.Println("NAME\t\t\t\tSHORTNAMES\tNAMESPACED\tGROUP\t\tDESCRIPTION")
	for group, resources := range groups {
		if apiResourcesGroup != "" && group != apiResourcesGroup {
			continue
		}
		for _, rt := range resources {
			short := rt.Short
			if short == "" {
				short = "-"
			}
			namespaced := "true"
			if !rt.Namespaced {
				namespaced = "false"
			}
			fmt.Printf("%-24s\t%s\t\t%s\t\t%-12s\t%s\n",
				rt.Name, short, namespaced, rt.Group, rt.Description)
		}
	}
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func createFromFile(filename string) error {
	resources, err := readResourceFile(filename)
	if err != nil {
		return err
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, resource := range resources {
		if err := createResource(ctx, client, resource); err != nil {
			return err
		}
	}
	return nil
}

func deleteFromFile(filename string) error {
	resources, err := readResourceFile(filename)
	if err != nil {
		return err
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, resource := range resources {
		if err := deleteResource(ctx, client, resource); err != nil {
			return err
		}
	}
	return nil
}

func applyFromFile(filename string, replaceOnly bool) error {
	resources, err := readResourceFile(filename)
	if err != nil {
		return err
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, resource := range resources {
		if err := applyResource(ctx, client, resource, replaceOnly); err != nil {
			return err
		}
	}
	return nil
}

func readResourceFile(filename string) ([]map[string]interface{}, error) {
	var data []byte
	var err error

	if filename == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(filename)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try YAML first (handles JSON too)
	var resources []map[string]interface{}

	// Split by YAML document separator
	docs := strings.Split(string(data), "---")
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var resource map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &resource); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		if len(resource) > 0 {
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

func createResource(ctx context.Context, client *runtime.Client, resource map[string]interface{}) error {
	kind, ns, name, err := extractResourceInfo(resource)
	if err != nil {
		return err
	}

	rt := ResolveResourceType(kind)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", kind)
	}

	if dryRun {
		fmt.Printf("Would create %s/%s in namespace %s\n", rt.Name, name, ns)
		return nil
	}

	path := rt.GetAPIPath(ns)
	resp, err := client.Post(ctx, path, resource)
	if err != nil {
		return fmt.Errorf("failed to create %s/%s: %w", rt.Name, name, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s created", rt.Name, name)
	return nil
}

func deleteResource(ctx context.Context, client *runtime.Client, resource map[string]interface{}) error {
	kind, ns, name, err := extractResourceInfo(resource)
	if err != nil {
		return err
	}

	rt := ResolveResourceType(kind)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", kind)
	}

	path := rt.GetItemPath(ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete %s/%s: %w", rt.Name, name, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s deleted", rt.Name, name)
	return nil
}

func applyResource(ctx context.Context, client *runtime.Client, resource map[string]interface{}, replaceOnly bool) error {
	kind, ns, name, err := extractResourceInfo(resource)
	if err != nil {
		return err
	}

	rt := ResolveResourceType(kind)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", kind)
	}

	if dryRun {
		fmt.Printf("Would apply %s/%s in namespace %s\n", rt.Name, name, ns)
		return nil
	}

	// Check if resource exists
	path := rt.GetItemPath(ns, name)
	resp, err := client.Get(ctx, path, nil)

	exists := err == nil && resp.IsSuccess()

	if replaceOnly && !exists {
		return fmt.Errorf("%s/%s does not exist (use 'apply' to create)", rt.Name, name)
	}

	if exists {
		// Update
		resp, err := client.Put(ctx, path, resource)
		if err != nil {
			return fmt.Errorf("failed to update %s/%s: %w", rt.Name, name, err)
		}
		if err := resp.Error(); err != nil {
			return err
		}
		output.Successf("%s/%s configured", rt.Name, name)
	} else {
		// Create
		basePath := rt.GetAPIPath(ns)
		resp, err := client.Post(ctx, basePath, resource)
		if err != nil {
			return fmt.Errorf("failed to create %s/%s: %w", rt.Name, name, err)
		}
		if err := resp.Error(); err != nil {
			return err
		}
		output.Successf("%s/%s created", rt.Name, name)
	}

	return nil
}

func extractResourceInfo(resource map[string]interface{}) (kind, namespace, name string, err error) {
	// Get kind
	if k, ok := resource["kind"].(string); ok {
		kind = k
	} else {
		err = fmt.Errorf("resource missing 'kind' field")
		return
	}

	// Get metadata
	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		err = fmt.Errorf("resource missing 'metadata' field")
		return
	}

	// Get name
	if n, ok := metadata["name"].(string); ok {
		name = n
	} else {
		err = fmt.Errorf("resource missing 'metadata.name' field")
		return
	}

	// Get namespace (optional)
	if ns, ok := metadata["namespace"].(string); ok {
		namespace = ns
	} else {
		namespace = "default"
	}

	return
}

func listAllNamespaces(ctx context.Context, client *runtime.Client, rt *ResourceType) error {
	// First get all namespaces
	resp, err := client.Get(ctx, "/api/web/namespaces", nil)
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	var nsResp struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := resp.DecodeJSON(&nsResp); err != nil {
		return err
	}

	fmt.Printf("NAMESPACE\tNAME\n")
	for _, ns := range nsResp.Items {
		path := rt.GetAPIPath(ns.Name)
		resp, err := client.Get(ctx, path, nil)
		if err != nil || !resp.IsSuccess() {
			continue
		}

		var listResp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := resp.DecodeJSON(&listResp); err != nil {
			continue
		}

		for _, item := range listResp.Items {
			name := extractName(item)
			fmt.Printf("%s\t%s\n", ns.Name, name)
		}
	}
	return nil
}

//nolint:unparam // rt kept for future resource-specific formatting
func printResource(resource map[string]interface{}, rt *ResourceType) error {
	if outputFmt == "json" {
		data, _ := json.MarshalIndent(resource, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if outputFmt == "yaml" {
		data, _ := yaml.Marshal(resource)
		fmt.Println(string(data))
		return nil
	}

	// Table format
	name := extractName(resource)
	fmt.Printf("NAME\n%s\n", name)
	return nil
}

//nolint:unparam // error return kept for API consistency
func printResourceList(result map[string]interface{}, rt *ResourceType, ns string) error {
	items, ok := result["items"].([]interface{})
	if !ok {
		items = []interface{}{}
	}

	if outputFmt == "json" {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if outputFmt == "yaml" {
		data, _ := yaml.Marshal(result)
		fmt.Println(string(data))
		return nil
	}

	if len(items) == 0 {
		output.Infof("No resources found in namespace %q", ns)
		return nil
	}

	// Output format: name
	if outputFmt == "name" {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				name := extractName(m)
				fmt.Printf("%s/%s\n", rt.Name, name)
			}
		}
		return nil
	}

	// Output format: wide - shows additional columns
	if outputFmt == "wide" {
		if rt.Namespaced {
			fmt.Println("NAME\t\t\t\tNAMESPACE\tUID\t\t\t\t\t\tCREATED")
		} else {
			fmt.Println("NAME\t\t\t\tUID\t\t\t\t\t\tCREATED")
		}
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				name := extractName(m)
				uid := extractUID(m)
				created := extractCreated(m)
				if rt.Namespaced {
					itemNs := extractNamespace(m)
					fmt.Printf("%-24s\t%s\t%s\t%s\n", name, itemNs, uid, created)
				} else {
					fmt.Printf("%-24s\t%s\t%s\n", name, uid, created)
				}
			}
		}
		return nil
	}

	// Table output (default)
	if showLabels {
		fmt.Println("NAME\t\t\tLABELS")
	} else {
		fmt.Println("NAME")
	}

	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			name := extractName(m)
			if showLabels {
				labels := extractLabels(m)
				fmt.Printf("%s\t\t%s\n", name, labels)
			} else {
				fmt.Println(name)
			}
		}
	}
	return nil
}

//nolint:unparam // error return kept for API consistency
func printDescribe(resource map[string]interface{}, rt *ResourceType, name string) error {
	fmt.Printf("Name:         %s\n", name)
	fmt.Printf("Kind:         %s\n", rt.Kind)

	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "" {
			fmt.Printf("Namespace:    %s\n", ns)
		}
		if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
			fmt.Println("Labels:")
			for k, v := range labels {
				fmt.Printf("              %s=%v\n", k, v)
			}
		}
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
			fmt.Println("Annotations:")
			for k, v := range annotations {
				fmt.Printf("              %s=%v\n", k, v)
			}
		}
	}

	if sysMeta, ok := resource["system_metadata"].(map[string]interface{}); ok {
		if uid, ok := sysMeta["uid"].(string); ok {
			fmt.Printf("UID:          %s\n", uid)
		}
		if created, ok := sysMeta["creation_timestamp"].(string); ok {
			fmt.Printf("Created:      %s\n", created)
		}
	}

	fmt.Println("\nSpec:")
	if spec, ok := resource["spec"].(map[string]interface{}); ok {
		data, _ := yaml.Marshal(spec)
		// Indent the spec
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				fmt.Printf("  %s\n", line)
			}
		}
	}

	return nil
}

func extractName(resource map[string]interface{}) string {
	// Try metadata.name first
	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			return name
		}
	}
	// Try top-level name
	if name, ok := resource["name"].(string); ok {
		return name
	}
	return "<unknown>"
}

func extractLabels(resource map[string]interface{}) string {
	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			var parts []string
			for k, v := range labels {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			return strings.Join(parts, ",")
		}
	}
	// Try top-level labels
	if labels, ok := resource["labels"].(map[string]interface{}); ok {
		var parts []string
		for k, v := range labels {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		return strings.Join(parts, ",")
	}
	return ""
}

func extractUID(resource map[string]interface{}) string {
	// Try system_metadata.uid first
	if sysMeta, ok := resource["system_metadata"].(map[string]interface{}); ok {
		if uid, ok := sysMeta["uid"].(string); ok {
			return uid
		}
	}
	// Try top-level uid
	if uid, ok := resource["uid"].(string); ok {
		return uid
	}
	return "<none>"
}

func extractCreated(resource map[string]interface{}) string {
	// Try system_metadata.creation_timestamp first
	if sysMeta, ok := resource["system_metadata"].(map[string]interface{}); ok {
		if created, ok := sysMeta["creation_timestamp"].(string); ok {
			// Parse and format the timestamp
			if t, err := time.Parse(time.RFC3339Nano, created); err == nil {
				return t.Format("2006-01-02T15:04:05Z")
			}
			return created
		}
	}
	return "<unknown>"
}

func extractNamespace(resource map[string]interface{}) string {
	// Try metadata.namespace first
	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if ns, ok := metadata["namespace"].(string); ok && ns != "" {
			return ns
		}
	}
	// Try top-level namespace
	if ns, ok := resource["namespace"].(string); ok && ns != "" {
		return ns
	}
	return "default"
}

// watchResources implements the watch mode for the get command
// It polls the API every 2 seconds and refreshes the display.
func watchResources(client *runtime.Client, rt *ResourceType, ns, resourceName string) error {
	// Set up signal handling for graceful exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Channel to signal stop
	stopChan := make(chan struct{})
	go func() {
		<-sigChan
		close(stopChan)
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Track previous state for change detection
	var previousHash string

	// Fetch and display function
	fetchAndDisplay := func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var result map[string]interface{}

		if resourceName != "" {
			// Watch a specific resource
			path := rt.GetItemPath(ns, resourceName)
			resp, err := client.Get(ctx, path, nil)
			if err != nil {
				return "", fmt.Errorf("failed to get %s %q: %w", rt.Name, resourceName, err)
			}
			if err := resp.Error(); err != nil {
				return "", err
			}
			if err := resp.DecodeJSON(&result); err != nil {
				return "", fmt.Errorf("failed to decode response: %w", err)
			}
		} else {
			// Watch list of resources
			if allNamespaces && rt.Namespaced {
				// For all namespaces, we handle it differently
				return watchAllNamespaces(ctx, client, rt)
			}

			path := rt.GetAPIPath(ns)
			resp, err := client.Get(ctx, path, nil)
			if err != nil {
				return "", fmt.Errorf("failed to list %s: %w", rt.Plural, err)
			}
			if err := resp.Error(); err != nil {
				return "", err
			}
			if err := resp.DecodeJSON(&result); err != nil {
				return "", fmt.Errorf("failed to decode response: %w", err)
			}
		}

		// Create hash of the result for change detection
		jsonData, _ := json.Marshal(result)
		currentHash := fmt.Sprintf("%x", jsonData)

		return currentHash, nil
	}

	// Initial display
	clearScreen()
	printWatchHeader(rt, ns, resourceName)

	hash, err := fetchAndDisplay()
	if err != nil {
		return err
	}
	previousHash = hash

	if resourceName != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		path := rt.GetItemPath(ns, resourceName)
		resp, _ := client.Get(ctx, path, nil)
		var result map[string]interface{}
		_ = resp.DecodeJSON(&result)
		_ = printResource(result, rt)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		path := rt.GetAPIPath(ns)
		resp, _ := client.Get(ctx, path, nil)
		var result map[string]interface{}
		_ = resp.DecodeJSON(&result)
		_ = printResourceList(result, rt, ns)
	}

	fmt.Printf("\n--- Last updated: %s (Ctrl+C to exit) ---\n", time.Now().Format("15:04:05"))

	// Watch loop
	for {
		select {
		case <-stopChan:
			fmt.Println("\nWatch stopped.")
			return nil
		case <-ticker.C:
			currentHash, err := fetchAndDisplay()
			if err != nil {
				output.Warningf("Error fetching resources: %v", err)
				continue
			}

			// Only refresh display if something changed
			if currentHash != previousHash {
				clearScreen()
				printWatchHeader(rt, ns, resourceName)

				if resourceName != "" {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					path := rt.GetItemPath(ns, resourceName)
					resp, _ := client.Get(ctx, path, nil)
					var result map[string]interface{}
					_ = resp.DecodeJSON(&result)
					_ = printResource(result, rt)
					cancel()
				} else {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					path := rt.GetAPIPath(ns)
					resp, _ := client.Get(ctx, path, nil)
					var result map[string]interface{}
					_ = resp.DecodeJSON(&result)
					_ = printResourceList(result, rt, ns)
					cancel()
				}

				previousHash = currentHash
				fmt.Printf("\n--- Updated: %s (Ctrl+C to exit) ---\n", time.Now().Format("15:04:05"))
			} else {
				// Just update the timestamp
				fmt.Printf("\r--- Last checked: %s (Ctrl+C to exit) ---", time.Now().Format("15:04:05"))
			}
		}
	}
}

// watchAllNamespaces handles watch mode for all namespaces.
func watchAllNamespaces(ctx context.Context, client *runtime.Client, rt *ResourceType) (string, error) {
	// First get all namespaces
	resp, err := client.Get(ctx, "/api/web/namespaces", nil)
	if err != nil {
		return "", fmt.Errorf("failed to list namespaces: %w", err)
	}

	var nsResp struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := resp.DecodeJSON(&nsResp); err != nil {
		return "", err
	}

	var allItems []map[string]interface{}
	for _, ns := range nsResp.Items {
		path := rt.GetAPIPath(ns.Name)
		resp, err := client.Get(ctx, path, nil)
		if err != nil || !resp.IsSuccess() {
			continue
		}

		var listResp struct {
			Items []map[string]interface{} `json:"items"`
		}
		if err := resp.DecodeJSON(&listResp); err != nil {
			continue
		}

		for _, item := range listResp.Items {
			// Add namespace info
			item["_namespace"] = ns.Name
			allItems = append(allItems, item)
		}
	}

	// Create hash for change detection
	jsonData, _ := json.Marshal(allItems)
	return fmt.Sprintf("%x", jsonData), nil
}

// clearScreen clears the terminal screen.
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

// printWatchHeader prints the header for watch mode.
func printWatchHeader(rt *ResourceType, ns, resourceName string) {
	if resourceName != "" {
		fmt.Printf("Watching %s/%s", rt.Name, resourceName)
	} else {
		fmt.Printf("Watching %s", rt.Plural)
	}
	if ns != "" && ns != "default" {
		fmt.Printf(" in namespace %q", ns)
	} else if allNamespaces {
		fmt.Print(" in all namespaces")
	}
	fmt.Println()
	fmt.Println()
}

// convertToURLValues converts a map to url.Values.
func convertToURLValues(params map[string]string) url.Values {
	if params == nil {
		return nil
	}
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}

// convertLabelSelector converts kubectl-style label selectors to F5XC format
// kubectl: "env=prod,team=platform" or "env in (prod,staging)"
// F5XC: Uses label_filter parameter format.
func convertLabelSelector(selector string) string {
	// F5XC API may accept different formats; for now, pass through
	// The API typically expects: key=value format
	return selector
}

// filterByLabelSelector filters resources by label selector (client-side fallback)
// Supports:
//   - equality: key=value, key==value
//   - inequality: key!=value
//   - set-based: key in (val1,val2), key notin (val1,val2), key, !key
func filterByLabelSelector(result map[string]interface{}, selector string) map[string]interface{} {
	items, ok := result["items"].([]interface{})
	if !ok {
		return result
	}

	// Parse selector into conditions
	conditions := parseLabelSelector(selector)
	if len(conditions) == 0 {
		return result
	}

	var filtered []interface{}
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			labels := extractLabelsMap(m)
			if matchesLabelConditions(labels, conditions) {
				filtered = append(filtered, item)
			}
		}
	}

	result["items"] = filtered
	return result
}

// labelCondition represents a parsed label selector condition.
type labelCondition struct {
	key      string
	operator string // "=", "==", "!=", "in", "notin", "exists", "notexists"
	values   []string
}

// parseLabelSelector parses a kubectl-style label selector string.
func parseLabelSelector(selector string) []labelCondition {
	var conditions []labelCondition

	// Split by comma (respecting parentheses)
	parts := splitLabelSelector(selector)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		cond := labelCondition{}

		// Check for "in" or "notin"
		if strings.Contains(part, " in (") || strings.Contains(part, " in(") {
			idx := strings.Index(part, " in")
			cond.key = strings.TrimSpace(part[:idx])
			cond.operator = "in"
			valuesStr := part[idx+3:]
			valuesStr = strings.Trim(valuesStr, " ()")
			for _, v := range strings.Split(valuesStr, ",") {
				cond.values = append(cond.values, strings.TrimSpace(v))
			}
		} else if strings.Contains(part, " notin (") || strings.Contains(part, " notin(") {
			idx := strings.Index(part, " notin")
			cond.key = strings.TrimSpace(part[:idx])
			cond.operator = "notin"
			valuesStr := part[idx+7:]
			valuesStr = strings.Trim(valuesStr, " ()")
			for _, v := range strings.Split(valuesStr, ",") {
				cond.values = append(cond.values, strings.TrimSpace(v))
			}
		} else if strings.HasPrefix(part, "!") {
			// !key - key must not exist
			cond.key = strings.TrimPrefix(part, "!")
			cond.operator = "notexists"
		} else if strings.Contains(part, "!=") {
			// key!=value
			kv := strings.SplitN(part, "!=", 2)
			cond.key = strings.TrimSpace(kv[0])
			cond.operator = "!="
			if len(kv) > 1 {
				cond.values = []string{strings.TrimSpace(kv[1])}
			}
		} else if strings.Contains(part, "==") {
			// key==value
			kv := strings.SplitN(part, "==", 2)
			cond.key = strings.TrimSpace(kv[0])
			cond.operator = "="
			if len(kv) > 1 {
				cond.values = []string{strings.TrimSpace(kv[1])}
			}
		} else if strings.Contains(part, "=") {
			// key=value
			kv := strings.SplitN(part, "=", 2)
			cond.key = strings.TrimSpace(kv[0])
			cond.operator = "="
			if len(kv) > 1 {
				cond.values = []string{strings.TrimSpace(kv[1])}
			}
		} else {
			// key - key must exist
			cond.key = part
			cond.operator = "exists"
		}

		if cond.key != "" {
			conditions = append(conditions, cond)
		}
	}

	return conditions
}

// splitLabelSelector splits a selector by comma while respecting parentheses.
func splitLabelSelector(selector string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, ch := range selector {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// extractLabelsMap extracts labels as a map from a resource.
func extractLabelsMap(resource map[string]interface{}) map[string]string {
	result := make(map[string]string)

	// Try metadata.labels
	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			for k, v := range labels {
				result[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Try top-level labels
	if labels, ok := resource["labels"].(map[string]interface{}); ok {
		for k, v := range labels {
			result[k] = fmt.Sprintf("%v", v)
		}
	}

	return result
}

// matchesLabelConditions checks if labels match all conditions.
func matchesLabelConditions(labels map[string]string, conditions []labelCondition) bool {
	for _, cond := range conditions {
		if !matchesLabelCondition(labels, cond) {
			return false
		}
	}
	return true
}

// matchesLabelCondition checks if labels match a single condition.
func matchesLabelCondition(labels map[string]string, cond labelCondition) bool {
	value, exists := labels[cond.key]

	switch cond.operator {
	case "=", "==":
		return exists && len(cond.values) > 0 && value == cond.values[0]
	case "!=":
		return !exists || (len(cond.values) > 0 && value != cond.values[0])
	case "in":
		if !exists {
			return false
		}
		for _, v := range cond.values {
			if value == v {
				return true
			}
		}
		return false
	case "notin":
		if !exists {
			return true
		}
		for _, v := range cond.values {
			if value == v {
				return false
			}
		}
		return true
	case "exists":
		return exists
	case "notexists":
		return !exists
	default:
		return true
	}
}

// Completion function for resource types.
func completeResourceTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		// Complete resource type
		var completions []string
		for name, rt := range ResourceRegistry {
			if strings.HasPrefix(name, toComplete) {
				completions = append(completions, name+"\t"+rt.Description)
			}
			if rt.Short != "" && strings.HasPrefix(rt.Short, toComplete) {
				completions = append(completions, rt.Short+"\t"+rt.Description)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// filterByFieldSelector filters resources by field selector (client-side)
// Supports paths like metadata.name=value, metadata.namespace=value, spec.field=value.
func filterByFieldSelector(result map[string]interface{}, selector string) map[string]interface{} {
	items, ok := result["items"].([]interface{})
	if !ok {
		return result
	}

	// Parse field selector into conditions
	conditions := parseFieldSelector(selector)
	if len(conditions) == 0 {
		return result
	}

	var filtered []interface{}
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			if matchesFieldConditions(m, conditions) {
				filtered = append(filtered, item)
			}
		}
	}

	result["items"] = filtered
	return result
}

// fieldCondition represents a parsed field selector condition.
type fieldCondition struct {
	path     string // e.g., "metadata.name"
	operator string // "=" or "!="
	value    string
}

// parseFieldSelector parses a kubectl-style field selector string
// Format: field1=value1,field2!=value2.
func parseFieldSelector(selector string) []fieldCondition {
	var conditions []fieldCondition

	parts := strings.Split(selector, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		cond := fieldCondition{}

		if strings.Contains(part, "!=") {
			kv := strings.SplitN(part, "!=", 2)
			cond.path = strings.TrimSpace(kv[0])
			cond.operator = "!="
			if len(kv) > 1 {
				cond.value = strings.TrimSpace(kv[1])
			}
		} else if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			cond.path = strings.TrimSpace(kv[0])
			cond.operator = "="
			if len(kv) > 1 {
				cond.value = strings.TrimSpace(kv[1])
			}
		}

		if cond.path != "" {
			conditions = append(conditions, cond)
		}
	}

	return conditions
}

// matchesFieldConditions checks if a resource matches all field conditions.
func matchesFieldConditions(resource map[string]interface{}, conditions []fieldCondition) bool {
	for _, cond := range conditions {
		if !matchesFieldCondition(resource, cond) {
			return false
		}
	}
	return true
}

// matchesFieldCondition checks if a resource matches a single field condition.
func matchesFieldCondition(resource map[string]interface{}, cond fieldCondition) bool {
	// Get the field value using the path
	value := getFieldValue(resource, cond.path)
	valueStr := fmt.Sprintf("%v", value)

	switch cond.operator {
	case "=":
		return value != nil && valueStr == cond.value
	case "!=":
		return value == nil || valueStr != cond.value
	default:
		return true
	}
}

// getFieldValue retrieves a value from a nested map using a dot-separated path.
func getFieldValue(resource map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = resource

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}

	return current
}
