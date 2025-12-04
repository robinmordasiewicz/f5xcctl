package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/output"
)

// Certificate represents a certificate resource.
type Certificate struct {
	Metadata   ResourceMetadata       `json:"metadata"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	SystemMeta map[string]interface{} `json:"system_metadata,omitempty"`
}

// CertificateList represents a list response.
type CertificateList struct {
	Items []Certificate `json:"items"`
}

// CertificateTableRow for table display.
type CertificateTableRow struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	Expires   string `json:"expires"`
}

var (
	certLabelFilter string
	certFile        string
	keyFile         string
	certChain       string
	certForce       bool
)

func newCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cert",
		Aliases: []string{"certificate", "certs", "certificates"},
		Short:   "Manage certificates",
		Long: `Manage TLS certificates in F5 Distributed Cloud.

Certificates are used for TLS termination on load balancers.

Commands:
  list      List certificates
  get       Get certificate details
  upload    Upload a new certificate
  delete    Delete a certificate`,
	}

	cmd.AddCommand(newCertListCmd())
	cmd.AddCommand(newCertGetCmd())
	cmd.AddCommand(newCertUploadCmd())
	cmd.AddCommand(newCertDeleteCmd())

	return cmd
}

func newCertListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List certificates",
		Long:  `List all certificates in the specified namespace.`,
		Example: `  # List all certificates
  f5xcctl cert list

  # List in a specific namespace
  f5xcctl cert list -n production`,
		RunE: runCertList,
	}

	cmd.Flags().StringVar(&certLabelFilter, "label-filter", "", "filter by label selector")

	return cmd
}

func runCertList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ns := namespace
	if ns == "" {
		ns = "default"
	}

	query := url.Values{}
	if certLabelFilter != "" {
		query.Set("label_filter", certLabelFilter)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/certificates", ns)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to list certificates: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var list CertificateList
	if err := resp.DecodeJSON(&list); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, list.Items)
	}

	if len(list.Items) == 0 {
		output.Infof("No certificates found in namespace %q", ns)
		return nil
	}

	tableData := make([]CertificateTableRow, 0, len(list.Items))
	for _, cert := range list.Items {
		certType := "custom"
		expires := "-"

		if spec := cert.Spec; spec != nil {
			if _, ok := spec["certificate_url"]; ok {
				certType = "url"
			}
		}

		tableData = append(tableData, CertificateTableRow{
			Name:      cert.Metadata.Name,
			Namespace: cert.Metadata.Namespace,
			Type:      certType,
			Expires:   expires,
		})
	}

	return output.Print("table", tableData)
}

func newCertGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Get certificate details",
		Long:  `Get detailed information about a specific certificate.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runCertGet,
	}
	return cmd
}

func runCertGet(cmd *cobra.Command, args []string) error {
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

	path := fmt.Sprintf("/api/config/namespaces/%s/certificates/%s", ns, name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get certificate: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var cert Certificate
	if err := resp.DecodeJSON(&cert); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, cert)
	}

	fmt.Printf("Name:        %s\n", cert.Metadata.Name)
	fmt.Printf("Namespace:   %s\n", cert.Metadata.Namespace)
	if cert.Metadata.Description != "" {
		fmt.Printf("Description: %s\n", cert.Metadata.Description)
	}

	return nil
}

func newCertUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <name>",
		Short: "Upload a certificate",
		Long: `Upload a new TLS certificate with private key.

The certificate and key files should be PEM encoded.`,
		Example: `  # Upload a certificate
  f5xcctl cert upload my-cert --cert-file cert.pem --key-file key.pem

  # Upload with certificate chain
  f5xcctl cert upload my-cert --cert-file cert.pem --key-file key.pem --chain chain.pem`,
		Args: cobra.ExactArgs(1),
		RunE: runCertUpload,
	}

	cmd.Flags().StringVar(&certFile, "cert-file", "", "path to certificate file (PEM format)")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "path to private key file (PEM format)")
	cmd.Flags().StringVar(&certChain, "chain", "", "path to certificate chain file (PEM format)")
	_ = cmd.MarkFlagRequired("cert-file")
	_ = cmd.MarkFlagRequired("key-file")

	return cmd
}

func runCertUpload(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	// Read certificate file
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Read key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	spec := map[string]interface{}{
		"certificate": string(certData),
		"private_key": map[string]interface{}{
			"clear_secret_info": map[string]interface{}{
				"url": "string:///" + string(keyData),
			},
		},
	}

	// Add chain if provided
	if certChain != "" {
		chainData, err := os.ReadFile(certChain)
		if err != nil {
			return fmt.Errorf("failed to read chain file: %w", err)
		}
		spec["certificate_chain"] = string(chainData)
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

	path := fmt.Sprintf("/api/config/namespaces/%s/certificates", ns)
	resp, err := client.Post(ctx, path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to upload certificate: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("Certificate %q uploaded to namespace %q", name, ns)
	return nil
}

func newCertDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a certificate",
		Args:  cobra.ExactArgs(1),
		RunE:  runCertDelete,
	}

	cmd.Flags().BoolVarP(&certForce, "yes", "y", false, "skip confirmation prompt")

	return cmd
}

func runCertDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	if !certForce {
		fmt.Printf("Are you sure you want to delete certificate %q? [y/N]: ", name)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Canceled")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := fmt.Sprintf("/api/config/namespaces/%s/certificates/%s", ns, name)
	resp, err := client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	output.Successf("Certificate %q deleted from namespace %q", name, ns)
	return nil
}
