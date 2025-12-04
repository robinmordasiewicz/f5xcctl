package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/config"
)

var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i", "shell"},
	Short:   "Start interactive shell with auto-completion",
	Long: `Start an interactive shell with intelligent auto-completion.

The interactive mode provides:
  - Tab completion for commands, subcommands, and flags
  - Dynamic resource name completion (namespaces, load balancers, etc.)
  - Command history (use Up/Down arrows)
  - Emacs-style keyboard shortcuts

Examples:
  # Start interactive mode
  f5xcctl interactive

  # Or use the short alias
  f5xcctl i

Keyboard Shortcuts:
  Tab          Auto-complete
  Ctrl+A       Go to beginning of line
  Ctrl+E       Go to end of line
  Ctrl+W       Delete word before cursor
  Ctrl+K       Delete to end of line
  Ctrl+U       Delete to beginning of line
  Ctrl+L       Clear screen
  Ctrl+D       Exit interactive mode`,
	Run: runInteractive,
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

// commandTree defines the CLI command structure for completion.
var commandTree = map[string][]prompt.Suggest{
	"": {
		// kubectl-style verb commands (primary)
		{Text: "get", Description: "Display one or more resources"},
		{Text: "create", Description: "Create a resource from file or stdin"},
		{Text: "delete", Description: "Delete resources by name or file"},
		{Text: "apply", Description: "Apply configuration to a resource"},
		{Text: "replace", Description: "Replace a resource by filename"},
		{Text: "describe", Description: "Show details of a resource"},
		{Text: "label", Description: "Update labels on a resource"},
		{Text: "api-resources", Description: "Print supported API resources"},
		// Legacy resource-based commands
		{Text: "namespace", Description: "Manage namespaces"},
		{Text: "ns", Description: "Manage namespaces (alias)"},
		{Text: "lb", Description: "Manage load balancers"},
		{Text: "loadbalancer", Description: "Manage load balancers (alias)"},
		{Text: "origin", Description: "Manage origin pools"},
		{Text: "security", Description: "Manage security resources"},
		{Text: "sec", Description: "Manage security resources (alias)"},
		{Text: "cert", Description: "Manage certificates"},
		{Text: "certificate", Description: "Manage certificates (alias)"},
		{Text: "dns", Description: "Manage DNS resources"},
		{Text: "monitor", Description: "Manage monitoring resources"},
		{Text: "mon", Description: "Manage monitoring resources (alias)"},
		{Text: "stats", Description: "View statistics and metrics"},
		{Text: "statistics", Description: "View statistics and metrics (alias)"},
		{Text: "metrics", Description: "View statistics and metrics (alias)"},
		{Text: "config", Description: "Manage CLI configuration"},
		{Text: "auth", Description: "Authentication commands"},
		{Text: "configure", Description: "Configure CLI settings"},
		{Text: "version", Description: "Show version information"},
		{Text: "help", Description: "Show help for commands"},
		{Text: "exit", Description: "Exit interactive mode"},
		{Text: "quit", Description: "Exit interactive mode"},
	},
	// Resource type completions for kubectl-style verbs
	"get": {
		{Text: "namespace", Description: "Namespace resources"},
		{Text: "ns", Description: "Namespace resources (alias)"},
		{Text: "http_loadbalancer", Description: "HTTP Load Balancers"},
		{Text: "httplb", Description: "HTTP Load Balancers (alias)"},
		{Text: "tcp_loadbalancer", Description: "TCP Load Balancers"},
		{Text: "tcplb", Description: "TCP Load Balancers (alias)"},
		{Text: "origin_pool", Description: "Origin Pools"},
		{Text: "op", Description: "Origin Pools (alias)"},
		{Text: "app_firewall", Description: "Application Firewalls"},
		{Text: "af", Description: "Application Firewalls (alias)"},
		{Text: "certificate", Description: "TLS Certificates"},
		{Text: "cert", Description: "TLS Certificates (alias)"},
		{Text: "dns_zone", Description: "DNS Zones"},
		{Text: "dnsz", Description: "DNS Zones (alias)"},
		{Text: "healthcheck", Description: "Health Checks"},
		{Text: "hc", Description: "Health Checks (alias)"},
		{Text: "site", Description: "Sites"},
		{Text: "virtual_site", Description: "Virtual Sites"},
		{Text: "vsite", Description: "Virtual Sites (alias)"},
	},
	"create": {
		{Text: "namespace", Description: "Create namespace"},
		{Text: "ns", Description: "Create namespace (alias)"},
		{Text: "-f", Description: "Create from file"},
		{Text: "--filename", Description: "Create from file"},
	},
	"delete": {
		{Text: "namespace", Description: "Delete namespace"},
		{Text: "ns", Description: "Delete namespace (alias)"},
		{Text: "http_loadbalancer", Description: "Delete HTTP Load Balancer"},
		{Text: "httplb", Description: "Delete HTTP Load Balancer (alias)"},
		{Text: "tcp_loadbalancer", Description: "Delete TCP Load Balancer"},
		{Text: "tcplb", Description: "Delete TCP Load Balancer (alias)"},
		{Text: "origin_pool", Description: "Delete Origin Pool"},
		{Text: "op", Description: "Delete Origin Pool (alias)"},
		{Text: "-f", Description: "Delete from file"},
		{Text: "--filename", Description: "Delete from file"},
	},
	"describe": {
		{Text: "namespace", Description: "Describe namespace"},
		{Text: "ns", Description: "Describe namespace (alias)"},
		{Text: "http_loadbalancer", Description: "Describe HTTP Load Balancer"},
		{Text: "httplb", Description: "Describe HTTP Load Balancer (alias)"},
		{Text: "origin_pool", Description: "Describe Origin Pool"},
		{Text: "op", Description: "Describe Origin Pool (alias)"},
	},
	"label": {
		{Text: "namespace", Description: "Label namespace"},
		{Text: "ns", Description: "Label namespace (alias)"},
		{Text: "http_loadbalancer", Description: "Label HTTP Load Balancer"},
		{Text: "httplb", Description: "Label HTTP Load Balancer (alias)"},
	},
	"namespace": {
		{Text: "list", Description: "List all namespaces"},
		{Text: "get", Description: "Get namespace details"},
		{Text: "create", Description: "Create a new namespace"},
		{Text: "delete", Description: "Delete a namespace"},
	},
	"ns": {
		{Text: "list", Description: "List all namespaces"},
		{Text: "get", Description: "Get namespace details"},
		{Text: "create", Description: "Create a new namespace"},
		{Text: "delete", Description: "Delete a namespace"},
	},
	"lb": {
		{Text: "http", Description: "HTTP load balancers"},
		{Text: "tcp", Description: "TCP load balancers"},
		{Text: "udp", Description: "UDP load balancers"},
	},
	"loadbalancer": {
		{Text: "http", Description: "HTTP load balancers"},
		{Text: "tcp", Description: "TCP load balancers"},
		{Text: "udp", Description: "UDP load balancers"},
	},
	"lb http": {
		{Text: "list", Description: "List HTTP load balancers"},
		{Text: "get", Description: "Get HTTP load balancer details"},
		{Text: "create", Description: "Create HTTP load balancer"},
		{Text: "update", Description: "Update HTTP load balancer"},
		{Text: "delete", Description: "Delete HTTP load balancer"},
	},
	"lb tcp": {
		{Text: "list", Description: "List TCP load balancers"},
		{Text: "get", Description: "Get TCP load balancer details"},
		{Text: "create", Description: "Create TCP load balancer"},
		{Text: "update", Description: "Update TCP load balancer"},
		{Text: "delete", Description: "Delete TCP load balancer"},
	},
	"lb udp": {
		{Text: "list", Description: "List UDP load balancers"},
		{Text: "get", Description: "Get UDP load balancer details"},
		{Text: "create", Description: "Create UDP load balancer"},
		{Text: "update", Description: "Update UDP load balancer"},
		{Text: "delete", Description: "Delete UDP load balancer"},
	},
	"origin": {
		{Text: "pool", Description: "Manage origin pools"},
	},
	"origin pool": {
		{Text: "list", Description: "List origin pools"},
		{Text: "get", Description: "Get origin pool details"},
		{Text: "create", Description: "Create origin pool"},
		{Text: "update", Description: "Update origin pool"},
		{Text: "delete", Description: "Delete origin pool"},
	},
	"security": {
		{Text: "app-firewall", Description: "Application firewalls"},
		{Text: "waf", Description: "Web Application Firewall (alias)"},
		{Text: "service-policy", Description: "Service policies"},
		{Text: "bot-defense", Description: "Bot defense configurations"},
		{Text: "rate-limiter", Description: "Rate limiters"},
	},
	"sec": {
		{Text: "app-firewall", Description: "Application firewalls"},
		{Text: "waf", Description: "Web Application Firewall (alias)"},
		{Text: "service-policy", Description: "Service policies"},
		{Text: "bot-defense", Description: "Bot defense configurations"},
		{Text: "rate-limiter", Description: "Rate limiters"},
	},
	"security app-firewall": {
		{Text: "list", Description: "List application firewalls"},
		{Text: "get", Description: "Get firewall details"},
		{Text: "create", Description: "Create application firewall"},
		{Text: "update", Description: "Update application firewall"},
		{Text: "delete", Description: "Delete application firewall"},
	},
	"cert": {
		{Text: "list", Description: "List certificates"},
		{Text: "get", Description: "Get certificate details"},
		{Text: "upload", Description: "Upload a certificate"},
		{Text: "delete", Description: "Delete a certificate"},
	},
	"certificate": {
		{Text: "list", Description: "List certificates"},
		{Text: "get", Description: "Get certificate details"},
		{Text: "upload", Description: "Upload a certificate"},
		{Text: "delete", Description: "Delete a certificate"},
	},
	"dns": {
		{Text: "zone", Description: "DNS zones"},
		{Text: "record", Description: "DNS records"},
		{Text: "load-balancer", Description: "DNS load balancers"},
	},
	"dns zone": {
		{Text: "list", Description: "List DNS zones"},
		{Text: "get", Description: "Get DNS zone details"},
		{Text: "create", Description: "Create DNS zone"},
		{Text: "update", Description: "Update DNS zone"},
		{Text: "delete", Description: "Delete DNS zone"},
	},
	"monitor": {
		{Text: "alert-policy", Description: "Alert policies"},
		{Text: "alert", Description: "Alert policies (alias)"},
		{Text: "synthetic", Description: "Synthetic monitors"},
		{Text: "log-receiver", Description: "Log receivers"},
	},
	"mon": {
		{Text: "alert-policy", Description: "Alert policies"},
		{Text: "alert", Description: "Alert policies (alias)"},
		{Text: "synthetic", Description: "Synthetic monitors"},
		{Text: "log-receiver", Description: "Log receivers"},
	},
	"monitor alert-policy": {
		{Text: "list", Description: "List alert policies"},
		{Text: "get", Description: "Get alert policy details"},
		{Text: "create", Description: "Create alert policy"},
		{Text: "update", Description: "Update alert policy"},
		{Text: "delete", Description: "Delete alert policy"},
	},
	"config": {
		{Text: "set", Description: "Set configuration value"},
		{Text: "get", Description: "Get configuration value"},
		{Text: "list", Description: "List all configuration"},
		{Text: "profiles", Description: "Manage profiles"},
	},
	"config profiles": {
		{Text: "list", Description: "List profiles"},
		{Text: "create", Description: "Create profile"},
		{Text: "use", Description: "Switch to profile"},
		{Text: "delete", Description: "Delete profile"},
	},
	"auth": {
		{Text: "login", Description: "Authenticate with F5XC"},
		{Text: "logout", Description: "Clear authentication"},
		{Text: "status", Description: "Show auth status"},
	},
	"stats": {
		{Text: "lb", Description: "Load balancer statistics"},
		{Text: "site", Description: "Site statistics and status"},
		{Text: "security", Description: "Security event statistics"},
		{Text: "api", Description: "API endpoint statistics"},
	},
	"statistics": {
		{Text: "lb", Description: "Load balancer statistics"},
		{Text: "site", Description: "Site statistics and status"},
		{Text: "security", Description: "Security event statistics"},
		{Text: "api", Description: "API endpoint statistics"},
	},
	"metrics": {
		{Text: "lb", Description: "Load balancer statistics"},
		{Text: "site", Description: "Site statistics and status"},
		{Text: "security", Description: "Security event statistics"},
		{Text: "api", Description: "API endpoint statistics"},
	},
	"stats lb": {
		{Text: "http", Description: "HTTP load balancer statistics"},
		{Text: "tcp", Description: "TCP load balancer statistics"},
		{Text: "udp", Description: "UDP load balancer statistics"},
	},
}

