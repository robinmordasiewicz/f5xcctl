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

// ResourceMetadata represents common metadata for resources.
type ResourceMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Description string            `json:"description,omitempty"`
}

// HTTPLoadBalancer represents an HTTP load balancer resource.
type HTTPLoadBalancer struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
	Status     map[string]interface{} `json:"status,omitempty"`
}

// HTTPLoadBalancerList represents a list response.
type HTTPLoadBalancerList struct {
	Items []HTTPLoadBalancer `json:"items"`
}

// HTTPLoadBalancerTableRow for table display.
type HTTPLoadBalancerTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Domains   string `json:"domains"`
	State     string `json:"state"`
}

var (
	lbLabelFilter string
	lbSpecFile    string
	lbWait        bool
	lbForce       bool
)

func newLBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lb",
		Aliases: []string{"loadbalancer"},
		Short:   "Manage load balancers",
		Long: `Manage F5 Distributed Cloud load balancers.

Load balancers distribute traffic across origin pools and provide
features like WAF, bot defense, and API protection.

Subcommands:
  http    HTTP/HTTPS load balancers
  tcp     TCP load balancers (coming soon)
  udp     UDP load balancers (coming soon)`,
	}

	cmd.AddCommand(newHTTPLBCmd())

	return cmd
}

func newHTTPLBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http",
		Short: "Manage HTTP load balancers",
		Long: `Manage HTTP/HTTPS load balancers in F5 Distributed Cloud.

HTTP load balancers provide application delivery with features including:
  - SSL/TLS termination
  - Web Application Firewall (WAF)
  - Bot defense
  - API discovery and protection
  - Rate limiting
  - Custom routes and rewrites`,
	}

	cmd.AddCommand(newHTTPLBListCmd())
	cmd.AddCommand(newHTTPLBGetCmd())
	cmd.AddCommand(newHTTPLBCreateCmd())
	cmd.AddCommand(newHTTPLBUpdateCmd())
	cmd.AddCommand(newHTTPLBDeleteCmd())

	return cmd
}

func newHTTPLBListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List HTTP load balancers",
		Long:  `List all HTTP load balancers in the specified namespace.`,
		Example: `  # List all HTTP load balancers in default namespace
  f5xcctl lb http list

  # List in a specific namespace
  f5xcctl lb http list -n production

  # Output as JSON
  f5xcctl lb http list -o json

  # Filter by label
  f5xcctl lb http list --label-filter "env=prod"`,
		RunE: runHTTPLBList,
	}

	cmd.Flags().StringVar(&lbLabelFilter, "label-filter", "", "filter by label selector (e.g., 'env=prod,team=platform')")

	return cmd
}

func runHTTPLBList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	// Build query parameters
	query := url.Values{}
	if lbLabelFilter != "" {
		query.Set("label_filter", lbLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/http_loadbalancers", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list HTTP load balancers: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list HTTPLoadBalancerList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Format output
	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	// Table output
	if len(list.Items) == 0 {
		output.Infof("No HTTP load balancers found in namespace %q", ns)
		return nil
	}

	tableData := make([]HTTPLoadBalancerTableRow, 0, len(list.Items))
	for _, lb := range list.Items {
		domains := "-"
		if spec := lb.Spec; spec != nil {
			if d, ok := spec["domains"].([]interface{}); ok && len(d) > 0 {
				if len(d) == 1 {
					domains = fmt.Sprintf("%v", d[0])
				} else {
					domains = fmt.Sprintf("%v (+%d more)", d[0], len(d)-1)
				}
			}
		}
		state := "UNKNOWN"
		if status := lb.Status; status != nil {
			if s, ok := status["state"].(string); ok {
				state = s
			}
		}
		tableData = append(tableData, HTTPLoadBalancerTableRow{
			Name:      lb.Metadata.Name,
			Namespace: lb.Metadata.Namespace,
			Domains:   domains,
			State:     state,
		})
	}

	return output.Print("table", tableData)
}

func newHTTPLBGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get HTTP load balancer details",
		Long:  `Get detailed information about a specific HTTP load balancer.`,
		Example: `  # Get load balancer details
  f5xcctl lb http get my-lb

  # Output as YAML
  f5xcctl lb http get my-lb -o yaml

  # Get from specific namespace
  f5xcctl lb http get my-lb -n production`,
		Args: cobra.ExactArgs(1),
		RunE: runHTTPLBGet,
	}

	return cmd
}

