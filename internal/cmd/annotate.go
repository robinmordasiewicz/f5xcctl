package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/output"
)

// Annotate command flags.
var (
	annotateOverwrite bool
	annotateList      bool
)

// annotateCmd represents the annotate command.
var annotateCmd = &cobra.Command{
	Use:   "annotate <resource-type> <name> <key>=<value>... [flags]",
	Short: "Update annotations on a resource",
	Long: `Update the annotations on a resource.

An annotation key and value must be specified. To remove an annotation, use the key with
a trailing dash (e.g., 'key-').

Annotations are key-value pairs that can be used to store arbitrary non-identifying
metadata about resources. Unlike labels, annotations are not used for selection.

Examples:
  # Add annotations to a load balancer
  f5xcctl annotate httplb my-lb description="Main LB" owner="platform-team"

  # Add annotation with namespace
  f5xcctl annotate httplb my-lb -n production description="Production load balancer"

  # Remove an annotation
  f5xcctl annotate httplb my-lb description- -n production

  # Overwrite existing annotations
  f5xcctl annotate httplb my-lb description="Updated description" --overwrite

  # List current annotations
  f5xcctl annotate httplb my-lb --list

  # Dry run - show what would be changed
  f5xcctl annotate httplb my-lb new-key=new-value --dry-run`,
	Args:              cobra.MinimumNArgs(2),
	RunE:              runAnnotate,
	ValidArgsFunction: completeResourceTypes,
}

func init() {
	annotateCmd.Flags().BoolVar(&annotateOverwrite, "overwrite", false, "Overwrite existing annotation values")
	annotateCmd.Flags().BoolVar(&annotateList, "list", false, "List current annotations")
	annotateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be changed")

	rootCmd.AddCommand(annotateCmd)
}

func runAnnotate(cmd *cobra.Command, args []string) error {
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

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current resource
	path := rt.GetItemPath(ns, resourceName)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s %q: %w", rt.Name, resourceName, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	var resource map[string]interface{}
	if err := resp.DecodeJSON(&resource); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Handle --list flag
	if annotateList {
		return listAnnotations(resource, rt, resourceName)
	}

	// Parse annotation args
	if len(args) < 3 {
		return fmt.Errorf("at least one annotation required\n\nUsage: f5xcctl annotate <resource-type> <name> <key>=<value>... [flags]")
	}

	annotationArgs := args[2:]
	addAnnotations := make(map[string]string)
	removeAnnotations := []string{}

	for _, arg := range annotationArgs {
		if strings.HasSuffix(arg, "-") {
			// Remove annotation
			removeAnnotations = append(removeAnnotations, strings.TrimSuffix(arg, "-"))
		} else if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			addAnnotations[parts[0]] = parts[1]
		} else {
			return fmt.Errorf("invalid annotation format: %s (expected key=value or key-)", arg)
		}
	}

	// Get current annotations
	metadata := getOrCreateMap(resource, "metadata")
	annotations := getOrCreateMap(metadata, "annotations")

	// Check for existing annotations (unless overwrite)
	if !annotateOverwrite {
		for key := range addAnnotations {
			if _, exists := annotations[key]; exists {
				return fmt.Errorf("annotation %q already exists (use --overwrite to update)", key)
			}
		}
	}

	// Apply changes
	for key, value := range addAnnotations {
		annotations[key] = value
	}
	for _, key := range removeAnnotations {
		delete(annotations, key)
	}

	metadata["annotations"] = annotations
	resource["metadata"] = metadata

	if dryRun {
		fmt.Printf("Would update annotations on %s/%s:\n", rt.Name, resourceName)
		if len(addAnnotations) > 0 {
			fmt.Println("  Add/Update:")
			for k, v := range addAnnotations {
				fmt.Printf("    %s=%s\n", k, v)
			}
		}
		if len(removeAnnotations) > 0 {
			fmt.Println("  Remove:")
			for _, k := range removeAnnotations {
				fmt.Printf("    %s\n", k)
			}
		}
		return nil
	}

	// Update the resource
	updateResp, err := client.Put(ctx, path, resource)
	if err != nil {
		return fmt.Errorf("failed to update %s %q: %w", rt.Name, resourceName, err)
	}
	if err := updateResp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s annotated", rt.Name, resourceName)
	return nil
}

// listAnnotations displays the current annotations on a resource.
//
//nolint:unparam // error return kept for API consistency with other commands
func listAnnotations(resource map[string]interface{}, rt *ResourceType, name string) error {
	metadata, ok := resource["metadata"].(map[string]interface{})
	if !ok {
		fmt.Printf("%s/%s has no annotations\n", rt.Name, name)
		return nil
	}

	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok || len(annotations) == 0 {
		fmt.Printf("%s/%s has no annotations\n", rt.Name, name)
		return nil
	}

	fmt.Printf("Annotations on %s/%s:\n", rt.Name, name)
	for key, value := range annotations {
		fmt.Printf("  %s=%v\n", key, value)
	}
	return nil
}

// getOrCreateMap gets a map value from a parent map, creating it if necessary.
func getOrCreateMap(parent map[string]interface{}, key string) map[string]interface{} {
	if val, ok := parent[key].(map[string]interface{}); ok {
		return val
	}
	newMap := make(map[string]interface{})
	parent[key] = newMap
	return newMap
}
