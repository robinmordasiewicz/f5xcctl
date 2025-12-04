package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/f5/f5xcctl/internal/output"
)

// Patch command flags.
var (
	patchData     string
	patchType     string
	patchFromFile string
)

// patchCmd represents the patch command.
var patchCmd = &cobra.Command{
	Use:   "patch <resource-type> <name> [flags]",
	Short: "Update fields of a resource",
	Long: `Update fields of a resource using a strategic merge patch, JSON merge patch, or JSON patch.

Patch types:
  merge       - JSON merge patch (default). Merges the patch into the existing resource.
  strategic   - Strategic merge patch. Similar to merge but with special handling for arrays.
  json        - JSON patch (RFC 6902). Array of add/remove/replace operations.

Examples:
  # Patch a load balancer using merge patch (default)
  f5xcctl patch httplb my-lb -p '{"spec":{"domains":["example.com","api.example.com"]}}'

  # Patch using a file
  f5xcctl patch httplb my-lb --patch-file patch.yaml

  # Use JSON patch to add a value
  f5xcctl patch httplb my-lb --type json -p '[{"op":"add","path":"/spec/domains/-","value":"new.example.com"}]'

  # Patch from stdin
  echo '{"metadata":{"labels":{"env":"prod"}}}' | f5xcctl patch httplb my-lb -p -

  # Dry run - show what would be patched
  f5xcctl patch httplb my-lb -p '{"spec":{"port":8080}}' --dry-run`,
	Args:              cobra.MinimumNArgs(2),
	RunE:              runPatch,
	ValidArgsFunction: completeResourceTypes,
}

func init() {
	patchCmd.Flags().StringVarP(&patchData, "patch", "p", "", "The patch to be applied (JSON or YAML)")
	patchCmd.Flags().StringVar(&patchType, "type", "merge", "Patch type: merge, strategic, or json")
	patchCmd.Flags().StringVar(&patchFromFile, "patch-file", "", "File containing the patch")
	patchCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only print what would be patched")

	rootCmd.AddCommand(patchCmd)
}

func runPatch(cmd *cobra.Command, args []string) error {
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

	// Get patch data
	patchContent, err := getPatchContent()
	if err != nil {
		return err
	}

	// Parse patch data
	var patch map[string]interface{}
	if patchType == "json" {
		// JSON patch is an array, handle differently
		var jsonPatch []interface{}
		if err := json.Unmarshal([]byte(patchContent), &jsonPatch); err != nil {
			// Try YAML
			if err := yaml.Unmarshal([]byte(patchContent), &jsonPatch); err != nil {
				return fmt.Errorf("failed to parse JSON patch: %w", err)
			}
		}
		// For JSON patch, we need to apply it differently
		return applyJSONPatch(rt, ns, resourceName, jsonPatch)
	}

	// Parse merge patch (JSON or YAML)
	if err := yaml.Unmarshal([]byte(patchContent), &patch); err != nil {
		return fmt.Errorf("failed to parse patch: %w", err)
	}

	if dryRun {
		fmt.Printf("Would patch %s/%s in namespace %q with:\n", rt.Name, resourceName, ns)
		return output.Print("yaml", patch)
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

	var current map[string]interface{}
	if err := resp.DecodeJSON(&current); err != nil {
		return fmt.Errorf("failed to decode current resource: %w", err)
	}

	// Apply merge patch
	merged := deepMerge(current, patch)

	// Update the resource
	updateResp, err := client.Put(ctx, path, merged)
	if err != nil {
		return fmt.Errorf("failed to patch %s %q: %w", rt.Name, resourceName, err)
	}
	if err := updateResp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s patched", rt.Name, resourceName)
	return nil
}

// getPatchContent retrieves patch content from various sources.
func getPatchContent() (string, error) {
	// Check for patch file first
	if patchFromFile != "" {
		data, err := os.ReadFile(patchFromFile)
		if err != nil {
			return "", fmt.Errorf("failed to read patch file: %w", err)
		}
		return string(data), nil
	}

	// Check for patch data
	if patchData == "" {
		return "", fmt.Errorf("patch data required: use -p/--patch or --patch-file")
	}

	// Check for stdin
	if patchData == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read patch from stdin: %w", err)
		}
		return string(data), nil
	}

	return patchData, nil
}

// deepMerge performs a deep merge of src into dst.
// Values in src override values in dst.
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy dst to result
	for k, v := range dst {
		result[k] = v
	}

	// Merge src into result
	for k, v := range src {
		if v == nil {
			// Explicit null means delete
			delete(result, k)
			continue
		}

		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := result[k].(map[string]interface{}); ok {
				// Both are maps, recursively merge
				result[k] = deepMerge(dstMap, srcMap)
			} else {
				// dst is not a map, replace with src
				result[k] = srcMap
			}
		} else {
			// Not a map, replace
			result[k] = v
		}
	}

	return result
}

// applyJSONPatch applies a JSON patch (RFC 6902) to a resource.
func applyJSONPatch(rt *ResourceType, ns, resourceName string, patch []interface{}) error {
	if dryRun {
		fmt.Printf("Would apply JSON patch to %s/%s:\n", rt.Name, resourceName)
		return output.Print("yaml", patch)
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

	var current map[string]interface{}
	if err := resp.DecodeJSON(&current); err != nil {
		return fmt.Errorf("failed to decode current resource: %w", err)
	}

	// Apply each operation in the patch
	for _, op := range patch {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid JSON patch operation: expected object")
		}

		operation, _ := opMap["op"].(string)
		opPath, _ := opMap["path"].(string)
		value := opMap["value"]

		switch operation {
		case "add":
			current, err = jsonPatchAdd(current, opPath, value)
		case "remove":
			current, err = jsonPatchRemove(current, opPath)
		case "replace":
			current, err = jsonPatchReplace(current, opPath, value)
		case "copy", "move", "test":
			return fmt.Errorf("JSON patch operation %q not yet implemented", operation)
		default:
			return fmt.Errorf("unknown JSON patch operation: %s", operation)
		}

		if err != nil {
			return fmt.Errorf("failed to apply operation %s: %w", operation, err)
		}
	}

	// Update the resource
	updateResp, err := client.Put(ctx, path, current)
	if err != nil {
		return fmt.Errorf("failed to patch %s %q: %w", rt.Name, resourceName, err)
	}
	if err := updateResp.Error(); err != nil {
		return err
	}

	output.Successf("%s/%s patched (json)", rt.Name, resourceName)
	return nil
}

