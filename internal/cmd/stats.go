package cmd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/f5/f5xcctl/internal/output"
)

var statsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"statistics", "metrics"},
	Short:   "View statistics and metrics",
	Long: `View statistics and metrics for F5 Distributed Cloud resources.

Statistics provide real-time and historical insights into:
  - Load balancer performance (requests, latency, errors)
  - Site health and infrastructure metrics
  - Security events and threat analytics
  - API endpoint usage patterns

Examples:
  # View HTTP load balancer statistics
  f5xcctl stats lb http my-lb -n my-namespace

  # View site status and metrics
  f5xcctl stats site my-site

  # View security event summary
  f5xcctl stats security my-lb -n my-namespace`,
}

// Load Balancer stats commands.
var statsLBCmd = &cobra.Command{
	Use:   "lb",
	Short: "Load balancer statistics",
	Long:  `View statistics for HTTP, TCP, and UDP load balancers.`,
}

var statsLBHTTPCmd = &cobra.Command{
	Use:   "http <name>",
	Short: "HTTP load balancer statistics",
	Long: `View statistics for an HTTP load balancer.

Shows metrics including:
  - Request rate and throughput
  - Response codes distribution
  - Latency percentiles
  - Error rates
  - Top API endpoints`,
	Args: cobra.ExactArgs(1),
	RunE: runStatsLBHTTP,
}

// Site stats commands.
var statsSiteCmd = &cobra.Command{
	Use:   "site <name>",
	Short: "Site statistics and status",
	Long: `View status and metrics for a site.

Shows metrics including:
  - Site health status
  - CPU, memory, disk utilization
  - Network throughput and drops
  - Pod/container metrics`,
	Args: cobra.ExactArgs(1),
	RunE: runStatsSite,
}

// Security stats commands.
var statsSecurityCmd = &cobra.Command{
	Use:   "security <lb-name>",
	Short: "Security event statistics",
	Long: `View security statistics for a load balancer.

Shows metrics including:
  - WAF events and blocked requests
  - Bot detection statistics
  - API security violations
  - Threat analytics summary`,
	Args: cobra.ExactArgs(1),
	RunE: runStatsSecurity,
}

// API stats commands.
var statsAPICmd = &cobra.Command{
	Use:   "api <lb-name>",
	Short: "API endpoint statistics",
	Long: `View API endpoint statistics for a load balancer.

Shows metrics including:
  - Top active endpoints
  - Response code distribution
  - Sensitive data exposure
  - API inventory summary`,
	Args: cobra.ExactArgs(1),
	RunE: runStatsAPI,
}

var (
	statsTimeRange string
	statsTopN      int
	statsDetailed  bool
)

func init() {
	// Common flags for stats commands
	statsLBHTTPCmd.Flags().StringVar(&statsTimeRange, "time-range", "1h", "Time range for metrics (5m, 15m, 1h, 6h, 24h, 7d)")
	statsLBHTTPCmd.Flags().IntVar(&statsTopN, "top", 10, "Number of top items to show")
	statsLBHTTPCmd.Flags().BoolVar(&statsDetailed, "detailed", false, "Show detailed metrics")

	statsSiteCmd.Flags().StringVar(&statsTimeRange, "time-range", "1h", "Time range for metrics")
	statsSiteCmd.Flags().BoolVar(&statsDetailed, "detailed", false, "Show detailed metrics")

	statsSecurityCmd.Flags().StringVar(&statsTimeRange, "time-range", "1h", "Time range for metrics")
	statsSecurityCmd.Flags().IntVar(&statsTopN, "top", 10, "Number of top items to show")

	statsAPICmd.Flags().StringVar(&statsTimeRange, "time-range", "1h", "Time range for metrics")
	statsAPICmd.Flags().IntVar(&statsTopN, "top", 10, "Number of top items to show")

	// Build command tree
	statsLBCmd.AddCommand(statsLBHTTPCmd)

	statsCmd.AddCommand(statsLBCmd)
	statsCmd.AddCommand(statsSiteCmd)
	statsCmd.AddCommand(statsSecurityCmd)
	statsCmd.AddCommand(statsAPICmd)

	rootCmd.AddCommand(statsCmd)
}

