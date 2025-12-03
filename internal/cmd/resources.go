package cmd

// ResourceType defines a F5XC resource type with its API path and aliases.
type ResourceType struct {
	// Name is the canonical resource name (e.g., "http_loadbalancer")
	Name string

	// Plural is the plural form used in API paths (e.g., "http_loadbalancers")
	Plural string

	// Short is the short alias (e.g., "httplb")
	Short string

	// Aliases are additional aliases for the resource
	Aliases []string

	// APIPath is the base API path pattern (e.g., "/api/config/namespaces/{namespace}/http_loadbalancers")
	APIPath string

	// Kind is the resource kind for YAML/JSON (e.g., "http_loadbalancer")
	Kind string

	// Group is the resource group for organization (e.g., "load-balancing")
	Group string

	// Namespaced indicates if the resource is namespace-scoped
	Namespaced bool

	// Description is a short description of the resource
	Description string

	// SupportedVerbs lists the verbs supported by this resource
	SupportedVerbs []string
}

// Standard verbs supported by most resources.
var StandardVerbs = []string{"get", "list", "create", "delete", "replace", "apply"}

// AllVerbs includes all possible verbs.
var AllVerbs = []string{"get", "list", "create", "delete", "replace", "apply", "patch", "label", "annotate", "describe"}

