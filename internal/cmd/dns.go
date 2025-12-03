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

// DNSZone represents a DNS zone resource.
type DNSZone struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
}

// DNSZoneList represents a list response.
type DNSZoneList struct {
	Items []DNSZone `json:"items"`
}

// DNSZoneTableRow for table display.
type DNSZoneTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Domain    string `json:"domain"`
	Type      string `json:"type"`
}

var (
	dnsLabelFilter string
	dnsSpecFile    string
	dnsForce       bool
)

func newDNSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS resources",
		Long: `Manage DNS resources in F5 Distributed Cloud.

DNS management includes zones, records, and DNS load balancing.

Subcommands:
  zone    DNS zones`,
	}

	cmd.AddCommand(newDNSZoneCmd())

	return cmd
}

func newDNSZoneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "zone",
		Aliases: []string{"zones"},
		Short:   "Manage DNS zones",
		Long: `Manage DNS zones in F5 Distributed Cloud.

DNS zones allow you to host and manage DNS records for your domains.`,
	}

	cmd.AddCommand(newDNSZoneListCmd())
	cmd.AddCommand(newDNSZoneGetCmd())
	cmd.AddCommand(newDNSZoneCreateCmd())
	cmd.AddCommand(newDNSZoneUpdateCmd())
	cmd.AddCommand(newDNSZoneDeleteCmd())

	return cmd
}

func newDNSZoneListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List DNS zones",
		Long:  `List all DNS zones in the specified namespace.`,
		Example: `  # List all DNS zones
  f5xcctl dns zone list

  # List in a specific namespace
  f5xcctl dns zone list -n production`,
		RunE: runDNSZoneList,
	}

	cmd.Flags().StringVar(&dnsLabelFilter, "label-filter", "", "filter by label selector")

	return cmd
}

func runDNSZoneList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	query := url.Values{}
	if dnsLabelFilter != "" {
		query.Set("label_filter", dnsLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/dns_zones", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list DNS zones: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list DNSZoneList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	if len(list.Items) == 0 {
		output.Info("No DNS zones found in namespace %q", ns)
		return nil
	}

	tableData := make([]DNSZoneTableRow, 0, len(list.Items))
	for _, zone := range list.Items {
		domain := "-"
		zoneType := "primary"

		if spec := zone.Spec; spec != nil {
			if d, ok := spec["domain"].(string); ok {
				domain = d
			}
		}

		tableData = append(tableData, DNSZoneTableRow{
			Name:      zone.Metadata.Name,
			Namespace: zone.Metadata.Namespace,
			Domain:    domain,
			Type:      zoneType,
		})
	}

	return output.Print("table", tableData)
}

func newDNSZoneGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get DNS zone details",
		Long:  `Get detailed information about a specific DNS zone.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runDNSZoneGet,
	}
	return cmd
}

func runDNSZoneGet(cmd *cobra.Command, args []string) error {
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

	path := fmt.Sprintf("/api/config/namespaces/%s/dns_zones/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get DNS zone: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var zone DNSZone
	if err := resp.DecodeJSON(&zone); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, zone)
	}

	fmt.Printf("Name:        %s\n", zone.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", zone.Metadata.Namespace)
	if zone.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", zone.Metadata.Description)
	}

	if spec := zone.Spec; spec != nil {
		if domain, ok := spec["domain"].(string); ok {
			fmt.Printf("Domain:      %s\n", domain)
		}
	}

	return nil
}

func newDNSZoneCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a DNS zone",
		Long:  `Create a new DNS zone from a specification file.`,
		Example: `  # Create from spec file
  f5xcctl dns zone create my-zone --spec-file zone.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: runDNSZoneCreate,
	}

	cmd.Flags().StringVarP(&dnsSpecFile, "spec-file", "f", "", "path to specification file (required)")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runDNSZoneCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(dnsSpecFile)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/dns_zones", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create DNS zone: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("DNS zone %q created in namespace %q", name, ns)
	return nil
}

func newDNSZoneUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a DNS zone",
		Args:  cobra.ExactArgs(1),
		RunE:  runDNSZoneUpdate,
	}

	cmd.Flags().StringVarP(&dnsSpecFile, "spec-file", "f", "", "path to specification file (required)")
	cmd.MarkFlagRequired("spec-file")

	return cmd
}

func runDNSZoneUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	data, err := os.ReadFile(dnsSpecFile)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/dns_zones/%s", ns, name)
	resp, err := client.Put(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update DNS zone: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("DNS zone %q updated in namespace %q", name, ns)
	return nil
}

func newDNSZoneDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a DNS zone",
		Args:  cobra.ExactArgs(1),
		RunE:  runDNSZoneDelete,
	}

	cmd.Flags().BoolVarP(&dnsForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runDNSZoneDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if !dnsForce {
		fmt.Printf("Are you sure you want to delete DNS zone %q? [y/N]: ", name)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/dns_zones/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete DNS zone: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Success("DNS zone %q deleted from namespace %q", name, ns)
	return nil
}
