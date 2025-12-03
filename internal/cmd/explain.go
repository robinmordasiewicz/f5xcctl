package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var explainRecursive bool

var explainCmd = &cobra.Command{
	Use:   "explain <resource-type> [field-path]",
	Short: "Get documentation for a resource",
	Long: `Get documentation for a resource type and its fields.

Explains resource types and their fields, similar to kubectl explain.

Examples:
  # Explain a resource type
  f5xcctl explain http_loadbalancer

  # Explain using short name
  f5xcctl explain httplb

  # Explain a specific field
  f5xcctl explain http_loadbalancer.spec.domains

  # Show all fields recursively
  f5xcctl explain http_loadbalancer --recursive`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExplain,
}

func init() {
	explainCmd.Flags().BoolVar(&explainRecursive, "recursive", false, "Show fields recursively")

	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	resourceArg := args[0]

	// Parse resource type and optional field path
	parts := strings.SplitN(resourceArg, ".", 2)
	resourceType := parts[0]
	fieldPath := ""
	if len(parts) > 1 {
		fieldPath = parts[1]
	}

	rt := ResolveResourceType(resourceType)
	if rt == nil {
		return fmt.Errorf("unknown resource type: %s\n\nUse 'f5xcctl api-resources' to list available resource types", resourceType)
	}

	// If a field path is specified, explain that field
	if fieldPath != "" {
		return explainField(rt, fieldPath)
	}

	// Otherwise, explain the resource type
	return explainResourceType(rt)
}

// explainResourceType explains a resource type.
func explainResourceType(rt *ResourceType) error {
	fmt.Printf("KIND:     %s\n", rt.Kind)
	fmt.Printf("VERSION:  v1\n")
	fmt.Println()
	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("     %s\n", rt.Description)
	fmt.Println()
	fmt.Printf("GROUP:    %s\n", rt.Group)
	fmt.Printf("NAMESPACED: %t\n", rt.Namespaced)
	fmt.Println()
	fmt.Printf("ALIASES:\n")
	if rt.Short != "" {
		fmt.Printf("     %s (short)\n", rt.Short)
	}
	for _, alias := range rt.Aliases {
		fmt.Printf("     %s\n", alias)
	}
	fmt.Println()
	fmt.Printf("SUPPORTED VERBS:\n")
	for _, verb := range rt.SupportedVerbs {
		fmt.Printf("     %s\n", verb)
	}
	fmt.Println()

	// Show standard fields
	fmt.Printf("FIELDS:\n")
	fields := getResourceFields(rt)
	for _, field := range fields {
		if explainRecursive {
			printFieldRecursive(field, "   ")
		} else {
			fmt.Printf("   %s\t<%s>\n", field.Name, field.Type)
			if field.Description != "" {
				fmt.Printf("     %s\n", field.Description)
			}
		}
	}

	return nil
}

// explainField explains a specific field.
func explainField(rt *ResourceType, fieldPath string) error {
	fields := getResourceFields(rt)
	field := findField(fields, fieldPath)

	if field == nil {
		return fmt.Errorf("field %q not found in %s", fieldPath, rt.Name)
	}

	fmt.Printf("KIND:     %s\n", rt.Kind)
	fmt.Printf("FIELD:    %s\n", fieldPath)
	fmt.Printf("TYPE:     %s\n", field.Type)
	if field.Required {
		fmt.Printf("REQUIRED: true\n")
	}
	fmt.Println()
	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("     %s\n", field.Description)

	if len(field.Children) > 0 {
		fmt.Println()
		fmt.Printf("FIELDS:\n")
		for _, child := range field.Children {
			fmt.Printf("   %s\t<%s>\n", child.Name, child.Type)
			if child.Description != "" {
				fmt.Printf("     %s\n", child.Description)
			}
		}
	}

	return nil
}

// ResourceField represents a field in a resource schema.
type ResourceField struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Children    []ResourceField
}

// findField finds a field by path.
func findField(fields []ResourceField, path string) *ResourceField {
	parts := strings.SplitN(path, ".", 2)
	name := parts[0]

	for _, field := range fields {
		if field.Name == name {
			if len(parts) == 1 {
				return &field
			}
			return findField(field.Children, parts[1])
		}
	}

	return nil
}

// printFieldRecursive prints a field and its children recursively.
func printFieldRecursive(field ResourceField, indent string) {
	fmt.Printf("%s%s\t<%s>", indent, field.Name, field.Type)
	if field.Required {
		fmt.Print(" -required-")
	}
	fmt.Println()
	if field.Description != "" {
		fmt.Printf("%s  %s\n", indent, field.Description)
	}
	for _, child := range field.Children {
		printFieldRecursive(child, indent+"   ")
	}
}

// getResourceFields returns the field schema for a resource type
// This is a simplified schema - in production, this would be fetched from the API.
func getResourceFields(rt *ResourceType) []ResourceField {
	// Common fields for all resources
	commonFields := []ResourceField{
		{
			Name:        "kind",
			Type:        "string",
			Description: "Kind is a string value representing the REST resource this object represents",
			Required:    true,
		},
		{
			Name:        "metadata",
			Type:        "Object",
			Description: "Standard object's metadata",
			Required:    true,
			Children: []ResourceField{
				{Name: "name", Type: "string", Description: "Name must be unique within a namespace", Required: true},
				{Name: "namespace", Type: "string", Description: "Namespace defines the space within which the name must be unique"},
				{Name: "labels", Type: "map[string]string", Description: "Map of string keys and values for organizing objects"},
				{Name: "annotations", Type: "map[string]string", Description: "Map of string keys and values for storing arbitrary metadata"},
				{Name: "description", Type: "string", Description: "Human-readable description of the resource"},
			},
		},
		{
			Name:        "spec",
			Type:        "Object",
			Description: "Specification of the desired behavior of the resource",
			Required:    true,
			Children:    getSpecFields(rt),
		},
	}

	return commonFields
}

