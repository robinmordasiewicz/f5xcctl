package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has expected subcommands
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "f5xcctl", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Long)

	// Verify subcommands are registered
	subcommands := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		subcommands[cmd.Use] = true
	}

	// Check expected commands exist
	expectedCommands := []string{
		"version",
		"configure",
		"config",
		"auth",
		"namespace",
		"lb",
		"origin",
		"security",
		"cert",
		"dns",
		"monitor",
	}

	for _, expected := range expectedCommands {
		assert.True(t, subcommands[expected], "Expected command %q not found", expected)
	}
}

func TestVersionCommand(t *testing.T) {
	assert.NotNil(t, versionCmd)
	assert.Equal(t, "version", versionCmd.Use)
}

func TestSetVersionInfo(t *testing.T) {
	SetVersionInfo("1.0.0", "abc123", "2024-01-01")

	assert.Equal(t, "1.0.0", versionInfo.Version)
	assert.Equal(t, "abc123", versionInfo.Commit)
	assert.Equal(t, "2024-01-01", versionInfo.Date)
}

func TestNamespaceCommand(t *testing.T) {
	assert.NotNil(t, namespaceCmd)
	assert.Equal(t, "namespace", namespaceCmd.Use)
	assert.Contains(t, namespaceCmd.Aliases, "ns")

	// Verify subcommands
	subcommands := make(map[string]bool)
	for _, cmd := range namespaceCmd.Commands() {
		subcommands[cmd.Use] = true
	}

	assert.True(t, subcommands["list"])
	assert.True(t, subcommands["get <name>"])
	assert.True(t, subcommands["create <name>"])
	assert.True(t, subcommands["delete <name>"])
}

func TestLBCommand(t *testing.T) {
	lbCmd := newLBCmd()
	assert.NotNil(t, lbCmd)
	assert.Equal(t, "lb", lbCmd.Use)
	assert.Contains(t, lbCmd.Aliases, "loadbalancer")

	// Verify http subcommand exists
	var httpCmd *cobra.Command
	for _, cmd := range lbCmd.Commands() {
		if cmd.Use == "http" {
			httpCmd = cmd
			break
		}
	}
	assert.NotNil(t, httpCmd, "http subcommand not found")

	// Verify http subcommands
	if httpCmd != nil {
		subcommands := make(map[string]bool)
		for _, cmd := range httpCmd.Commands() {
			subcommands[cmd.Use] = true
		}
		assert.True(t, subcommands["list"])
		assert.True(t, subcommands["get <name>"])
		assert.True(t, subcommands["create <name>"])
		assert.True(t, subcommands["update <name>"])
		assert.True(t, subcommands["delete <name>"])
	}
}

func TestOriginCommand(t *testing.T) {
	originCmd := newOriginCmd()
	assert.NotNil(t, originCmd)
	assert.Equal(t, "origin", originCmd.Use)

	// Verify pool subcommand exists
	var poolCmd *cobra.Command
	for _, cmd := range originCmd.Commands() {
		if cmd.Use == "pool" {
			poolCmd = cmd
			break
		}
	}
	assert.NotNil(t, poolCmd, "pool subcommand not found")
}

func TestSecurityCommand(t *testing.T) {
	secCmd := newSecurityCmd()
	assert.NotNil(t, secCmd)
	assert.Equal(t, "security", secCmd.Use)
	assert.Contains(t, secCmd.Aliases, "sec")

	// Verify app-firewall subcommand exists
	var afCmd *cobra.Command
	for _, cmd := range secCmd.Commands() {
		if cmd.Use == "app-firewall" {
			afCmd = cmd
			break
		}
	}
	assert.NotNil(t, afCmd, "app-firewall subcommand not found")
	assert.Contains(t, afCmd.Aliases, "waf")
}

func TestCertCommand(t *testing.T) {
	certCmd := newCertCmd()
	assert.NotNil(t, certCmd)
	assert.Equal(t, "cert", certCmd.Use)
	assert.Contains(t, certCmd.Aliases, "certificate")

	subcommands := make(map[string]bool)
	for _, cmd := range certCmd.Commands() {
		subcommands[cmd.Use] = true
	}
	assert.True(t, subcommands["list"])
	assert.True(t, subcommands["get <name>"])
	assert.True(t, subcommands["upload <name>"])
	assert.True(t, subcommands["delete <name>"])
}

func TestDNSCommand(t *testing.T) {
	dnsCmd := newDNSCmd()
	assert.NotNil(t, dnsCmd)
	assert.Equal(t, "dns", dnsCmd.Use)

	// Verify zone subcommand exists
	var zoneCmd *cobra.Command
	for _, cmd := range dnsCmd.Commands() {
		if cmd.Use == "zone" {
			zoneCmd = cmd
			break
		}
	}
	assert.NotNil(t, zoneCmd, "zone subcommand not found")
}