// LBStatsResponse represents load balancer statistics response.
type LBStatsResponse struct {
	RequestsTotal  int64            `json:"requests_total,omitempty"`
	RequestsPerSec float64          `json:"requests_per_sec,omitempty"`
	ErrorRate      float64          `json:"error_rate,omitempty"`
	LatencyP50     float64          `json:"latency_p50_ms,omitempty"`
	LatencyP95     float64          `json:"latency_p95_ms,omitempty"`
	LatencyP99     float64          `json:"latency_p99_ms,omitempty"`
	ThroughputIn   int64            `json:"throughput_in_bytes,omitempty"`
	ThroughputOut  int64            `json:"throughput_out_bytes,omitempty"`
	ResponseCodes  map[string]int64 `json:"response_codes,omitempty"`
	TopEndpoints   []EndpointStats  `json:"top_endpoints,omitempty"`
}

// EndpointStats represents API endpoint statistics.
type EndpointStats struct {
	Path         string  `json:"path"`
	Method       string  `json:"method"`
	Requests     int64   `json:"requests"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// SiteStatusResponse represents site status response.
type SiteStatusResponse struct {
	Name            string           `json:"name"`
	State           string           `json:"state"`
	SiteType        string           `json:"site_type,omitempty"`
	SoftwareVersion string           `json:"software_version,omitempty"`
	Coordinates     *SiteCoordinates `json:"coordinates,omitempty"`
	Conditions      []SiteCondition  `json:"conditions,omitempty"`
	Metrics         *SiteMetrics     `json:"metrics,omitempty"`
}

// SiteCoordinates represents geographic coordinates of a site.
type SiteCoordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// SiteCondition represents a condition status of a site.
type SiteCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// SiteMetrics represents resource usage metrics of a site.
type SiteMetrics struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent,omitempty"`
	MemoryUsagePercent float64 `json:"memory_usage_percent,omitempty"`
	DiskUsagePercent   float64 `json:"disk_usage_percent,omitempty"`
	NetworkInBytes     int64   `json:"network_in_bytes,omitempty"`
	NetworkOutBytes    int64   `json:"network_out_bytes,omitempty"`
}

// SecurityStatsResponse represents security statistics.
type SecurityStatsResponse struct {
	TotalEvents     int64            `json:"total_events"`
	BlockedRequests int64            `json:"blocked_requests"`
	WAFEvents       int64            `json:"waf_events"`
	BotDetections   int64            `json:"bot_detections"`
	DDoSMitigations int64            `json:"ddos_mitigations"`
	TopAttackTypes  []AttackTypeStat `json:"top_attack_types,omitempty"`
	TopSourceIPs    []SourceIPStat   `json:"top_source_ips,omitempty"`
}

// AttackTypeStat represents statistics for an attack type.
type AttackTypeStat struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

// SourceIPStat represents statistics for a source IP address.
type SourceIPStat struct {
	IP      string `json:"ip"`
	Country string `json:"country,omitempty"`
	Count   int64  `json:"count"`
}

// APIStatsResponse represents API statistics.
type APIStatsResponse struct {
	TotalEndpoints     int              `json:"total_endpoints"`
	DiscoveredAPIs     int              `json:"discovered_apis"`
	ShadowAPIs         int              `json:"shadow_apis"`
	SensitiveDataFound int              `json:"sensitive_data_found"`
	TopActiveAPIs      []EndpointStats  `json:"top_active_apis,omitempty"`
	ResponseCodeDist   map[string]int64 `json:"response_code_distribution,omitempty"`
}

