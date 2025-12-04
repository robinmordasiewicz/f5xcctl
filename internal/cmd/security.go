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

// AppFirewall represents an app firewall resource.
type AppFirewall struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
}

// AppFirewallList represents a list response.
type AppFirewallList struct {
	Items []AppFirewall `json:"items"`
}

// AppFirewallTableRow for table display.
type AppFirewallTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Mode      string `json:"mode"`
	RuleSets  string `json:"rule_sets"`
}

var (
	afLabelFilter string
	afSpecFile    string
	afForce       bool
)

func newSecurityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "security",
		Aliases: []string{"sec"},
		Short:   "Manage security resources",
		Long: `Manage F5 Distributed Cloud security resources.

Security resources protect your applications from threats.

Subcommands:
  app-firewall    Web Application Firewall (WAF)
  waf            Alias for app-firewall
  service-policy  Service policies (coming soon)
  bot-defense     Bot defense (coming soon)`,
	}

	cmd.AddCommand(newAppFirewallCmd())

	return cmd
}

func newAppFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "app-firewall",
		Aliases: []string{"waf", "appfw"},
		Short:   "Manage application firewalls (WAF)",
		Long: `Manage Web Application Firewalls in F5 Distributed Cloud.

Application Firewalls protect web applications with features including:
  - OWASP Top 10 protection
  - Custom WAF rules
  - Bot detection
  - API protection
  - Attack signatures`,
	}

	cmd.AddCommand(newAppFirewallListCmd())
	cmd.AddCommand(newAppFirewallGetCmd())
	cmd.AddCommand(newAppFirewallCreateCmd())
	cmd.AddCommand(newAppFirewallUpdateCmd())
	cmd.AddCommand(newAppFirewallDeleteCmd())

	return cmd
}

func newAppFirewallListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List application firewalls",
		Long:  `List all application firewalls in the specified namespace.`,
		Example: `  # List all app firewalls
  f5xcctl security app-firewall list

  # List in a specific namespace
  f5xcctl security app-firewall list -n production`,
		RunE: runAppFirewallList,
	}

	cmd.Flags().StringVar(&afLabelFilter, "label-filter", "", "filter by label selector")

	return cmd
}

func runAppFirewallList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	query := url.Values{}
	if afLabelFilter != "" {
		query.Set("label_filter", afLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/app_firewalls", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list app firewalls: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list AppFirewallList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	if len(list.Items) == 0 {
		output.Infof("No app firewalls found in namespace %q", ns)
		return nil
	}

	tableData := make([]AppFirewallTableRow, 0, len(list.Items))
	for _, af := range list.Items {
		mode := "-"
		ruleSets := "-"

		if spec := af.Spec; spec != nil {
			if m, ok := spec["detection_mode"].(string); ok {
				mode = m
			}
			if rs, ok := spec["use_default_detection_settings"].(bool); ok && rs {
				ruleSets = "default"
			}
		}

		tableData = append(tableData, AppFirewallTableRow{
			Name:      af.Metadata.Name,
			Namespace: af.Metadata.Namespace,
			Mode:      mode,
			RuleSets:  ruleSets,
		})
	}

	return output.Print("table", tableData)
}

func newAppFirewallGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get app firewall details",
		Long:  `Get detailed information about a specific app firewall.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runAppFirewallGet,
	}
	return cmd
}

func runAppFirewallGet(cmd *cobra.Command, args []string) error {
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

	path := fmt.Sprintf("/api/config/namespaces/%s/app_firewalls/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get app firewall: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var af AppFirewall
	if err := resp.DecodeJSON(&af); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, af)
	}

	fmt.Printf("Name:        %s\n", af.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", af.Metadata.Namespace)
	if af.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", af.Metadata.Description)
	}

	return nil
}

func newAppFirewallCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an app firewall",
		Long:  `Create a new app firewall from a specification file.`,
		Example: `  # Create from spec file
  f5xcctl security app-firewall create my-waf --spec-file waf.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runAppFirewallCreate,
	}

	cmd.Flags().StringVarP(&afSpecFile, "spec-file", "f", "", "path to specification file (required)")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runAppFirewallCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(afSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse spec file: %w", err)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/app_firewalls", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create app firewall: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("App firewall %q created in namespace %q", name, ns)
	return nil
}

func newAppFirewallUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an app firewall",
		Args:  cobra.ExactArgs(1),
		RunE:  runAppFirewallUpdate,
	}

	cmd.Flags().StringVarP(&afSpecFile, "spec-file", "f", "", "path to specification file (required)")
	_ = cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runAppFirewallUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(afSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec map[string]interface{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse spec file: %w", err)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/app_firewalls/%s", ns, name)
	resp, err := client.Put(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update app firewall: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("App firewall %q updated in namespace %q", name, ns)
	return nil
}

func newAppFirewallDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an app firewall",
		Args:  cobra.ExactArgs(1),
		RunE:  runAppFirewallDelete,
	}

	cmd.Flags().BoolVarP(&afForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runAppFirewallDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if !afForce {
		fmt.Printf("Are you sure you want to delete app firewall %q? [y/N]: ", name)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/app_firewalls/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete app firewall: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("App firewall %q deleted from namespace %q", name, ns)
	return nil
}