func TestMonitorCommand(t *testing.T) {
	monCmd := newMonitorCmd()
	assert.NotNil(t, monCmd)
	assert.Equal(t, "monitor", monCmd.Use)
	assert.Contains(t, monCmd.Aliases, "mon")

	// Verify alert-policy subcommand exists
	var alertCmd *cobra.Command
	for _, cmd := range monCmd.Commands() {
		if cmd.Use == "alert-policy" {
			alertCmd = cmd
			break
		}
	}
	assert.NotNil(t, alertCmd, "alert-policy subcommand not found")
	assert.Contains(t, alertCmd.Aliases, "alert")
}

func TestGetOutputFormatter(t *testing.T) {
	// Test default
	outputFmt = "table"
	formatter := GetOutputFormatter()
	assert.NotNil(t, formatter)

	// Test JSON
	outputFmt = "json"
	formatter = GetOutputFormatter()
	assert.NotNil(t, formatter)

	// Test YAML
	outputFmt = "yaml"
	formatter = GetOutputFormatter()
	assert.NotNil(t, formatter)

	// Reset
	outputFmt = "table"
}

func TestGetNamespace(t *testing.T) {
	// Test with namespace set
	namespace = "my-namespace"
	assert.Equal(t, "my-namespace", GetNamespace())

	// Test default
	namespace = ""
	assert.Equal(t, "default", GetNamespace())
}

func TestGetTenant(t *testing.T) {
	tenant = "my-tenant"
	assert.Equal(t, "my-tenant", GetTenant())
	tenant = ""
}

func TestGetAPIURL(t *testing.T) {
	apiURL = "https://api.example.com"
	assert.Equal(t, "https://api.example.com", GetAPIURL())
	apiURL = ""
}

func TestIsDebug(t *testing.T) {
	debug = true
	assert.True(t, IsDebug())

	debug = false
	assert.False(t, IsDebug())
}

func TestReadSpecFile(t *testing.T) {
	// Test with valid YAML content
	yamlContent := `domains:
  - example.com
origin_pool:
  name: my-pool
`
	// Create temp file
	tmpfile, err := os.CreateTemp("", "spec-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(yamlContent)
	assert.NoError(t, err)
	tmpfile.Close()

	spec, err := readSpecFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Contains(t, spec, "domains")
	assert.Contains(t, spec, "origin_pool")
}

func TestReadSpecFileNotFound(t *testing.T) {
	_, err := readSpecFile("/nonexistent/file.yaml")
	assert.Error(t, err)
}

func TestReadSpecFileInvalidYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "invalid-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString("invalid: yaml: content: [")
	assert.NoError(t, err)
	tmpfile.Close()

	_, err = readSpecFile(tmpfile.Name())
	assert.Error(t, err)
}

// Test command execution helpers.
func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()
	return buf.String(), err
}

func TestVersionCommandExecution(t *testing.T) {
	SetVersionInfo("test-version", "test-commit", "test-date")

	output, err := executeCommand("version")
	assert.NoError(t, err)
	assert.Contains(t, output, "test-version")
}

// =============================================================================
// CLI Command Execution Tests
// =============================================================================