// flagSuggestions provides flag completions for specific commands.
var flagSuggestions = map[string][]prompt.Suggest{
	// kubectl-style verb flags
	"get": {
		{Text: "-A", Description: "List across all namespaces"},
		{Text: "--all-namespaces", Description: "List across all namespaces"},
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "-o", Description: "Output format (json, yaml, table, wide, name)"},
		{Text: "--output", Description: "Output format (json, yaml, table, wide, name)"},
		{Text: "-l", Description: "Label selector"},
		{Text: "--selector", Description: "Label selector"},
		{Text: "--show-labels", Description: "Show labels in output"},
		{Text: "-w", Description: "Watch for changes"},
		{Text: "--watch", Description: "Watch for changes"},
	},
	"create": {
		{Text: "-f", Description: "Filename to create from"},
		{Text: "--filename", Description: "Filename to create from"},
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "--dry-run", Description: "Only print what would be created"},
		{Text: "-R", Description: "Process directory recursively"},
		{Text: "--recursive", Description: "Process directory recursively"},
	},
	"delete": {
		{Text: "-f", Description: "Filename to delete from"},
		{Text: "--filename", Description: "Filename to delete from"},
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "--force", Description: "Skip confirmation prompt"},
		{Text: "--wait", Description: "Wait for deletion to complete"},
	},
	"apply": {
		{Text: "-f", Description: "Filename to apply (required)"},
		{Text: "--filename", Description: "Filename to apply (required)"},
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "--dry-run", Description: "Only print what would be applied"},
		{Text: "-R", Description: "Process directory recursively"},
		{Text: "--recursive", Description: "Process directory recursively"},
	},
	"replace": {
		{Text: "-f", Description: "Filename to replace from (required)"},
		{Text: "--filename", Description: "Filename to replace from (required)"},
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "--dry-run", Description: "Only print what would be replaced"},
	},
	"describe": {
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
	},
	"label": {
		{Text: "-n", Description: "Target namespace"},
		{Text: "--namespace", Description: "Target namespace"},
		{Text: "--dry-run", Description: "Only print what would be changed"},
		{Text: "--overwrite", Description: "Overwrite existing label values"},
	},
	"api-resources": {
		{Text: "--group", Description: "Filter by resource group"},
		{Text: "--short", Description: "Show short names only"},
	},
	// Legacy command flags
	"namespace list": {
		{Text: "--label-filter", Description: "Filter by labels (e.g., 'env=prod')"},
		{Text: "-o", Description: "Output format (json, yaml, table)"},
		{Text: "--output", Description: "Output format (json, yaml, table)"},
	},
	"namespace get": {
		{Text: "-o", Description: "Output format (json, yaml, table)"},
		{Text: "--output", Description: "Output format (json, yaml, table)"},
	},
	"namespace create": {
		{Text: "--description", Description: "Namespace description"},
		{Text: "--labels", Description: "Labels (key=value,...)"},
	},
	"namespace delete": {
		{Text: "--yes", Description: "Skip confirmation prompt"},
	},
	"stats lb http": {
		{Text: "--time-range", Description: "Time range (5m, 15m, 1h, 6h, 24h, 7d)"},
		{Text: "--top", Description: "Number of top items to show"},
		{Text: "--detailed", Description: "Show detailed metrics"},
		{Text: "-n", Description: "Namespace"},
		{Text: "--namespace", Description: "Namespace"},
		{Text: "-o", Description: "Output format (json, yaml, table)"},
	},
	"stats site": {
		{Text: "--time-range", Description: "Time range (5m, 15m, 1h, 6h, 24h, 7d)"},
		{Text: "--detailed", Description: "Show detailed metrics"},
		{Text: "-o", Description: "Output format (json, yaml, table)"},
	},
	"stats security": {
		{Text: "--time-range", Description: "Time range (5m, 15m, 1h, 6h, 24h, 7d)"},
		{Text: "--top", Description: "Number of top items to show"},
		{Text: "-n", Description: "Namespace"},
		{Text: "--namespace", Description: "Namespace"},
		{Text: "-o", Description: "Output format (json, yaml, table)"},
	},
	"stats api": {
		{Text: "--time-range", Description: "Time range (5m, 15m, 1h, 6h, 24h, 7d)"},
		{Text: "--top", Description: "Number of top items to show"},
		{Text: "-n", Description: "Namespace"},
		{Text: "--namespace", Description: "Namespace"},
		{Text: "-o", Description: "Output format (json, yaml, table)"},
	},
}