// ResourceRegistry holds all known F5XC resource types.
var ResourceRegistry = map[string]*ResourceType{
	// Namespaces (special - lives in system namespace)
	"namespace": {
		Name:           "namespace",
		Plural:         "namespaces",
		Short:          "ns",
		Aliases:        []string{},
		APIPath:        "/api/web/namespaces",
		Kind:           "namespace",
		Group:          "core",
		Namespaced:     false,
		Description:    "Namespace for organizing resources",
		SupportedVerbs: StandardVerbs,
	},

	// Load Balancers
	"http_loadbalancer": {
		Name:           "http_loadbalancer",
		Plural:         "http_loadbalancers",
		Short:          "httplb",
		Aliases:        []string{"http-lb", "http-loadbalancer", "hlb"},
		APIPath:        "/api/config/namespaces/{namespace}/http_loadbalancers",
		Kind:           "http_loadbalancer",
		Group:          "load-balancing",
		Namespaced:     true,
		Description:    "HTTP Load Balancer for L7 traffic",
		SupportedVerbs: AllVerbs,
	},
	"tcp_loadbalancer": {
		Name:           "tcp_loadbalancer",
		Plural:         "tcp_loadbalancers",
		Short:          "tcplb",
		Aliases:        []string{"tcp-lb", "tcp-loadbalancer", "tlb"},
		APIPath:        "/api/config/namespaces/{namespace}/tcp_loadbalancers",
		Kind:           "tcp_loadbalancer",
		Group:          "load-balancing",
		Namespaced:     true,
		Description:    "TCP Load Balancer for L4 traffic",
		SupportedVerbs: AllVerbs,
	},
	"udp_loadbalancer": {
		Name:           "udp_loadbalancer",
		Plural:         "udp_loadbalancers",
		Short:          "udplb",
		Aliases:        []string{"udp-lb", "udp-loadbalancer", "ulb"},
		APIPath:        "/api/config/namespaces/{namespace}/udp_loadbalancers",
		Kind:           "udp_loadbalancer",
		Group:          "load-balancing",
		Namespaced:     true,
		Description:    "UDP Load Balancer for L4 UDP traffic",
		SupportedVerbs: AllVerbs,
	},

	// Origin Pools
	"origin_pool": {
		Name:           "origin_pool",
		Plural:         "origin_pools",
		Short:          "op",
		Aliases:        []string{"origin-pool", "pool", "originpool"},
		APIPath:        "/api/config/namespaces/{namespace}/origin_pools",
		Kind:           "origin_pool",
		Group:          "load-balancing",
		Namespaced:     true,
		Description:    "Origin pool for backend servers",
		SupportedVerbs: AllVerbs,
	},

	// Health Checks
	"healthcheck": {
		Name:           "healthcheck",
		Plural:         "healthchecks",
		Short:          "hc",
		Aliases:        []string{"health-check", "health_check"},
		APIPath:        "/api/config/namespaces/{namespace}/healthchecks",
		Kind:           "healthcheck",
		Group:          "load-balancing",
		Namespaced:     true,
		Description:    "Health check for origin pools",
		SupportedVerbs: AllVerbs,
	},

	// Security - WAF/Firewall
	"app_firewall": {
		Name:           "app_firewall",
		Plural:         "app_firewalls",
		Short:          "af",
		Aliases:        []string{"app-firewall", "waf", "firewall", "appfirewall"},
		APIPath:        "/api/config/namespaces/{namespace}/app_firewalls",
		Kind:           "app_firewall",
		Group:          "security",
		Namespaced:     true,
		Description:    "Application firewall (WAF) policy",
		SupportedVerbs: AllVerbs,
	},

	// Security - Service Policy
	"service_policy": {
		Name:           "service_policy",
		Plural:         "service_policys",
		Short:          "sp",
		Aliases:        []string{"service-policy", "servicepolicy", "svcpolicy"},
		APIPath:        "/api/config/namespaces/{namespace}/service_policys",
		Kind:           "service_policy",
		Group:          "security",
		Namespaced:     true,
		Description:    "Service policy for access control",
		SupportedVerbs: AllVerbs,
	},

	// Security - Rate Limiter
	"rate_limiter": {
		Name:           "rate_limiter",
		Plural:         "rate_limiters",
		Short:          "rl",
		Aliases:        []string{"rate-limiter", "ratelimiter", "ratelimit"},
		APIPath:        "/api/config/namespaces/{namespace}/rate_limiters",
		Kind:           "rate_limiter",
		Group:          "security",
		Namespaced:     true,
		Description:    "Rate limiting policy",
		SupportedVerbs: AllVerbs,
	},

	// Certificates
	"certificate": {
		Name:           "certificate",
		Plural:         "certificates",
		Short:          "cert",
		Aliases:        []string{"certs"},
		APIPath:        "/api/config/namespaces/{namespace}/certificates",
		Kind:           "certificate",
		Group:          "certificates",
		Namespaced:     true,
		Description:    "TLS certificate",
		SupportedVerbs: StandardVerbs,
	},

	// DNS
	"dns_zone": {
		Name:           "dns_zone",
		Plural:         "dns_zones",
		Short:          "dnsz",
		Aliases:        []string{"dns-zone", "dnszone", "zone"},
		APIPath:        "/api/config/namespaces/{namespace}/dns_zones",
		Kind:           "dns_zone",
		Group:          "dns",
		Namespaced:     true,
		Description:    "DNS zone configuration",
		SupportedVerbs: AllVerbs,
	},
	"dns_load_balancer": {
		Name:           "dns_load_balancer",
		Plural:         "dns_load_balancers",
		Short:          "dnslb",
		Aliases:        []string{"dns-lb", "dns-load-balancer", "gslb"},
		APIPath:        "/api/config/namespaces/{namespace}/dns_load_balancers",
		Kind:           "dns_load_balancer",
		Group:          "dns",
		Namespaced:     true,
		Description:    "DNS load balancer (GSLB)",
		SupportedVerbs: AllVerbs,
	},

	// Network
	"virtual_network": {
		Name:           "virtual_network",
		Plural:         "virtual_networks",
		Short:          "vnet",
		Aliases:        []string{"virtual-network", "virtualnetwork", "vn"},
		APIPath:        "/api/config/namespaces/{namespace}/virtual_networks",
		Kind:           "virtual_network",
		Group:          "networking",
		Namespaced:     true,
		Description:    "Virtual network configuration",
		SupportedVerbs: AllVerbs,
	},
	"network_policy": {
		Name:           "network_policy",
		Plural:         "network_policys",
		Short:          "netpol",
		Aliases:        []string{"network-policy", "networkpolicy", "np"},
		APIPath:        "/api/config/namespaces/{namespace}/network_policys",
		Kind:           "network_policy",
		Group:          "networking",
		Namespaced:     true,
		Description:    "Network policy for traffic control",
		SupportedVerbs: AllVerbs,
	},

	// Sites
	"site": {
		Name:           "site",
		Plural:         "sites",
		Short:          "",
		Aliases:        []string{},
		APIPath:        "/api/config/namespaces/system/sites",
		Kind:           "site",
		Group:          "infrastructure",
		Namespaced:     false, // Sites are in system namespace
		Description:    "Edge site or cloud site",
		SupportedVerbs: AllVerbs,
	},
	"virtual_site": {
		Name:           "virtual_site",
		Plural:         "virtual_sites",
		Short:          "vsite",
		Aliases:        []string{"virtual-site", "virtualsite", "vs"},
		APIPath:        "/api/config/namespaces/{namespace}/virtual_sites",
		Kind:           "virtual_site",
		Group:          "infrastructure",
		Namespaced:     true,
		Description:    "Virtual site for grouping sites",
		SupportedVerbs: AllVerbs,
	},

	// Monitoring
	"alert_policy": {
		Name:           "alert_policy",
		Plural:         "alert_policys",
		Short:          "alert",
		Aliases:        []string{"alert-policy", "alertpolicy", "ap"},
		APIPath:        "/api/config/namespaces/{namespace}/alert_policys",
		Kind:           "alert_policy",
		Group:          "monitoring",
		Namespaced:     true,
		Description:    "Alert policy for notifications",
		SupportedVerbs: AllVerbs,
	},
	"global_log_receiver": {
		Name:           "global_log_receiver",
		Plural:         "global_log_receivers",
		Short:          "glr",
		Aliases:        []string{"global-log-receiver", "log-receiver", "logreceiver"},
		APIPath:        "/api/config/namespaces/{namespace}/global_log_receivers",
		Kind:           "global_log_receiver",
		Group:          "monitoring",
		Namespaced:     true,
		Description:    "Global log receiver for log export",
		SupportedVerbs: AllVerbs,
	},

	// API Credentials
	"api_credential": {
		Name:           "api_credential",
		Plural:         "api_credentials",
		Short:          "apicred",
		Aliases:        []string{"api-credential", "apicredential", "cred"},
		APIPath:        "/api/web/namespaces/system/api_credentials",
		Kind:           "api_credential",
		Group:          "access",
		Namespaced:     false,
		Description:    "API credential for authentication",
		SupportedVerbs: StandardVerbs,
	},

	// Cloud Credentials
	"cloud_credentials": {
		Name:           "cloud_credentials",
		Plural:         "cloud_credentialss",
		Short:          "cloudcred",
		Aliases:        []string{"cloud-credentials", "cloudcredentials", "cc"},
		APIPath:        "/api/config/namespaces/{namespace}/cloud_credentialss",
		Kind:           "cloud_credentials",
		Group:          "infrastructure",
		Namespaced:     true,
		Description:    "Cloud provider credentials",
		SupportedVerbs: StandardVerbs,
	},
}