// TestRootHelpExecution tests the root help command.
func TestRootHelpExecution(t *testing.T) {
	output, err := executeCommand("--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "f5xcctl is a command-line interface")
	assert.Contains(t, output, "Available Commands:")
	assert.Contains(t, output, "namespace")
	assert.Contains(t, output, "lb")
	assert.Contains(t, output, "version")
}

// TestNamespaceHelpExecution tests namespace command help.
func TestNamespaceHelpExecution(t *testing.T) {
	output, err := executeCommand("namespace", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud namespaces")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "delete")
}

// TestNamespaceListHelpExecution tests namespace list help.
func TestNamespaceListHelpExecution(t *testing.T) {
	output, err := executeCommand("namespace", "list", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "List all namespaces")
	assert.Contains(t, output, "--label-filter")
}

// TestNamespaceGetMissingArg tests namespace get without name.
func TestNamespaceGetMissingArg(t *testing.T) {
	_, err := executeCommand("namespace", "get")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestNamespaceCreateMissingArg tests namespace create without name.
func TestNamespaceCreateMissingArg(t *testing.T) {
	_, err := executeCommand("namespace", "create")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestNamespaceDeleteMissingArg tests namespace delete without name.
func TestNamespaceDeleteMissingArg(t *testing.T) {
	_, err := executeCommand("namespace", "delete")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestLBHelpExecution tests lb command help.
func TestLBHelpExecution(t *testing.T) {
	output, err := executeCommand("lb", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud load balancers")
	assert.Contains(t, output, "http")
	assert.Contains(t, output, "tcp")
}

// TestLBHTTPHelpExecution tests lb http command help.
func TestLBHTTPHelpExecution(t *testing.T) {
	output, err := executeCommand("lb", "http", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "HTTP load balancer")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "update")
	assert.Contains(t, output, "delete")
}

// TestLBHTTPGetMissingArg tests lb http get without name.
func TestLBHTTPGetMissingArg(t *testing.T) {
	_, err := executeCommand("lb", "http", "get")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestLBHTTPCreateMissingArg tests lb http create without name.
func TestLBHTTPCreateMissingArg(t *testing.T) {
	_, err := executeCommand("lb", "http", "create")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestOriginHelpExecution tests origin command help.
func TestOriginHelpExecution(t *testing.T) {
	output, err := executeCommand("origin", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "origin")
	assert.Contains(t, output, "pool")
}

// TestOriginPoolHelpExecution tests origin pool command help.
func TestOriginPoolHelpExecution(t *testing.T) {
	output, err := executeCommand("origin", "pool", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "origin pool")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "create")
}

// TestSecurityHelpExecution tests security command help.
func TestSecurityHelpExecution(t *testing.T) {
	output, err := executeCommand("security", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud security resources")
	assert.Contains(t, output, "app-firewall")
	assert.Contains(t, output, "service-policy")
}

// TestSecurityAppFirewallHelpExecution tests security app-firewall command help.
func TestSecurityAppFirewallHelpExecution(t *testing.T) {
	output, err := executeCommand("security", "app-firewall", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Web Application Firewall")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
}

// TestCertHelpExecution tests cert command help.
func TestCertHelpExecution(t *testing.T) {
	output, err := executeCommand("cert", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage TLS certificates")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "upload")
	assert.Contains(t, output, "delete")
}

// TestCertUploadMissingArg tests cert upload without name.
func TestCertUploadMissingArg(t *testing.T) {
	_, err := executeCommand("cert", "upload")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

// TestDNSHelpExecution tests dns command help.
func TestDNSHelpExecution(t *testing.T) {
	output, err := executeCommand("dns", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage DNS resources")
	assert.Contains(t, output, "zone")
}

// TestDNSZoneHelpExecution tests dns zone command help.
func TestDNSZoneHelpExecution(t *testing.T) {
	output, err := executeCommand("dns", "zone", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage DNS zones")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
}

// TestMonitorHelpExecution tests monitor command help.
func TestMonitorHelpExecution(t *testing.T) {
	output, err := executeCommand("monitor", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage monitoring and observability resources")
	assert.Contains(t, output, "alert-policy")
	assert.Contains(t, output, "synthetic")
}

// TestMonitorAlertPolicyHelpExecution tests monitor alert-policy command help.
func TestMonitorAlertPolicyHelpExecution(t *testing.T) {
	output, err := executeCommand("monitor", "alert-policy", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage alert policies")
	assert.Contains(t, output, "list")
	assert.Contains(t, output, "get")
}

// TestConfigHelpExecution tests config command help.
func TestConfigHelpExecution(t *testing.T) {
	output, err := executeCommand("config", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage f5xcctl CLI configuration settings")
	assert.Contains(t, output, "get")
	assert.Contains(t, output, "set")
	assert.Contains(t, output, "list")
}

// TestAuthHelpExecution tests auth command help.
func TestAuthHelpExecution(t *testing.T) {
	output, err := executeCommand("auth", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage authentication")
	assert.Contains(t, output, "login")
	assert.Contains(t, output, "logout")
	assert.Contains(t, output, "status")
}

// TestConfigureHelpExecution tests configure command help.
func TestConfigureHelpExecution(t *testing.T) {
	output, err := executeCommand("configure", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Interactive configuration wizard")
}

// =============================================================================
// Flag Parsing Tests
// =============================================================================

// TestGlobalOutputFlag tests the global --output flag.
func TestGlobalOutputFlag(t *testing.T) {
	// Test that output flag is recognized
	output, err := executeCommand("--output", "json", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "output format")
}

// TestGlobalNamespaceFlag tests the global --namespace flag.
func TestGlobalNamespaceFlag(t *testing.T) {
	output, err := executeCommand("--namespace", "test-ns", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "namespace")
}

// TestGlobalDebugFlag tests the global --debug flag.
func TestGlobalDebugFlag(t *testing.T) {
	output, err := executeCommand("--debug", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "debug")
}

// TestGlobalProfileFlag tests the global --profile flag.
func TestGlobalProfileFlag(t *testing.T) {
	output, err := executeCommand("--profile", "test-profile", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "profile")
}

// =============================================================================
// Alias Tests
// =============================================================================

// TestNamespaceAlias tests ns alias for namespace.
func TestNamespaceAlias(t *testing.T) {
	output, err := executeCommand("ns", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud namespaces")
}

// TestLoadbalancerAlias tests loadbalancer alias for lb.
func TestLoadbalancerAlias(t *testing.T) {
	output, err := executeCommand("loadbalancer", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud load balancers")
}

// TestCertificateAlias tests certificate alias for cert.
func TestCertificateAlias(t *testing.T) {
	output, err := executeCommand("certificate", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage TLS certificates")
}

// TestSecAlias tests sec alias for security.
func TestSecAlias(t *testing.T) {
	output, err := executeCommand("sec", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage F5 Distributed Cloud security resources")
}

// TestMonAlias tests mon alias for monitor.
func TestMonAlias(t *testing.T) {
	output, err := executeCommand("mon", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage monitoring and observability resources")
}

// TestWafAlias tests waf alias for app-firewall.
func TestWafAlias(t *testing.T) {
	output, err := executeCommand("security", "waf", "--help")
	assert.NoError(t, err)
	assert.Contains(t, output, "Manage Web Application Firewalls")
}

// =============================================================================
// Unknown Command Tests
// =============================================================================

// TestUnknownCommand tests behavior with unknown command.
func TestUnknownCommand(t *testing.T) {
	_, err := executeCommand("unknowncommand")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestUnknownSubcommand tests behavior with unknown subcommand.
func TestUnknownSubcommand(t *testing.T) {
	// Subcommands that don't exist trigger the parent command's Run
	// The namespace command requires a subcommand, so this should error
	output, err := executeCommand("namespace", "unknownsub")
	// This may error or may show help depending on cobra config
	if err != nil {
		assert.Contains(t, err.Error(), "unknown command")
	} else {
		// If no error, the output should mention available commands
		assert.Contains(t, output, "Available Commands")
	}
}

// =============================================================================
// Command Structure Validation Tests
// =============================================================================

// TestAllCommandsHaveShortDescription verifies all commands have short descriptions.
func TestAllCommandsHaveShortDescription(t *testing.T) {
	checkCommandShort(t, rootCmd, "root")
}

func checkCommandShort(t *testing.T, cmd *cobra.Command, path string) {
	// Skip root command itself
	if cmd != rootCmd {
		assert.NotEmpty(t, cmd.Short, "Command %s should have a short description", path)
	}

	for _, sub := range cmd.Commands() {
		checkCommandShort(t, sub, path+"/"+sub.Name())
	}
}

// TestAllCommandsHaveUse verifies all commands have Use defined.
func TestAllCommandsHaveUse(t *testing.T) {
	checkCommandUse(t, rootCmd, "root")
}

func checkCommandUse(t *testing.T, cmd *cobra.Command, path string) {
	assert.NotEmpty(t, cmd.Use, "Command %s should have Use defined", path)

	for _, sub := range cmd.Commands() {
		checkCommandUse(t, sub, path+"/"+sub.Name())
	}
}

// TestCommandTreeStructure validates the complete command tree.
func TestCommandTreeStructure(t *testing.T) {
	// Define expected command tree structure
	expectedTree := map[string][]string{
		"f5xcctl": {
			"version",
			"configure",
			"config",
			"auth",
			"namespace",
			"lb",
			"origin",
			"security",
			"cert",
			"dns",
			"monitor",
		},
		"namespace": {"list", "get <name>", "create <name>", "delete <name>"},
		"config":    {"get <key>", "set <key> <value>", "list"},
		"auth":      {"login", "logout", "status"},
	}

	// Verify root level commands
	rootSubs := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		rootSubs[cmd.Use] = true
	}

	for _, expected := range expectedTree["f5xcctl"] {
		assert.True(t, rootSubs[expected], "Root command should have %q subcommand", expected)
	}

	// Verify namespace subcommands
	nsSubs := make(map[string]bool)
	for _, cmd := range namespaceCmd.Commands() {
		nsSubs[cmd.Use] = true
	}

	for _, expected := range expectedTree["namespace"] {
		assert.True(t, nsSubs[expected], "Namespace command should have %q subcommand", expected)
	}
}

// TestTotalCommandCount verifies expected number of commands.
func TestTotalCommandCount(t *testing.T) {
	count := countCommands(rootCmd)
	// Root + version + configure + config(3) + auth(3) + namespace(4) +
	// lb(http:5, tcp:5) + origin(pool:5) + security(af:5, sp:5) +
	// cert(4) + dns(zone:5) + monitor(alert:5, synth:5)
	// This is approximate - just ensure we have a reasonable number
	assert.Greater(t, count, 30, "Should have at least 30 commands total")
	t.Logf("Total command count: %d", count)
}

func countCommands(cmd *cobra.Command) int {
	count := 1 // Count this command
	for _, sub := range cmd.Commands() {
		count += countCommands(sub)
	}
	return count
}
