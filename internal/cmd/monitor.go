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

// AlertPolicy represents an alert policy resource.
type AlertPolicy struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
}

// AlertPolicyList represents a list response.
type AlertPolicyList struct {
	Items []AlertPolicy `json:"items"`
}

// AlertPolicyTableRow for table display.
type AlertPolicyTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Severity  string `json:"severity"`
	Receivers string `json:"receivers"`
}

var (
	alertLabelFilter string
	alertSpecFile    string
	alertForce       bool
)

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"monitoring", "mon"},
		Short:   "Manage monitoring resources",
		Long: `Manage monitoring and observability resources in F5 Distributed Cloud.

Subcommands:
  alert-policy    Alert policies
  synthetic       Synthetic monitors (coming soon)
  log-receiver    Log receivers (coming soon)`,
	}

	cmd.AddCommand(newAlertPolicyCmd())

	return cmd
}

func newAlertPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "alert-policy",
		Aliases: []string{"alert", "alerts"},
		Short:   "Manage alert policies",
		Long: `Manage alert policies in F5 Distributed Cloud.

Alert policies define conditions and notifications for monitoring events.`,
	}

	cmd.AddCommand(newAlertPolicyListCmd())
	cmd.AddCommand(newAlertPolicyGetCmd())
	cmd.AddCommand(newAlertPolicyCreateCmd())
	cmd.AddCommand(newAlertPolicyUpdateCmd())
	cmd.AddCommand(newAlertPolicyDeleteCmd())

	return cmd
}

func newAlertPolicyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List alert policies",
		Long:  `List all alert policies in the specified namespace.`,
		Example: `  # List all alert policies
  f5xcctl monitor alert-policy list

  # List in a specific namespace
  f5xcctl monitor alert-policy list -n production`,
		RunE: runAlertPolicyList,
	}

	cmd.Flags().StringVar(&alertLabelFilter, "label-filter", "", "filter by label selector")

	return cmd
}

func runAlertPolicyList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	query := url.Values{}
	if alertLabelFilter != "" {
		query.Set("label_filter", alertLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/alert_policys", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list alert policies: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list AlertPolicyList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	if len(list.Items) == 0 {
		output.Info("No alert policies found in namespace %q", ns)
		return nil
	}

	tableData := make([]AlertPolicyTableRow, 0, len(list.Items))
	for _, policy := range list.Items {
		severity := "-"
		receivers := "-"

		if spec := policy.Spec; spec != nil {
			if s, ok := spec["severity"].(string); ok {
				severity = s
			}
			if r, ok := spec["receivers"].([]interface{}); ok {
				receivers = fmt.Sprintf("%d configured", len(r))
			}
		}

		tableData = append(tableData, AlertPolicyTableRow{
			Name:      policy.Metadata.Name,
			Namespace: policy.Metadata.Namespace,
			Severity:  severity,
			Receivers: receivers,
		})
	}

	return output.Print("table", tableData)
}

func newAlertPolicyGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get alert policy details",
		Long:  `Get detailed information about a specific alert policy.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runAlertPolicyGet,
	}
	return cmd
}

func runAlertPolicyGet(cmd *cobra.Command, args []string) error {
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

	path := fmt.Sprintf("/api/config/namespaces/%s/alert_policys/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get alert policy: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var policy AlertPolicy
	if err := resp.DecodeJSON(&policy); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, policy)
	}

	fmt.Printf("Name:        %s\n", policy.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", policy.Metadata.Namespace)
	if policy.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", policy.Metadata.Description)
	}

	return nil
}

func newAlertPolicyCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an alert policy",
		Long:  `Create a new alert policy from a specification file.`,
		Example: `  # Create from spec file
  f5xcctl monitor alert-policy create my-policy --spec-file policy.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runAlertPolicyCreate,
	}

	cmd.Flags().StringVarP(&alertSpecFile, "spec-file", "f", "", "path to specification file (required)")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runAlertPolicyCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(alertSpecFile)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/alert_policys", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create alert policy: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("Alert policy %q created in namespace %q", name, ns)
	return nil
}

func newAlertPolicyUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an alert policy",
		Args:  cobra.ExactArgs(1),
		RunE:  runAlertPolicyUpdate,
	}

	cmd.Flags().StringVarP(&alertSpecFile, "spec-file", "f", "", "path to specification file (required)")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runAlertPolicyUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(alertSpecFile)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/alert_policys/%s", ns, name)
	resp, err := client.Put(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update alert policy: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("Alert policy %q updated in namespace %q", name, ns)
	return nil
}

func newAlertPolicyDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an alert policy",
		Args:  cobra.ExactArgs(1),
		RunE:  runAlertPolicyDelete,
	}

	cmd.Flags().BoolVarP(&alertForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runAlertPolicyDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if !alertForce {
		fmt.Printf("Are you sure you want to delete alert policy %q? [y/N]: ", name)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/alert_policys/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete alert policy: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("Alert policy %q deleted from namespace %q", name, ns)
	return nil
}