func runStatsLBHTTP(cmd *cobra.Command, args []string) error {
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

	// Try to get API endpoint stats
	query := url.Values{}
	query.Set("time_range", statsTimeRange)

	// First, get basic stats from the API endpoints summary
	path := fmt.Sprintf("/api/ml/data/namespaces/%s/http_loadbalancers/%s/api_endpoints/stats", ns, name)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to get load balancer stats: %w", err)
	}

	if err := resp.Error(); err != nil {
		// If ML data endpoint fails, show basic configuration info
		output.Warningf("Detailed metrics not available. Showing configuration status.")
		return showLBConfigStatus(ctx, client, ns, name)
	}

	// Parse and display stats
	var stats LBStatsResponse
	if err := resp.DecodeJSON(&stats); err != nil {
		return fmt.Errorf("failed to decode stats response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, stats)
	}

	// Table output
	fmt.Printf("\n=== HTTP Load Balancer Statistics: %s ===\n\n", name)
	fmt.Printf("Namespace: %s\n", ns)
	fmt.Printf("Time Range: %s\n\n", statsTimeRange)

	fmt.Println("Traffic Overview:")
	fmt.Printf("  Total Requests:    %d\n", stats.RequestsTotal)
	fmt.Printf("  Requests/sec:      %.2f\n", stats.RequestsPerSec)
	fmt.Printf("  Error Rate:        %.2f%%\n", stats.ErrorRate*100)
	fmt.Println()

	if stats.LatencyP50 > 0 || stats.LatencyP95 > 0 {
		fmt.Println("Latency:")
		fmt.Printf("  P50:               %.2f ms\n", stats.LatencyP50)
		fmt.Printf("  P95:               %.2f ms\n", stats.LatencyP95)
		fmt.Printf("  P99:               %.2f ms\n", stats.LatencyP99)
		fmt.Println()
	}

	if len(stats.ResponseCodes) > 0 {
		fmt.Println("Response Codes:")
		for code, count := range stats.ResponseCodes {
			fmt.Printf("  %s: %d\n", code, count)
		}
		fmt.Println()
	}

	if len(stats.TopEndpoints) > 0 {
		fmt.Println("Top Endpoints:")
		for i, ep := range stats.TopEndpoints {
			if i >= statsTopN {
				break
			}
			fmt.Printf("  %s %s - %d requests (%.2f%% errors, %.2fms avg)\n",
				ep.Method, ep.Path, ep.Requests, ep.ErrorRate*100, ep.AvgLatencyMs)
		}
	}

	return nil
}

//nolint:unparam // ctx and client kept for API consistency with other status functions
func showLBConfigStatus(ctx context.Context, client interface{}, ns, name string) error {
	// Show basic LB configuration when detailed metrics aren't available
	fmt.Printf("\n=== HTTP Load Balancer: %s ===\n\n", name)
	fmt.Printf("Namespace: %s\n", ns)
	fmt.Println("Status: Configuration view (detailed metrics require ML data access)")
	fmt.Println("\nUse 'f5xcctl lb http get " + name + " -n " + ns + "' for full configuration details")
	return nil
}

