package nginx

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Tondeptrai23/claude-worktree-skills/pkg/config"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/platform"
	"github.com/Tondeptrai23/claude-worktree-skills/pkg/slot"
)

// Generate creates nginx configs for all active slots and the main nginx.conf.
func Generate(cfg *config.Config, rootDir string) error {
	nginxDir := cfg.FindNginxDir(rootDir)
	confDir := filepath.Join(nginxDir, "conf.d")
	os.MkdirAll(confDir, 0755)

	// Clean old slot configs
	matches, _ := filepath.Glob(filepath.Join(confDir, "slot-*.conf"))
	for _, m := range matches {
		os.Remove(m)
	}

	proxyHost := platform.ProxyHost()
	worktreesDir := config.WorktreesDir(rootDir)
	slots, _ := slot.DiscoverSlots(worktreesDir)

	for _, meta := range slots {
		confPath := filepath.Join(confDir, fmt.Sprintf("slot-%d.conf", meta.Slot))
		content := generateSlotConfig(meta, cfg, proxyHost, rootDir)
		if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", confPath, err)
		}
		fmt.Printf("[OK] Generated %s\n", confPath)
	}

	if len(slots) == 0 {
		fmt.Println("[*] No active slots — nginx conf.d/ cleared")
	} else {
		fmt.Printf("[OK] Generated configs for %d slot(s)\n", len(slots))
	}

	return nil
}

func generateSlotConfig(meta *slot.SlotMeta, cfg *config.Config, proxyHost, rootDir string) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("# Feature slot %d: %s\n", meta.Slot, meta.FeatureName))
	buf.WriteString(fmt.Sprintf("# Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	worktreesDir := config.WorktreesDir(rootDir)

	for svcName, svcMeta := range meta.Services {
		svc, ok := cfg.Services[svcName]
		if !ok || !svc.Expose {
			continue
		}

		// Check worktree directory exists
		repoDir := filepath.Join(worktreesDir, fmt.Sprintf("slot-%d", meta.Slot), svcMeta.RepoKey)
		if _, err := os.Stat(repoDir); os.IsNotExist(err) {
			continue
		}

		subdomain := resolveSubdomain(svcName, meta.Slot, meta.FeatureName, cfg)
		if subdomain == "" {
			continue
		}

		writeServerBlock(&buf, svcName, subdomain, proxyHost, svcMeta.Port, cfg.Nginx.Port)
	}

	return buf.String()
}

func writeServerBlock(buf *bytes.Buffer, name, subdomain, proxyHost string, port, listenPort int) {
	fmt.Fprintf(buf, "# %s\n", name)
	fmt.Fprintf(buf, "server {\n")
	fmt.Fprintf(buf, "    listen %d;\n", listenPort)
	fmt.Fprintf(buf, "    server_name %s;\n\n", subdomain)
	fmt.Fprintf(buf, "    client_max_body_size 100M;\n\n")
	fmt.Fprintf(buf, "    location / {\n")
	fmt.Fprintf(buf, "        proxy_pass http://%s:%d;\n", proxyHost, port)
	fmt.Fprintf(buf, "        proxy_http_version 1.1;\n")
	fmt.Fprintf(buf, "        proxy_set_header Upgrade $http_upgrade;\n")
	fmt.Fprintf(buf, "        proxy_set_header Connection \"upgrade\";\n")
	fmt.Fprintf(buf, "        proxy_set_header Host $host;\n")
	fmt.Fprintf(buf, "        proxy_set_header X-Real-IP $remote_addr;\n")
	fmt.Fprintf(buf, "        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	fmt.Fprintf(buf, "        proxy_read_timeout 300s;\n")
	fmt.Fprintf(buf, "    }\n")
	fmt.Fprintf(buf, "}\n\n")
}

func resolveSubdomain(svcName string, slot int, featureName string, cfg *config.Config) string {
	pattern := ""
	if sub, ok := cfg.Nginx.Subdomains[svcName]; ok {
		pattern = sub
	} else if cfg.Nginx.SubdomainPattern != "" {
		pattern = cfg.Nginx.SubdomainPattern
	}
	if pattern == "" {
		return ""
	}
	result := pattern
	result = replaceAll(result, "{slot}", fmt.Sprintf("%d", slot))
	result = replaceAll(result, "{svc}", svcName)
	result = replaceAll(result, "{name}", featureName)
	return result
}

func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx < 0 {
			return s
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// EnsureRunning starts the nginx container if not already running.
// If the configured port is occupied, it increments until a free port is found.
func EnsureRunning(cfg *config.Config, rootDir string) error {
	if !cfg.Nginx.Enabled {
		return nil
	}

	// Check if already running
	out, _ := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	if bytes.Contains(out, []byte("feature-router")) {
		return nil
	}

	// Find an available port, starting from the configured one
	port := cfg.Nginx.Port
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		if isPortAvailable(port) {
			break
		}
		fmt.Printf("[!] Port %d is occupied, trying %d\n", port, port+1)
		port++
	}

	if !isPortAvailable(port) {
		return fmt.Errorf("no available port found (tried %d–%d)", cfg.Nginx.Port, port)
	}

	cfg.Nginx.Port = port

	// Patch the listen port in the existing nginx.conf
	nginxDir := cfg.FindNginxDir(rootDir)
	if err := patchListenPort(nginxDir, port); err != nil {
		return err
	}

	// Generate slot configs with the resolved port
	if err := Generate(cfg, rootDir); err != nil {
		return err
	}

	// Find compose file
	composeFile := filepath.Join(nginxDir, "docker-compose.nginx.yml")
	if cfg.Nginx.ComposeFile != "" {
		composeFile = filepath.Join(rootDir, cfg.Nginx.ComposeFile)
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("nginx compose file not found: %s", composeFile)
	}

	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("starting nginx: %w\n%s", err, out)
	}

	fmt.Printf("[OK] Nginx running on port %d\n", port)
	return nil
}

var listenRe = regexp.MustCompile(`listen\s+\d+`)

// patchListenPort rewrites all "listen <port>" directives in nginx.conf to use the given port.
func patchListenPort(nginxDir string, port int) error {
	confPath := filepath.Join(nginxDir, "nginx.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", confPath, err)
	}
	patched := listenRe.ReplaceAll(data, []byte(fmt.Sprintf("listen %d", port)))
	if err := os.WriteFile(confPath, patched, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", confPath, err)
	}
	return nil
}

// isPortAvailable checks whether a TCP port is free to bind.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// Reload sends a reload signal to the running nginx container.
func Reload() {
	out, _ := exec.Command("docker", "ps", "--format", "{{.Names}}").Output()
	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		name := string(bytes.TrimSpace(line))
		if name != "" && bytes.Contains(line, []byte("feature-router")) {
			exec.Command("docker", "exec", name, "nginx", "-s", "reload").Run()
			return
		}
	}
}