// globalFlags that can be used with any command.
var globalFlags = []prompt.Suggest{
	{Text: "-A", Description: "All namespaces"},
	{Text: "--all-namespaces", Description: "All namespaces"},
	{Text: "-o", Description: "Output format (json, yaml, table)"},
	{Text: "--output", Description: "Output format (json, yaml, table)"},
	{Text: "--namespace", Description: "Target namespace"},
	{Text: "-n", Description: "Target namespace (short)"},
	{Text: "--debug", Description: "Enable debug output"},
	{Text: "--profile", Description: "Use specific profile"},
	{Text: "--help", Description: "Show help for command"},
	{Text: "-h", Description: "Show help for command (short)"},
}

// cachedNamespaces stores fetched namespace names for completion.
var cachedNamespaces []prompt.Suggest
var namespaceCacheTime time.Time

// interactiveTenant stores the tenant name for the prompt.
var interactiveTenant string

// interactiveNamespace stores the current namespace for the prompt.
var interactiveNamespace string

// getLivePrefix returns the dynamic prompt prefix showing tenant/namespace.
func getLivePrefix() (string, bool) {
	ns := interactiveNamespace
	if ns == "" {
		ns = "default"
	}

	tenant := interactiveTenant
	if tenant == "" {
		tenant = "f5xc"
	}

	// Shorten tenant name if it's too long (remove common suffixes)
	if len(tenant) > 20 {
		// Try to find a shorter form
		parts := strings.Split(tenant, "-")
		if len(parts) > 2 {
			// Take first two parts for readability
			tenant = parts[0] + "-" + parts[1]
		}
	}

	return fmt.Sprintf("%s/%s> ", tenant, ns), true
}

