package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/f5/f5xcctl/internal/output"
)

var (
	diffFilename   string
	diffServerSide bool
	diffNoColor    bool
)

var diffCmd = &cobra.Command{
	Use:   "diff -f <filename>",
	Short: "Diff local config against live cluster state",
	Long: `Show differences between local configuration and live state.

Compares the local configuration file with the current state of the resource
in the cluster and displays the differences in unified diff format.

Exit codes:
  0 - No differences found
  1 - Differences found
  2 - Error occurred

Examples:
  # Show diff for a file
  f5xcctl diff -f loadbalancer.yaml

  # Diff without color
  f5xcctl diff -f loadbalancer.yaml --no-color

  # Diff multiple resources in a file
  f5xcctl diff -f configs/`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVarP(&diffFilename, "filename", "f", "", "Filename, directory, or URL to files containing the configuration to diff")
	diffCmd.Flags().BoolVar(&diffServerSide, "server-side", false, "Use server-side diff (if supported)")
	diffCmd.Flags().BoolVar(&diffNoColor, "no-color", false, "Disable color output")
	_ = diffCmd.MarkFlagRequired("filename")

	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	if diffFilename == "" {
		return fmt.Errorf("filename is required\n\nUsage: f5xcctl diff -f <filename>")
	}

	resources, err := readResourceFile(diffFilename)
	if err != nil {
		return err
	}

	if len(resources) == 0 {
		return fmt.Errorf("no resources found in %s", diffFilename)
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hasDiff := false

	for _, localResource := range resources {
		kind, ns, name, err := extractResourceInfo(localResource)
		if err != nil {
			output.Warningf("Skipping invalid resource: %v", err)
			continue
		}

		rt := ResolveResourceType(kind)
		if rt == nil {
			output.Warningf("Unknown resource type: %s", kind)
			continue
		}

		fmt.Printf("diff %s/%s\n", rt.Name, name)

		// Fetch current state from API
		path := rt.GetItemPath(ns, name)
		resp, err := client.Get(ctx, path, nil)

		var remoteResource map[string]interface{}
		if err != nil || !resp.IsSuccess() {
			// Resource doesn't exist - show as all new
			fmt.Printf("--- live/%s/%s\t(not found)\n", rt.Name, name)
			fmt.Printf("+++ local/%s/%s\n", rt.Name, name)

			localYAML, _ := yaml.Marshal(normalizeForDiff(localResource))
			for _, line := range strings.Split(string(localYAML), "\n") {
				if line != "" {
					printDiffLine("+", line)
				}
			}
			hasDiff = true
			fmt.Println()
			continue
		}

		if err := resp.DecodeJSON(&remoteResource); err != nil {
			output.Warningf("Failed to decode remote resource: %v", err)
			continue
		}

		// Normalize both resources for comparison
		localNorm := normalizeForDiff(localResource)
		remoteNorm := normalizeForDiff(remoteResource)

		// Generate YAML for comparison
		localYAML, _ := yaml.Marshal(localNorm)
		remoteYAML, _ := yaml.Marshal(remoteNorm)

		// Compare
		if bytes.Equal(localYAML, remoteYAML) {
			output.Infof("%s/%s is up to date", rt.Name, name)
			continue
		}

		hasDiff = true

		// Generate diff
		diff := generateUnifiedDiff(
			fmt.Sprintf("live/%s/%s", rt.Name, name),
			fmt.Sprintf("local/%s/%s", rt.Name, name),
			string(remoteYAML),
			string(localYAML),
		)

		fmt.Println(diff)
	}

	if hasDiff {
		cancel()   // Ensure context is canceled before exit
		os.Exit(1) //nolint:gocritic // exitAfterDefer: cancel() is called explicitly above
	}

	return nil
}

// normalizeForDiff normalizes a resource for diff comparison
// Removes fields that change between apply and get (like system_metadata).
func normalizeForDiff(resource map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy relevant fields
	if kind, ok := resource["kind"].(string); ok {
		result["kind"] = kind
	}

	if metadata, ok := resource["metadata"].(map[string]interface{}); ok {
		normMetadata := make(map[string]interface{})
		if name, ok := metadata["name"].(string); ok {
			normMetadata["name"] = name
		}
		if ns, ok := metadata["namespace"].(string); ok {
			normMetadata["namespace"] = ns
		}
		if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
			normMetadata["labels"] = labels
		}
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
			normMetadata["annotations"] = annotations
		}
		if description, ok := metadata["description"].(string); ok && description != "" {
			normMetadata["description"] = description
		}
		result["metadata"] = normMetadata
	}

	if spec, ok := resource["spec"].(map[string]interface{}); ok {
		result["spec"] = normalizeSpec(spec)
	}

	return result
}