// getSpecFields returns spec fields based on resource type.
func getSpecFields(rt *ResourceType) []ResourceField {
	switch rt.Name {
	case "http_loadbalancer":
		return []ResourceField{
			{Name: "domains", Type: "[]string", Description: "List of domains this load balancer serves", Required: true},
			{Name: "http", Type: "Object", Description: "HTTP configuration settings", Children: []ResourceField{
				{Name: "port", Type: "integer", Description: "HTTP port number (default 80)"},
				{Name: "dns_volterra_managed", Type: "boolean", Description: "Let F5XC manage DNS for the domains"},
			}},
			{Name: "https", Type: "Object", Description: "HTTPS configuration settings", Children: []ResourceField{
				{Name: "port", Type: "integer", Description: "HTTPS port number (default 443)"},
				{Name: "http_redirect", Type: "boolean", Description: "Redirect HTTP to HTTPS"},
				{Name: "tls_config", Type: "Object", Description: "TLS configuration"},
			}},
			{Name: "default_route_pools", Type: "[]Object", Description: "Default origin pools for this load balancer"},
			{Name: "routes", Type: "[]Object", Description: "Custom routing rules"},
			{Name: "waf", Type: "Object", Description: "Web Application Firewall configuration"},
			{Name: "rate_limiter", Type: "Object", Description: "Rate limiting configuration"},
			{Name: "advertise_on_public_default_vip", Type: "boolean", Description: "Advertise on the default public VIP"},
		}
	case "tcp_loadbalancer":
		return []ResourceField{
			{Name: "listen_port", Type: "integer", Description: "Port to listen on", Required: true},
			{Name: "origin_pools", Type: "[]Object", Description: "Origin pools for this load balancer", Required: true},
			{Name: "advertise_on_public_default_vip", Type: "boolean", Description: "Advertise on the default public VIP"},
		}
	case "origin_pool":
		return []ResourceField{
			{Name: "origin_servers", Type: "[]Object", Description: "List of origin servers", Required: true, Children: []ResourceField{
				{Name: "public_ip", Type: "Object", Description: "Public IP address of the origin"},
				{Name: "private_ip", Type: "Object", Description: "Private IP address of the origin"},
				{Name: "k8s_service", Type: "Object", Description: "Kubernetes service reference"},
				{Name: "consul_service", Type: "Object", Description: "Consul service reference"},
			}},
			{Name: "port", Type: "integer", Description: "Port number for origin servers"},
			{Name: "loadbalancer_algorithm", Type: "string", Description: "Load balancing algorithm (round_robin, least_active, etc.)"},
			{Name: "endpoint_selection", Type: "string", Description: "How endpoints are selected"},
			{Name: "healthcheck", Type: "[]Object", Description: "Health check references"},
		}
	case "app_firewall":
		return []ResourceField{
			{Name: "allow_all_response_codes", Type: "boolean", Description: "Allow all HTTP response codes"},
			{Name: "blocking", Type: "Object", Description: "Blocking mode configuration"},
			{Name: "detection_settings", Type: "Object", Description: "Detection settings for WAF"},
			{Name: "bot_protection_setting", Type: "Object", Description: "Bot protection configuration"},
		}
	case "namespace":
		return []ResourceField{
			{Name: "description", Type: "string", Description: "Description of the namespace"},
		}
	case "certificate":
		return []ResourceField{
			{Name: "certificate_url", Type: "string", Description: "URL to the certificate"},
			{Name: "private_key", Type: "Object", Description: "Private key configuration"},
			{Name: "description", Type: "string", Description: "Description of the certificate"},
		}
	case "dns_zone":
		return []ResourceField{
			{Name: "dns_zone_type", Type: "string", Description: "Type of DNS zone (primary/secondary)"},
			{Name: "primary", Type: "Object", Description: "Primary zone configuration"},
		}
	case "healthcheck":
		return []ResourceField{
			{Name: "http_health_check", Type: "Object", Description: "HTTP health check configuration"},
			{Name: "tcp_health_check", Type: "Object", Description: "TCP health check configuration"},
			{Name: "timeout", Type: "integer", Description: "Health check timeout in milliseconds"},
			{Name: "interval", Type: "integer", Description: "Health check interval in milliseconds"},
			{Name: "unhealthy_threshold", Type: "integer", Description: "Number of failures before marking unhealthy"},
			{Name: "healthy_threshold", Type: "integer", Description: "Number of successes before marking healthy"},
		}
	default:
		// Generic spec fields
		return []ResourceField{
			{Name: "...", Type: "various", Description: "Resource-specific configuration options"},
		}
	}
}

// GetAllResourceFields returns all available resources with their short descriptions for completion.
func GetAllResourceFields() []string {
	var resources []string
	for name, rt := range ResourceRegistry {
		resources = append(resources, fmt.Sprintf("%s\t%s", name, rt.Description))
	}
	sort.Strings(resources)
	return resources
}