// jsonPatchAdd adds a value at the specified path.
func jsonPatchAdd(doc map[string]interface{}, path string, value interface{}) (map[string]interface{}, error) {
	if path == "" || path == "/" {
		return doc, fmt.Errorf("cannot add to root path")
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return setPath(doc, parts, value, true)
}

// jsonPatchRemove removes a value at the specified path.
func jsonPatchRemove(doc map[string]interface{}, path string) (map[string]interface{}, error) {
	if path == "" || path == "/" {
		return doc, fmt.Errorf("cannot remove root path")
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return removePath(doc, parts)
}

// jsonPatchReplace replaces a value at the specified path.
func jsonPatchReplace(doc map[string]interface{}, path string, value interface{}) (map[string]interface{}, error) {
	if path == "" || path == "/" {
		return doc, fmt.Errorf("cannot replace root path")
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return setPath(doc, parts, value, false)
}

// setPath sets a value at the specified path in a nested map.
func setPath(doc map[string]interface{}, parts []string, value interface{}, allowAdd bool) (map[string]interface{}, error) {
	if len(parts) == 0 {
		return doc, nil
	}

	result := make(map[string]interface{})
	for k, v := range doc {
		result[k] = v
	}

	key := parts[0]

	if len(parts) == 1 {
		// Handle array append with "-"
		if key == "-" {
			return result, fmt.Errorf("array append (-) only valid for arrays, not objects")
		}

		if !allowAdd {
			// For replace, check that key exists
			if _, exists := result[key]; !exists {
				return result, fmt.Errorf("path %q does not exist", key)
			}
		}
		result[key] = value
		return result, nil
	}

	// Navigate deeper
	next, ok := result[key].(map[string]interface{})
	if !ok {
		// Handle array navigation
		if arr, ok := result[key].([]interface{}); ok {
			idx := parseArrayIndex(parts[1], len(arr))
			if idx < 0 || idx >= len(arr) {
				return result, fmt.Errorf("array index out of bounds: %s", parts[1])
			}
			if nestedMap, ok := arr[idx].(map[string]interface{}); ok {
				updated, err := setPath(nestedMap, parts[2:], value, allowAdd)
				if err != nil {
					return result, err
				}
				newArr := make([]interface{}, len(arr))
				copy(newArr, arr)
				newArr[idx] = updated
				result[key] = newArr
				return result, nil
			}
			// Direct array element update
			if len(parts) == 2 {
				newArr := make([]interface{}, len(arr))
				copy(newArr, arr)
				newArr[idx] = value
				result[key] = newArr
				return result, nil
			}
			return result, fmt.Errorf("cannot navigate through non-object array element")
		}

		if allowAdd {
			next = make(map[string]interface{})
			result[key] = next
		} else {
			return result, fmt.Errorf("path %q does not exist", key)
		}
	}

	updated, err := setPath(next, parts[1:], value, allowAdd)
	if err != nil {
		return result, err
	}
	result[key] = updated
	return result, nil
}

// removePath removes a value at the specified path.
func removePath(doc map[string]interface{}, parts []string) (map[string]interface{}, error) {
	if len(parts) == 0 {
		return doc, nil
	}

	result := make(map[string]interface{})
	for k, v := range doc {
		result[k] = v
	}

	key := parts[0]

	if len(parts) == 1 {
		if _, exists := result[key]; !exists {
			return result, fmt.Errorf("path %q does not exist", key)
		}
		delete(result, key)
		return result, nil
	}

	next, ok := result[key].(map[string]interface{})
	if !ok {
		// Handle array navigation
		if arr, ok := result[key].([]interface{}); ok {
			idx := parseArrayIndex(parts[1], len(arr))
			if idx < 0 || idx >= len(arr) {
				return result, fmt.Errorf("array index out of bounds: %s", parts[1])
			}
			if len(parts) == 2 {
				// Remove array element
				newArr := make([]interface{}, 0, len(arr)-1)
				newArr = append(newArr, arr[:idx]...)
				newArr = append(newArr, arr[idx+1:]...)
				result[key] = newArr
				return result, nil
			}
			// Navigate deeper
			if nestedMap, ok := arr[idx].(map[string]interface{}); ok {
				updated, err := removePath(nestedMap, parts[2:])
				if err != nil {
					return result, err
				}
				newArr := make([]interface{}, len(arr))
				copy(newArr, arr)
				newArr[idx] = updated
				result[key] = newArr
				return result, nil
			}
			return result, fmt.Errorf("cannot navigate through non-object array element")
		}
		return result, fmt.Errorf("path %q does not exist or is not an object", key)
	}

	updated, err := removePath(next, parts[1:])
	if err != nil {
		return result, err
	}
	result[key] = updated
	return result, nil
}

// parseArrayIndex parses an array index from a path segment.
func parseArrayIndex(segment string, arrLen int) int {
	if segment == "-" {
		return arrLen // Append position
	}
	var idx int
	if _, err := fmt.Sscanf(segment, "%d", &idx); err != nil {
		return -1
	}
	return idx
}