func runHTTPLBGet(cmd *cobra.Command, args []string) error {
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

	path := fmt.Sprintf("/api/config/namespaces/%s/http_loadbalancers/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get HTTP load balancer: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var lb HTTPLoadBalancer
	if err := resp.DecodeJSON(&lb); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Format output
	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, lb)
	}

	// Detailed output
	fmt.Printf("Name:        %s\n", lb.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", lb.Metadata.Namespace)
	if lb.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", lb.Metadata.Description)
	}

	// Show domains
	if spec := lb.Spec; spec != nil {
		if domains, ok := spec["domains"].([]interface{}); ok && len(domains) > 0 {
			fmt.Println("Domains:")
			for _, d := range domains {
				fmt.Printf("  - %v\n", d)
			}
		}
	}

	// Show status
	if status := lb.Status; status != nil {
		if state, ok := status["state"].(string); ok {
			fmt.Printf("State:       %s\n", state)
		}
	}

	// Show labels
	if len(lb.Metadata.Labels) > 0 {
		fmt.Println("Labels:")
		for k, v := range lb.Metadata.Labels {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}

func newHTTPLBCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an HTTP load balancer",
		Long: `Create a new HTTP load balancer from a specification file.

The specification file should be in JSON or YAML format containing
the load balancer configuration including domains, routes, and origin pools.`,
		Example: `  # Create from spec file
  f5xcctl lb http create my-lb --spec-file lb.yaml

  # Create and wait for ready state
  f5xcctl lb http create my-lb --spec-file lb.yaml --wait

  # Create in specific namespace
  f5xcctl lb http create my-lb --spec-file lb.yaml -n production`,
		Args: cobra.ExactArgs(1),
		RunE: runHTTPLBCreate,
	}

	cmd.Flags().StringVarP(&lbSpecFile, "spec-file", "f", "", "path to specification file (required)")
	cmd.Flags().BoolVar(&lbWait, "wait", false, "wait for load balancer to be ready")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runHTTPLBCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	// Read and parse spec file
	spec, err := readSpecFile(lbSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": spec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/http_loadbalancers", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create HTTP load balancer: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var lb HTTPLoadBalancer
	if err := resp.DecodeJSON(&lb); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, lb)
	}

	output.Successf("HTTP load balancer %q created in namespace %q", name, ns)

	if lbWait {
		output.Infof("Note: --wait not yet implemented")
	}

	return nil
}

func newHTTPLBUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an HTTP load balancer",
		Long: `Update an existing HTTP load balancer with a new specification.

The specification file should be in JSON or YAML format containing
the complete load balancer configuration.`,
		Example: `  # Update from spec file
  f5xcctl lb http update my-lb --spec-file lb.yaml

  # Update in specific namespace
  f5xcctl lb http update my-lb --spec-file lb.yaml -n production`,
		Args: cobra.ExactArgs(1),
		RunE: runHTTPLBUpdate,
	}

	cmd.Flags().StringVarP(&lbSpecFile, "spec-file", "f", "", "path to specification file (required)")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runHTTPLBUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	// Read and parse spec file
	spec, err := readSpecFile(lbSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	// Build request body
	reqBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": spec,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/http_loadbalancers/%s", ns, name)
	resp, err := client.Put(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update HTTP load balancer: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var lb HTTPLoadBalancer
	if err := resp.DecodeJSON(&lb); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, lb)
	}

	output.Successf("HTTP load balancer %q updated in namespace %q", name, ns)
	return nil
}

func newHTTPLBDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an HTTP load balancer",
		Long:  `Delete an HTTP load balancer from the specified namespace.`,
		Example: `  # Delete a load balancer (will prompt for confirmation)
  f5xcctl lb http delete my-lb

  # Force delete without confirmation
  f5xcctl lb http delete my-lb --yes

  # Delete from specific namespace
  f5xcctl lb http delete my-lb -n production`,
		Args: cobra.ExactArgs(1),
		RunE: runHTTPLBDelete,
	}

	cmd.Flags().BoolVarP(&lbForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runHTTPLBDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	// Confirmation prompt
	if !lbForce {
		fmt.Printf("Are you sure you want to delete HTTP load balancer %q in namespace %q? [y/N]: ", name, ns)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/http_loadbalancers/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete HTTP load balancer: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("HTTP load balancer %q deleted from namespace %q", name, ns)
	return nil
}

// readSpecFile reads and parses a JSON or YAML spec file.
func readSpecFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var spec map[string]interface{}

	// Try YAML first (which also handles JSON)
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec file: %w", err)
	}

	return spec, nil
}