// ResourceAliasMap maps all aliases to canonical resource names.
var ResourceAliasMap = buildAliasMap()

func buildAliasMap() map[string]string {
	aliasMap := make(map[string]string)
	for name, rt := range ResourceRegistry {
		// Map canonical name
		aliasMap[name] = name
		// Map plural
		aliasMap[rt.Plural] = name
		// Map short form
		if rt.Short != "" {
			aliasMap[rt.Short] = name
		}
		// Map all aliases
		for _, alias := range rt.Aliases {
			aliasMap[alias] = name
		}
	}
	return aliasMap
}

// ResolveResourceType resolves a resource name/alias to its ResourceType.
func ResolveResourceType(name string) *ResourceType {
	canonical, ok := ResourceAliasMap[name]
	if !ok {
		return nil
	}
	return ResourceRegistry[canonical]
}

// GetResourceGroups returns resources organized by group.
func GetResourceGroups() map[string][]*ResourceType {
	groups := make(map[string][]*ResourceType)
	for _, rt := range ResourceRegistry {
		groups[rt.Group] = append(groups[rt.Group], rt)
	}
	return groups
}

// ListResourceTypes returns all resource type names for completion.
func ListResourceTypes() []string {
	var names []string
	for name := range ResourceRegistry {
		names = append(names, name)
	}
	return names
}

// GetAPIPath returns the API path for a resource, substituting namespace.
func (rt *ResourceType) GetAPIPath(namespace string) string {
	if !rt.Namespaced {
		return rt.APIPath
	}
	// Replace {namespace} placeholder
	path := rt.APIPath
	if namespace == "" {
		namespace = "default"
	}
	return replaceNamespace(path, namespace)
}

// GetItemPath returns the API path for a specific resource item.
func (rt *ResourceType) GetItemPath(namespace, name string) string {
	basePath := rt.GetAPIPath(namespace)
	return basePath + "/" + name
}

func replaceNamespace(path, namespace string) string {
	// Simple string replacement for {namespace}
	result := ""
	for i := 0; i < len(path); i++ {
		if i+11 <= len(path) && path[i:i+11] == "{namespace}" {
			result += namespace
			i += 10 // Skip rest of placeholder
		} else {
			result += string(path[i])
		}
	}
	return result
}