// interactiveCompleter provides contextual completions.
type interactiveCompleter struct{}

func (c *interactiveCompleter) Complete(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	if text == "" {
		return commandTree[""]
	}

	// Split input into args
	args := strings.Fields(text)
	if len(args) == 0 {
		return commandTree[""]
	}

	// Check if we're completing a flag value
	word := d.GetWordBeforeCursor()

	// Check if previous arg is -n or --namespace - provide namespace completion
	if len(args) >= 1 {
		lastArg := args[len(args)-1]
		// If there's a trailing space and last arg is namespace flag
		if strings.HasSuffix(text, " ") && (lastArg == "-n" || lastArg == "--namespace") {
			return c.getNamespaceSuggestions("")
		}
		// If we're in the middle of typing after -n
		if len(args) >= 2 {
			prevArg := args[len(args)-2]
			if (prevArg == "-n" || prevArg == "--namespace") && !strings.HasPrefix(word, "-") {
				return c.getNamespaceSuggestions(word)
			}
		}
	}

	// Check if previous arg is -o or --output - provide output format completion
	if len(args) >= 1 {
		lastArg := args[len(args)-1]
		if strings.HasSuffix(text, " ") && (lastArg == "-o" || lastArg == "--output") {
			return []prompt.Suggest{
				{Text: "json", Description: "JSON output format"},
				{Text: "yaml", Description: "YAML output format"},
				{Text: "table", Description: "Table output format (default)"},
				{Text: "wide", Description: "Wide table with extra columns"},
				{Text: "name", Description: "Resource names only (type/name)"},
			}
		}
		if len(args) >= 2 {
			prevArg := args[len(args)-2]
			if (prevArg == "-o" || prevArg == "--output") && !strings.HasPrefix(word, "-") {
				return prompt.FilterHasPrefix([]prompt.Suggest{
					{Text: "json", Description: "JSON output format"},
					{Text: "yaml", Description: "YAML output format"},
					{Text: "table", Description: "Table output format (default)"},
					{Text: "wide", Description: "Wide table with extra columns"},
					{Text: "name", Description: "Resource names only (type/name)"},
				}, word, true)
			}
		}
	}

	// Check if we're completing a flag
	if strings.HasPrefix(word, "-") {
		return c.completeFlags(args, word)
	}

	// Check if we need to complete a resource name (after get/delete commands)
	if c.needsResourceCompletion(args) {
		return c.completeResources(args, word)
	}

	// Build the command path to look up in the tree
	return c.completeCommand(args, word, text)
}

