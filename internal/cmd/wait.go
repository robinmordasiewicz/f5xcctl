package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/output"
	"github.com/f5/f5xcctl/internal/runtime"
)

var (
	waitFor     string
	waitTimeout time.Duration
)

var waitCmd = &cobra.Command{
	Use:   "wait <resource-type> <name> [flags]",
	Short: "Wait for a specific condition on a resource",
	Long: `Wait for a specific condition on one or more resources.

The wait command blocks until the specified condition is met or the timeout is reached.

Conditions:
  --for=delete              Wait for the resource to be deleted
  --for=condition=<name>    Wait for a specific condition to be True
  --for=jsonpath=<expr>     Wait for a JSONPath expression to evaluate to a value

Examples:
  # Wait for a load balancer to be deleted
  f5xcctl wait httplb my-lb --for=delete -n production

  # Wait for a resource to be deleted with timeout
  f5xcctl wait httplb my-lb --for=delete --timeout=5m -n production

  # Wait for a condition
  f5xcctl wait httplb my-lb --for=condition=Ready -n production

  # Wait using JSONPath
  f5xcctl wait httplb my-lb --for=jsonpath='{.status.state}'=active -n production`,
	Args: cobra.ExactArgs(2),
	RunE: runWait,
}

func init() {
	waitCmd.Flags().StringVar(&waitFor, "for", "", "The condition to wait for (delete, condition=<name>, jsonpath=<expr>=<value>)")
	waitCmd.Flags().DurationVar(&waitTimeout, "timeout", 30*time.Second, "The timeout for the wait operation")
	waitCmd.MarkFlagRequired("for")

	rootCmd.AddCommand(waitCmd)
}

func runWait(cmd *cobra.Command, args []string) error {
	resourceType := args[0]
	resourceName := args[1]

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s\n\nUse 'f5xcctl api-resources' to list available resource types", resourceType)
	}

	ns := namespace
	if rt.Namespaced && ns == "" {
		ns = "default"
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	// Parse the --for flag
	switch {
	case waitFor == "delete":
		return waitForDeletion(client, rt, ns, resourceName, waitTimeout)
	case strings.HasPrefix(waitFor, "condition="):
		condition := strings.TrimPrefix(waitFor, "condition=")
		return waitForCondition(client, rt, ns, resourceName, condition, waitTimeout)
	case strings.HasPrefix(waitFor, "jsonpath="):
		expr := strings.TrimPrefix(waitFor, "jsonpath=")
		return waitForJSONPath(client, rt, ns, resourceName, expr, waitTimeout)
	default:
		return fmt.Errorf("invalid --for value: %s\n\nValid values:\n  delete\n  condition=<name>\n  jsonpath=<expr>=<value>", waitFor)
	}
}

// waitForDeletion waits until the resource no longer exists.
func waitForDeletion(client *runtime.Client, rt *ResourceType, ns, name string, timeout time.Duration) error {
	output.Info("Waiting for %s/%s to be deleted...", rt.Name, name)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s/%s to be deleted", rt.Name, name)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		path := rt.GetItemPath(ns, name)
		resp, err := client.Get(ctx, path, nil)
		cancel()

		// If we get an error or the resource doesn't exist, it's deleted
		if err != nil || !resp.IsSuccess() {
			output.Success("%s/%s deleted", rt.Name, name)
			return nil
		}

		<-ticker.C
	}
}

// waitForCondition waits for a specific condition to be True.
func waitForCondition(client *runtime.Client, rt *ResourceType, ns, name, condition string, timeout time.Duration) error {
	output.Info("Waiting for %s/%s condition %q...", rt.Name, name, condition)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s/%s condition %q", rt.Name, name, condition)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		path := rt.GetItemPath(ns, name)
		resp, err := client.Get(ctx, path, nil)
		cancel()

		if err != nil {
			<-ticker.C
			continue
		}

		if !resp.IsSuccess() {
			<-ticker.C
			continue
		}

		var result map[string]interface{}
		if err := resp.DecodeJSON(&result); err != nil {
			<-ticker.C
			continue
		}

		// Check for condition in status.conditions array
		if status, ok := result["status"].(map[string]interface{}); ok {
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					if cond, ok := c.(map[string]interface{}); ok {
						if cond["type"] == condition && cond["status"] == "True" {
							output.Success("%s/%s condition %q is True", rt.Name, name, condition)
							return nil
						}
					}
				}
			}
			// Also check direct status fields (F5XC specific)
			if state, ok := status["state"].(string); ok {
				if strings.EqualFold(state, condition) || strings.EqualFold(state, "active") && strings.EqualFold(condition, "ready") {
					output.Success("%s/%s is %s", rt.Name, name, state)
					return nil
				}
			}
		}

		// Check system_metadata for F5XC specific conditions
		if sysMeta, ok := result["system_metadata"].(map[string]interface{}); ok {
			if state, ok := sysMeta["state"].(string); ok {
				if strings.EqualFold(state, condition) || (strings.EqualFold(state, "active") && strings.EqualFold(condition, "ready")) {
					output.Success("%s/%s is %s", rt.Name, name, state)
					return nil
				}
			}
		}

		<-ticker.C
	}
}

// waitForJSONPath waits for a JSONPath expression to equal a specific value.
func waitForJSONPath(client *runtime.Client, rt *ResourceType, ns, name, expr string, timeout time.Duration) error {
	// Parse expr: {.status.state}=active
	parts := strings.SplitN(expr, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid jsonpath expression: %s\n\nExpected format: jsonpath='{.path.to.field}'=value", expr)
	}

	jsonPath := strings.Trim(parts[0], "'{}")
	expectedValue := parts[1]

	output.Info("Waiting for %s/%s %s=%s...", rt.Name, name, jsonPath, expectedValue)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s/%s %s=%s", rt.Name, name, jsonPath, expectedValue)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		path := rt.GetItemPath(ns, name)
		resp, err := client.Get(ctx, path, nil)
		cancel()

		if err != nil {
			<-ticker.C
			continue
		}

		if !resp.IsSuccess() {
			<-ticker.C
			continue
		}

		var result map[string]interface{}
		if err := resp.DecodeJSON(&result); err != nil {
			<-ticker.C
			continue
		}

		// Simple JSONPath evaluation
		value := evaluateSimpleJSONPath(result, jsonPath)
		if fmt.Sprintf("%v", value) == expectedValue {
			output.Success("%s/%s %s=%s", rt.Name, name, jsonPath, expectedValue)
			return nil
		}

		<-ticker.C
	}
}

// evaluateSimpleJSONPath evaluates a simple JSONPath expression
// Supports paths like ".status.state" or ".metadata.name".
func evaluateSimpleJSONPath(data map[string]interface{}, path string) interface{} {
	// Remove leading dot if present
	path = strings.TrimPrefix(path, ".")

	parts := strings.Split(path, ".")
	var current interface{} = data

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
