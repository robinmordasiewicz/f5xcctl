package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/f5/f5xcctl/internal/output"
)

// Edit command flags.
var (
	editOutputFormat string
	editShowPatch    bool
)

// editCmd represents the edit command.
var editCmd = &cobra.Command{
	Use:   "edit <resource-type> <name> [flags]",
	Short: "Edit a resource on the server",
	Long: `Edit a resource from the default editor.

The edit command allows you to directly edit any API resource. It will open the
editor defined by your KUBE_EDITOR, EDITOR, or F5XC_EDITOR environment variable,
or fall back to 'vi' for Linux or 'notepad' for Windows.

The resource will be opened in the editor as YAML. Upon saving and exiting,
the resource will be updated on the server.

Examples:
  # Edit a load balancer
  f5xcctl edit httplb my-lb -n production

  # Edit a load balancer with a specific editor
  EDITOR=nano f5xcctl edit httplb my-lb -n production

  # Edit from a file (validates and updates)
  f5xcctl edit -f loadbalancer.yaml

  # Show what would be patched without applying
  f5xcctl edit httplb my-lb -n production --output-patch`,
	RunE:              runEdit,
	ValidArgsFunction: completeResourceTypes,
}

func init() {
	editCmd.Flags().StringVarP(&filename, "filename", "f", "", "Filename to use for the resource")
	editCmd.Flags().StringVarP(&editOutputFormat, "output", "o", "yaml", "Output format for editing (yaml, json)")
	editCmd.Flags().BoolVar(&editShowPatch, "output-patch", false, "Show the patch that would be applied")

	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	// Handle file-based edit
	if filename != "" {
		return editFromFile(filename)
	}

	// Handle resource type/name edit
	if len(args) < 2 {
		return fmt.Errorf("resource type and name required\n\nUsage: f5xcctl edit <resource-type> <name> [flags]")
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

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch the current resource
	path := rt.GetItemPath(ns, resourceName)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s %q: %w", rt.Name, resourceName, err)
	}
	if err := resp.Error(); err != nil {
		return err
	}

	var original map[string]interface{}
	if err := resp.DecodeJSON(&original); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Store original resource version for conflict detection
	originalVersion := extractResourceVersion(original)

	// Convert to YAML for editing
	originalYAML, err := yaml.Marshal(original)
	if err != nil {
		return fmt.Errorf("failed to marshal resource to YAML: %w", err)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("f5xcctl-edit-%s-%s-*.yaml", rt.Name, resourceName))
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write original content to temp file
	if _, err := tmpFile.Write(originalYAML); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}
	_ = tmpFile.Close()

	// Get editor command
	editor := getEditor()

	// Loop for editing (allows retries on validation errors)
	for {
		// Open editor with context
		editorCtx, editorCancel := context.WithCancel(context.Background())
		editorCmd := exec.CommandContext(editorCtx, editor, tmpPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr

		runErr := editorCmd.Run()
		editorCancel()
		if runErr != nil {
			return fmt.Errorf("editor failed: %w", runErr)
		}

		// Read edited content
		editedContent, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read edited file: %w", err)
		}

		// Check if content changed
		if bytes.Equal(editedContent, originalYAML) {
			output.Infof("Edit canceled, no changes made")
			return nil
		}

		// Parse edited YAML
		var edited map[string]interface{}
		if err := yaml.Unmarshal(editedContent, &edited); err != nil {
			output.Errorf("YAML parse error: %v", err)
			if !promptRetryEdit() {
				return fmt.Errorf("edit canceled due to parse error")
			}
			continue
		}

		// Show patch if requested
		if editShowPatch {
			fmt.Println("# Changes to be applied:")
			showResourceDiff(original, edited)
			return nil
		}

		// Check for resource version conflict
		currentVersion := extractResourceVersion(edited)
		if originalVersion != "" && currentVersion != "" && originalVersion != currentVersion {
			output.Warningf("Resource version changed from %s to %s", originalVersion, currentVersion)
		}

		// Update the resource
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		updateResp, err := client.Put(ctx, path, edited)
		cancel()

		if err != nil {
			return fmt.Errorf("failed to update %s %q: %w", rt.Name, resourceName, err)
		}

		if apiErr := updateResp.Error(); apiErr != nil {
			output.Errorf("API error: %v", apiErr)
			if !promptRetryEdit() {
				return fmt.Errorf("edit canceled due to API error")
			}
			continue
		}

		output.Successf("%s/%s edited", rt.Name, resourceName)
		return nil
	}
}

// editFromFile handles editing a resource from a file.
func editFromFile(filename string) error {
	resources, err := readResourceFile(filename)
	if err != nil {
		return err
	}

	if len(resources) == 0 {
		return fmt.Errorf("no resources found in file: %s", filename)
	}

	if len(resources) > 1 {
		return fmt.Errorf("edit supports only a single resource at a time, found %d resources", len(resources))
	}

	resource := resources[0]

	kind, ns, name, err := extractResourceInfo(resource)
	if err != nil {
		return err
	}

	rt := ResolveResourceType(kind)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s", kind)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if resource exists
	path := rt.GetItemPath(ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil || !resp.IsSuccess() {
		return fmt.Errorf("%s/%s does not exist - use 'apply' or 'create' instead", rt.Name, name)
	}

	// Update the resource
	updateResp, err := client.Put(ctx, path, resource)
	if err != nil {
		return fmt.Errorf("failed to update %s/%s: %w", rt.Name, name, err)
	}
	if err := updateResp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s edited", rt.Name, name)
	return nil
}

// getEditor returns the editor to use for editing resources.
func getEditor() string {
	// Check environment variables in order of preference
	editors := []string{"F5XC_EDITOR", "KUBE_EDITOR", "EDITOR", "VISUAL"}
	for _, env := range editors {
		if editor := os.Getenv(env); editor != "" {
			return editor
		}
	}

	// Default based on OS
	if os.Getenv("OS") == "Windows_NT" {
		return "notepad"
	}
	return "vi"
}

// extractResourceVersion extracts the resource version from system metadata.
func extractResourceVersion(resource map[string]interface{}) string {
	if sysMeta, ok := resource["system_metadata"].(map[string]interface{}); ok {
		if version, ok := sysMeta["resource_version"].(string); ok {
			return version
		}
		// Try modification_timestamp as alternative version indicator
		if modTime, ok := sysMeta["modification_timestamp"].(string); ok {
			return modTime
		}
	}
	return ""
}

// promptRetryEdit prompts the user to retry editing.
func promptRetryEdit() bool {
	fmt.Print("Would you like to retry editing? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)
	return strings.EqualFold(response, "y")
}

// showResourceDiff shows the differences between original and edited resources.
func showResourceDiff(original, edited map[string]interface{}) {
	origYAML, _ := yaml.Marshal(original)
	editYAML, _ := yaml.Marshal(edited)

	fmt.Println("--- original")
	fmt.Println("+++ edited")
	fmt.Println("---")

	origLines := strings.Split(string(origYAML), "\n")
	editLines := strings.Split(string(editYAML), "\n")

	// Simple line-by-line diff
	maxLines := len(origLines)
	if len(editLines) > maxLines {
		maxLines = len(editLines)
	}

	for i := 0; i < maxLines; i++ {
		var origLine, editLine string
		if i < len(origLines) {
			origLine = origLines[i]
		}
		if i < len(editLines) {
			editLine = editLines[i]
		}

		if origLine != editLine {
			if origLine != "" {
				fmt.Printf("- %s\n", origLine)
			}
			if editLine != "" {
				fmt.Printf("+ %s\n", editLine)
			}
		}
	}
}