func runStatsSite(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get site status
	path := fmt.Sprintf("/api/data/namespaces/system/site/%s/status", name)
	resp, err := client.Get(ctx, path, nil)
	if err != nil {
		return fmt.Errorf("failed to get site status: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	var status SiteStatusResponse
	if err := resp.DecodeJSON(&status); err != nil {
		return fmt.Errorf("failed to decode status response: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, status)
	}

	// Table output
	fmt.Printf("\n=== Site Status: %s ===\n\n", name)
	fmt.Printf("State:           %s\n", status.State)
	if status.SiteType != "" {
		fmt.Printf("Type:            %s\n", status.SiteType)
	}
	if status.SoftwareVersion != "" {
		fmt.Printf("Version:         %s\n", status.SoftwareVersion)
	}
	fmt.Println()

	if len(status.Conditions) > 0 {
		fmt.Println("Conditions:")
		for _, cond := range status.Conditions {
			statusIcon := "?"
			switch cond.Status {
			case "True", "true", "READY", "Ready":
				statusIcon = "OK"
			case "False", "false":
				statusIcon = "FAIL"
			}
			fmt.Printf("  %-20s %s\n", cond.Type+":", statusIcon)
			if cond.Message != "" {
				fmt.Printf("    %s\n", cond.Message)
			}
		}
		fmt.Println()
	}

	if status.Metrics != nil {
		fmt.Println("Resource Utilization:")
		if status.Metrics.CPUUsagePercent > 0 {
			fmt.Printf("  CPU:           %.1f%%\n", status.Metrics.CPUUsagePercent)
		}
		if status.Metrics.MemoryUsagePercent > 0 {
			fmt.Printf("  Memory:        %.1f%%\n", status.Metrics.MemoryUsagePercent)
		}
		if status.Metrics.DiskUsagePercent > 0 {
			fmt.Printf("  Disk:          %.1f%%\n", status.Metrics.DiskUsagePercent)
		}
		if status.Metrics.NetworkInBytes > 0 || status.Metrics.NetworkOutBytes > 0 {
			fmt.Printf("  Network In:    %s\n", formatBytes(status.Metrics.NetworkInBytes))
			fmt.Printf("  Network Out:   %s\n", formatBytes(status.Metrics.NetworkOutBytes))
		}
	}

	return nil
}

func runStatsSecurity(cmd *cobra.Command, args []string) error {
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

	// Get security events summary
	query := url.Values{}
	query.Set("time_range", statsTimeRange)

	// Try to get security metrics
	path := fmt.Sprintf("/api/data/namespaces/%s/http_loadbalancers/%s/security_events/summary", ns, name)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		output.Warningf("Security metrics endpoint not available. Security statistics require additional API access.")
		return nil
	}

	if err := resp.Error(); err != nil {
		output.Warningf("Unable to retrieve security statistics: %v", err)
		fmt.Println("\nTo view security events, use the F5XC Console or ensure proper API permissions.")
		return nil
	}

	var stats SecurityStatsResponse
	if err := resp.DecodeJSON(&stats); err != nil {
		return fmt.Errorf("failed to decode security stats: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, stats)
	}

	// Table output
	fmt.Printf("\n=== Security Statistics: %s ===\n\n", name)
	fmt.Printf("Namespace: %s\n", ns)
	fmt.Printf("Time Range: %s\n\n", statsTimeRange)

	fmt.Println("Event Summary:")
	fmt.Printf("  Total Events:      %d\n", stats.TotalEvents)
	fmt.Printf("  Blocked Requests:  %d\n", stats.BlockedRequests)
	fmt.Printf("  WAF Events:        %d\n", stats.WAFEvents)
	fmt.Printf("  Bot Detections:    %d\n", stats.BotDetections)
	fmt.Printf("  DDoS Mitigations:  %d\n", stats.DDoSMitigations)
	fmt.Println()

	if len(stats.TopAttackTypes) > 0 {
		fmt.Println("Top Attack Types:")
		for i, at := range stats.TopAttackTypes {
			if i >= statsTopN {
				break
			}
			fmt.Printf("  %-30s %d\n", at.Type, at.Count)
		}
		fmt.Println()
	}

	if len(stats.TopSourceIPs) > 0 {
		fmt.Println("Top Source IPs:")
		for i, ip := range stats.TopSourceIPs {
			if i >= statsTopN {
				break
			}
			country := ""
			if ip.Country != "" {
				country = " (" + ip.Country + ")"
			}
			fmt.Printf("  %-20s %s%d requests\n", ip.IP+country, "", ip.Count)
		}
	}

	return nil
}

func runStatsAPI(cmd *cobra.Command, args []string) error {
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

	// Get API endpoint stats
	query := url.Values{}
	query.Set("time_range", statsTimeRange)

	path := fmt.Sprintf("/api/ml/data/namespaces/%s/http_loadbalancers/%s/api_endpoints/stats", ns, name)
	resp, err := client.Get(ctx, path, query)
	if err != nil {
		return fmt.Errorf("failed to get API stats: %w", err)
	}

	if err := resp.Error(); err != nil {
		output.Warningf("API statistics not available. API discovery may need to be enabled on this load balancer.")
		return nil
	}

	var stats APIStatsResponse
	if err := resp.DecodeJSON(&stats); err != nil {
		return fmt.Errorf("failed to decode API stats: %w", err)
	}

	if outputFmt == "json" || outputFmt == "yaml" {
		return output.Print(outputFmt, stats)
	}

	// Table output
	fmt.Printf("\n=== API Statistics: %s ===\n\n", name)
	fmt.Printf("Namespace: %s\n", ns)
	fmt.Printf("Time Range: %s\n\n", statsTimeRange)

	fmt.Println("API Inventory:")
	fmt.Printf("  Total Endpoints:       %d\n", stats.TotalEndpoints)
	fmt.Printf("  Discovered APIs:       %d\n", stats.DiscoveredAPIs)
	fmt.Printf("  Shadow APIs:           %d\n", stats.ShadowAPIs)
	fmt.Printf("  Sensitive Data Found:  %d\n", stats.SensitiveDataFound)
	fmt.Println()

	if len(stats.ResponseCodeDist) > 0 {
		fmt.Println("Response Code Distribution:")
		for code, count := range stats.ResponseCodeDist {
			fmt.Printf("  %s: %d\n", code, count)
		}
		fmt.Println()
	}

	if len(stats.TopActiveAPIs) > 0 {
		fmt.Println("Top Active APIs:")
		for i, api := range stats.TopActiveAPIs {
			if i >= statsTopN {
				break
			}
			fmt.Printf("  %s %s - %d requests\n", api.Method, api.Path, api.Requests)
		}
	}

	return nil
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