// normalizeSpec recursively normalizes a spec, removing empty values.
func normalizeSpec(spec map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range spec {
		switch val := v.(type) {
		case map[string]interface{}:
			if len(val) > 0 {
				normalized := normalizeSpec(val)
				if len(normalized) > 0 {
					result[k] = normalized
				}
			}
		case []interface{}:
			if len(val) > 0 {
				result[k] = val
			}
		case string:
			if val != "" {
				result[k] = val
			}
		case nil:
			// Skip nil values
		default:
			result[k] = v
		}
	}

	return result
}

// generateUnifiedDiff generates a unified diff between two strings.
func generateUnifiedDiff(oldName, newName, oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var result strings.Builder

	result.WriteString(fmt.Sprintf("--- %s\n", oldName))
	result.WriteString(fmt.Sprintf("+++ %s\n", newName))

	// Simple line-by-line diff
	// For more sophisticated diff, use a proper diff library
	diff := simpleDiff(oldLines, newLines)

	currentHunk := []string{}
	hunkStart := -1
	contextLines := 3

	for i, line := range diff {
		if line.Type != "=" {
			if hunkStart == -1 {
				hunkStart = max(0, i-contextLines)
			}
			currentHunk = append(currentHunk, formatDiffLine(line))
		} else if hunkStart != -1 {
			// Add context after changes
			if len(currentHunk) > 0 {
				currentHunk = append(currentHunk, formatDiffLine(line))
				if i-hunkStart > len(currentHunk)+contextLines {
					// End hunk
					result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunkStart+1, len(currentHunk), hunkStart+1, len(currentHunk)))
					for _, l := range currentHunk {
						result.WriteString(l)
						result.WriteString("\n")
					}
					currentHunk = []string{}
					hunkStart = -1
				}
			}
		}
	}

	// Output remaining hunk
	if len(currentHunk) > 0 {
		result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunkStart+1, len(currentHunk), hunkStart+1, len(currentHunk)))
		for _, l := range currentHunk {
			result.WriteString(l)
			result.WriteString("\n")
		}
	}

	return result.String()
}

type diffLine struct {
	Type    string // "=", "+", "-"
	Content string
}

// simpleDiff performs a simple line-by-line diff.
func simpleDiff(oldLines, newLines []string) []diffLine {
	var result []diffLine

	// Build a map of old lines for quick lookup
	oldMap := make(map[string]bool)
	for _, line := range oldLines {
		oldMap[line] = true
	}

	newMap := make(map[string]bool)
	for _, line := range newLines {
		newMap[line] = true
	}

	oldIdx := 0
	newIdx := 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		switch {
		case oldIdx >= len(oldLines):
			// Only new lines left
			result = append(result, diffLine{Type: "+", Content: newLines[newIdx]})
			newIdx++
		case newIdx >= len(newLines):
			// Only old lines left
			result = append(result, diffLine{Type: "-", Content: oldLines[oldIdx]})
			oldIdx++
		case oldLines[oldIdx] == newLines[newIdx]:
			// Lines match
			result = append(result, diffLine{Type: "=", Content: oldLines[oldIdx]})
			oldIdx++
			newIdx++
		case !newMap[oldLines[oldIdx]]:
			// Old line was removed
			result = append(result, diffLine{Type: "-", Content: oldLines[oldIdx]})
			oldIdx++
		case !oldMap[newLines[newIdx]]:
			// New line was added
			result = append(result, diffLine{Type: "+", Content: newLines[newIdx]})
			newIdx++
		default:
			// Both exist but in different positions - treat as change
			result = append(result,
				diffLine{Type: "-", Content: oldLines[oldIdx]},
				diffLine{Type: "+", Content: newLines[newIdx]},
			)
			oldIdx++
			newIdx++
		}
	}

	return result
}

func formatDiffLine(line diffLine) string {
	switch line.Type {
	case "+":
		if diffNoColor {
			return "+" + line.Content
		}
		return "\033[32m+" + line.Content + "\033[0m"
	case "-":
		if diffNoColor {
			return "-" + line.Content
		}
		return "\033[31m-" + line.Content + "\033[0m"
	default:
		return " " + line.Content
	}
}

func printDiffLine(prefix, content string) {
	switch prefix {
	case "+":
		if diffNoColor {
			fmt.Printf("+%s\n", content)
		} else {
			fmt.Printf("\033[32m+%s\033[0m\n", content)
		}
	case "-":
		if diffNoColor {
			fmt.Printf("-%s\n", content)
		} else {
			fmt.Printf("\033[31m-%s\033[0m\n", content)
		}
	default:
		fmt.Printf(" %s\n", content)
	}
}