func (c *interactiveCompleter) completeCommand(args []string, word, text string) []prompt.Suggest {
	// Try progressively longer command paths
	for i := len(args); i >= 0; i-- {
		var path string
		if i > 0 {
			path = strings.Join(args[:i], " ")
		}

		if suggestions, ok := commandTree[path]; ok {
			// If there's a trailing space, show all suggestions for next level
			if strings.HasSuffix(text, " ") {
				return suggestions
			}
			// Otherwise filter by current word
			return prompt.FilterHasPrefix(suggestions, word, true)
		}
	}

	return []prompt.Suggest{}
}

func (c *interactiveCompleter) completeFlags(args []string, word string) []prompt.Suggest {
	// Build command path for flag lookup
	cmdPath := ""
	hasAllNamespaces := false
	hasNamespace := false
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			// Check for mutual exclusivity flags
			if arg == "-A" || arg == "--all-namespaces" {
				hasAllNamespaces = true
			}
			if arg == "-n" || arg == "--namespace" {
				hasNamespace = true
			}
			continue
		}
		if cmdPath != "" {
			cmdPath += " "
		}
		cmdPath += arg
	}

	// Get command-specific flags
	var suggestions []prompt.Suggest
	if flags, ok := flagSuggestions[cmdPath]; ok {
		suggestions = append(suggestions, flags...)
	}

	// Add global flags
	suggestions = append(suggestions, globalFlags...)

	// Filter mutually exclusive flags: -A and -n cannot be used together
	filtered := make([]prompt.Suggest, 0, len(suggestions))
	for _, s := range suggestions {
		// If -A is already used, skip -n/--namespace suggestions
		if hasAllNamespaces && (s.Text == "-n" || s.Text == "--namespace") {
			continue
		}
		// If -n is already used, skip -A/--all-namespaces suggestions
		if hasNamespace && (s.Text == "-A" || s.Text == "--all-namespaces") {
			continue
		}
		filtered = append(filtered, s)
	}

	return prompt.FilterHasPrefix(filtered, word, true)
}

