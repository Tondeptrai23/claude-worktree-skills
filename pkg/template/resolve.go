package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"claude-worktree-skill/pkg/config"
)

var tmplRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// BuildVars constructs the global variable map for a given slot.
// Service-specific vars (self.port, self.url) are NOT included — use Resolve() instead.
func BuildVars(slot int, featureName string, cfg *config.Config) map[string]string {
	vars := map[string]string{
		"slot":         strconv.Itoa(slot),
		"name":         featureName,
		"project_name": cfg.ProjectName,
	}

	// Per-service ports and URLs
	for name, svc := range cfg.Services {
		port := svc.Port(slot, cfg.PortOffset)
		vars[name+".port"] = strconv.Itoa(port)
		vars[name+".url"] = resolveURL(name, slot, featureName, cfg)
	}

	// Database vars
	dbPort := cfg.Database.Port
	dbHost := cfg.Database.Host

	if cfg.Database.Isolation == "database" {
		// Per-slot container: connect to localhost on computed port
		dbPort = cfg.Database.PortBase + slot
		dbHost = "localhost"
	}

	vars["db.host"] = dbHost
	vars["db.port"] = strconv.Itoa(dbPort)
	vars["db.name"] = cfg.Database.Name
	vars["db.user"] = cfg.Database.User
	vars["db.password"] = cfg.Database.Password
	if cfg.Database.SchemaPrefix != "" {
		vars["db.schema"] = fmt.Sprintf("%s%d", cfg.Database.SchemaPrefix, slot)
	}

	return vars
}

// Resolve resolves all {{var}} templates in a string for a specific service.
func Resolve(tmpl string, svcName string, slot int, featureName string, cfg *config.Config) string {
	vars := BuildVars(slot, featureName, cfg)

	// Add self.* for this service
	if svc, ok := cfg.Services[svcName]; ok {
		port := svc.Port(slot, cfg.PortOffset)
		vars["self.port"] = strconv.Itoa(port)
		vars["self.url"] = resolveURL(svcName, slot, featureName, cfg)
	}

	return tmplRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := strings.TrimSpace(match[2 : len(match)-2])
		if val, ok := vars[key]; ok {
			return val
		}
		return match // leave unresolved
	})
}

// resolveURL returns the URL for a service in a slot.
func resolveURL(svcName string, slot int, featureName string, cfg *config.Config) string {
	svc, ok := cfg.Services[svcName]
	if !ok {
		return ""
	}

	port := svc.Port(slot, cfg.PortOffset)

	// If nginx enabled and service is exposed, use subdomain
	if cfg.Nginx.Enabled && svc.Expose {
		subdomain := resolveSubdomain(svcName, slot, featureName, cfg)
		if subdomain != "" {
			return "http://" + subdomain
		}
	}

	return fmt.Sprintf("http://localhost:%d", port)
}

// resolveSubdomain returns the nginx subdomain for a service.
func resolveSubdomain(svcName string, slot int, featureName string, cfg *config.Config) string {
	pattern := ""

	// Check per-service override first
	if sub, ok := cfg.Nginx.Subdomains[svcName]; ok {
		pattern = sub
	} else if cfg.Nginx.SubdomainPattern != "" {
		pattern = cfg.Nginx.SubdomainPattern
	} else {
		return ""
	}

	pattern = strings.ReplaceAll(pattern, "{slot}", strconv.Itoa(slot))
	pattern = strings.ReplaceAll(pattern, "{svc}", svcName)
	pattern = strings.ReplaceAll(pattern, "{name}", featureName)
	return pattern
}
