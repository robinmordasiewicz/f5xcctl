package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/f5/f5xcctl/internal/output"
)

// OriginPool represents an origin pool resource.
type OriginPool struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
	Status     map[string]interface{} `json:"status,omitempty"`
}

// OriginPoolList represents a list response.
type OriginPoolList struct {
	Items []OriginPool `json:"items"`
}

// OriginPoolTableRow for table display.
type OriginPoolTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Origins   string `json:"origins"`
	LBAlgo    string `json:"lb_algorithm"`
	HealthChk string `json:"health_check"`
}

var (
	opLabelFilter string
	opSpecFile    string
	opForce       bool
)

func newOriginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "origin",
		Short: "Manage origin resources",
		Long: `Manage F5 Distributed Cloud origin resources.

Origins define the backend servers that receive traffic from load balancers.

Subcommands:
  pool    Origin pools (groups of backend servers)`,
	}

	cmd.AddCommand(newOriginPoolCmd())

	return cmd
}

func newOriginPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pool",
		Aliases: []string{"pools"},
		Short:   "Manage origin pools",
		Long: `Manage origin pools in F5 Distributed Cloud.

Origin pools define groups of backend servers that receive traffic from
load balancers. Features include:
  - Multiple origin server types (IP, DNS, K8s service)
  - Health checking
  - Load balancing algorithms
  - TLS configuration for backend connections`,
	}

	cmd.AddCommand(newOriginPoolListCmd())
	cmd.AddCommand(newOriginPoolGetCmd())
	cmd.AddCommand(newOriginPoolCreateCmd())
	cmd.AddCommand(newOriginPoolUpdateCmd())
	cmd.AddCommand(newOriginPoolDeleteCmd())

	return cmd
}

func newOriginPoolListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List origin pools",
		Long:  `List all origin pools in the specified namespace.`,
		Example: `  # List all origin pools in default namespace
  f5xcctl origin pool list

  # List in a specific namespace
  f5xcctl origin pool list -n production

  # Output as JSON
  f5xcctl origin pool list -o json`,
		RunE: runOriginPoolList,
	}

	cmd.Flags().StringVar(&opLabelFilter, "label-filter", "", "filter by label selector")

	return cmd
}

func runOriginPoolList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	query := url.Values{}
	if opLabelFilter != "" {
		query.Set("label_filter", opLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/origin_pools", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list origin pools: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list OriginPoolList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	if len(list.Items) == 0 {
		output.Infof("No origin pools found in namespace %q", ns)
		return nil
	}

	tableData := make([]OriginPoolTableRow, 0, len(list.Items))
	for _, op := range list.Items {
		origins := "-"
		lbAlgo := "-"
		healthChk := "disabled"

		if spec := op.Spec; spec != nil {
			// Count origins
			if servers, ok := spec["origin_servers"].([]interface{}); ok {
				origins = fmt.Sprintf("%d servers", len(servers))
			}
			// Get LB algorithm
			if algo, ok := spec["loadbalancer_algorithm"].(string); ok {
				lbAlgo = algo
			}
			// Check health check
			if _, ok := spec["healthcheck"]; ok {
				healthChk = "enabled"
			}
		}

		tableData = append(tableData, OriginPoolTableRow{
			Name:      op.Metadata.Name,
			Namespace: op.Metadata.Namespace,
			Origins:   origins,
			LBAlgo:    lbAlgo,
			HealthChk: healthChk,
		})
	}

	return output.Print("table", tableData)
}

func newOriginPoolGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get origin pool details",
		Long:  `Get detailed information about a specific origin pool.`,
		Example: `  # Get origin pool details
  f5xcctl origin pool get my-pool

  # Output as YAML
  f5xcctl origin pool get my-pool -o yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runOriginPoolGet,
	}

	return cmd
}

func runOriginPoolGet(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/origin_pools/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get origin pool: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var op OriginPool
	if err := resp.DecodeJSON(&op); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, op)
	}

	// Detailed output
	fmt.Printf("Name:        %s\n", op.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", op.Metadata.Namespace)
	if op.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", op.Metadata.Description)
	}

	if spec := op.Spec; spec != nil {
		if servers, ok := spec["origin_servers"].([]interface{}); ok {
			fmt.Printf("Origins:     %d servers\n", len(servers))
		}
		if algo, ok := spec["loadbalancer_algorithm"].(string); ok {
			fmt.Printf("LB Algorithm: %s\n", algo)
		}
	}

	if len(op.Metadata.Labels) > 0 {
		fmt.Println("Labels:")
		for k, v := range op.Metadata.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}

func newOriginPoolCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an origin pool",
		Long: `Create a new origin pool from a specification file.

The specification file should be in JSON or YAML format containing
the origin pool configuration including origin servers and health checks.`,
		Example: `  # Create from spec file
  f5xcctl origin pool create my-pool --spec-file pool.yaml

  # Create in specific namespace
  f5xcctl origin pool create my-pool --spec-file pool.yaml -n production`,
		Args: cobra.ExactArgs(1),
		RunE: runOriginPoolCreate,
	}

	cmd.Flags().StringVarP(&opSpecFile, "spec-file", "f", "", "path to specification file (required)")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runOriginPoolCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	spec, err := readOriginPoolSpecFile(opSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	reqBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": spec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/origin_pools", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create origin pool: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var op OriginPool
	if err := resp.DecodeJSON(&op); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, op)
	}

	output.Successf("Origin pool %q created in namespace %q", name, ns)
	return nil
}

func newOriginPoolUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an origin pool",
		Long:  `Update an existing origin pool with a new specification.`,
		Example: `  # Update from spec file
  f5xcctl origin pool update my-pool --spec-file pool.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runOriginPoolUpdate,
	}

	cmd.Flags().StringVarP(&opSpecFile, "spec-file", "f", "", "path to specification file (required)")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runOriginPoolUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	spec, err := readOriginPoolSpecFile(opSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	reqBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": spec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/origin_pools/%s", ns, name)
	resp, err := client.Put(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update origin pool: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var op OriginPool
	if err := resp.DecodeJSON(&op); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, op)
	}

	output.Successf("Origin pool %q updated in namespace %q", name, ns)
	return nil
}

func newOriginPoolDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an origin pool",
		Long:  `Delete an origin pool from the specified namespace.`,
		Example: `  # Delete an origin pool
  f5xcctl origin pool delete my-pool

  # Force delete without confirmation
  f5xcctl origin pool delete my-pool --yes`,
		Args: cobra.ExactArgs(1),
		RunE: runOriginPoolDelete,
	}

	cmd.Flags().BoolVarP(&opForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runOriginPoolDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if !opForce {
		fmt.Printf("Are you sure you want to delete origin pool %q in namespace %q? [y/N]: ", name, ns)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/origin_pools/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete origin pool: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("Origin pool %q deleted from namespace %q", name, ns)
	return nil
}

func readOriginPoolSpecFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec file: %w", err)
	}

	return spec, nil
}