func (c *interactiveCompleter) needsResourceCompletion(args []string) bool {
	if len(args) < 2 {
		return false
	}

	// Commands that need resource name completion
	lastCmd := args[len(args)-1]
	needsResource := lastCmd == "get" || lastCmd == "delete" || lastCmd == "update"

	return needsResource
}

func (c *interactiveCompleter) completeResources(args []string, word string) []prompt.Suggest {
	// Determine resource type from command path
	resourceType := ""
	for _, arg := range args {
		switch arg {
		case "namespace", "ns":
			resourceType = "namespace"
		case "http", "tcp", "udp":
			if resourceType == "" {
				resourceType = "lb-" + arg
			}
		}
	}

	switch resourceType {
	case "namespace":
		return c.getNamespaceSuggestions(word)
	default:
		return []prompt.Suggest{}
	}
}

func (c *interactiveCompleter) getNamespaceSuggestions(prefix string) []prompt.Suggest {
	// Use cache if fresh (less than 30 seconds old)
	if time.Since(namespaceCacheTime) < 30*time.Second && len(cachedNamespaces) > 0 {
		return prompt.FilterHasPrefix(cachedNamespaces, prefix, true)
	}

	// Try to fetch namespaces
	client, err := getClient()
	if err != nil {
		return []prompt.Suggest{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/api/web/namespaces", nil)
	if err != nil || !resp.IsSuccess() {
		return []prompt.Suggest{}
	}

	var listResp NamespaceListResponse
	if err := resp.DecodeJSON(&listResp); err != nil {
		return []prompt.Suggest{}
	}

	// Update cache
	cachedNamespaces = make([]prompt.Suggest, 0, len(listResp.Items))
	for _, ns := range listResp.Items {
		desc := ns.Description
		if desc == "" {
			desc = "Namespace"
		}
		cachedNamespaces = append(cachedNamespaces, prompt.Suggest{
			Text:        ns.Name,
			Description: desc,
		})
	}
	namespaceCacheTime = time.Now()

	return prompt.FilterHasPrefix(cachedNamespaces, prefix, true)
}

// executor handles command execution in interactive mode.
func executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle exit commands
	if input == "exit" || input == "quit" {
		fmt.Println("Goodbye!")
		os.Exit(0)
	}

	// Handle help
	if input == "help" {
		_ = rootCmd.Help()
		return
	}

	// Split input and execute as f5xcctl command
	args := strings.Fields(input)

	// Reset flags to defaults before each command
	rootCmd.SetArgs(args)

	// Execute and handle errors (SilenceErrors is true, so we must print errors ourselves)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	// Reset command state for next execution
	resetCommandState()
}

// resetCommandState resets global variables that might persist between commands.
func resetCommandState() {
	// Update interactive namespace if it was changed via -n flag
	if namespace != "" {
		interactiveNamespace = namespace
	}

	// Reset output format
	outputFmt = "table"
	// Reset namespace to use the interactive default (not the flag value)
	namespace = interactiveNamespace
	// Reset namespace flags
	nsDescription = ""
	nsLabels = nil
	nsLabelFilter = ""
	nsForce = false
}

func runInteractive(cmd *cobra.Command, args []string) {
	// Initialize tenant and namespace from config
	initInteractivePrompt()

	fmt.Println("F5 Distributed Cloud Interactive Shell")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")
	fmt.Println("Use Tab for auto-completion")
	fmt.Println()

	completer := &interactiveCompleter{}

	p := prompt.New(
		executor,
		completer.Complete,
		prompt.OptionLivePrefix(getLivePrefix),
		prompt.OptionPrefixTextColor(prompt.Cyan),
		prompt.OptionTitle("F5XC Interactive Shell"),
		prompt.OptionMaxSuggestion(15),
		prompt.OptionShowCompletionAtStart(),
		prompt.OptionCompletionOnDown(),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.Cyan),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(prompt.LightGray),
		prompt.OptionSelectedDescriptionBGColor(prompt.Cyan),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionPreviewSuggestionTextColor(prompt.Blue),
		prompt.OptionScrollbarBGColor(prompt.DarkGray),
		prompt.OptionScrollbarThumbColor(prompt.Cyan),
	)

	p.Run()
}

// initInteractivePrompt initializes the tenant and namespace for the prompt.
func initInteractivePrompt() {
	// Load config to get tenant
	cfg, err := config.Load("", "")
	if err == nil && cfg != nil {
		profile := cfg.GetCurrentProfile()
		if profile != nil {
			interactiveTenant = profile.Tenant
			if profile.DefaultNamespace != "" {
				interactiveNamespace = profile.DefaultNamespace
			}
		}
	}

	// Use global namespace if set via flag
	if namespace != "" {
		interactiveNamespace = namespace
	}

	// Use global tenant if set via flag
	if tenant != "" {
		interactiveTenant = tenant
	}
}
