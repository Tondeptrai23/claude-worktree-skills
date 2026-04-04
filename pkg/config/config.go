package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProjectName  string             `yaml:"project_name"`
	MaxSlots     int                `yaml:"max_slots"`
	PortOffset   int                `yaml:"port_offset"`
	GitTopology  string             `yaml:"git_topology"`
	DefaultMode  string             `yaml:"default_mode"`
	BranchPrefix string             `yaml:"branch_prefix"`
	PrivateFiles []string           `yaml:"private_files"`
	Nginx        NginxConfig        `yaml:"nginx"`
	Database     DatabaseConfig     `yaml:"database"`
	Services     map[string]Service `yaml:"services"`
}

type NginxConfig struct {
	Enabled          bool              `yaml:"enabled"`
	Port             int               `yaml:"port"`
	ConfDir          string            `yaml:"conf_dir"`
	ComposeFile      string            `yaml:"compose_file"`
	SubdomainPattern string            `yaml:"subdomain_pattern"`
	Subdomains       map[string]string `yaml:"subdomains"`
}

type DatabaseConfig struct {
	Isolation     string                     `yaml:"isolation"`
	Host          string                     `yaml:"host"`
	Port          int                        `yaml:"port"`
	Name          string                     `yaml:"name"`
	User          string                     `yaml:"user"`
	Password      string                     `yaml:"password"`
	SchemaPrefix  string                     `yaml:"schema_prefix"`
	Image         string                     `yaml:"image"`
	PortBase      int                        `yaml:"port_base"`
	ContainerPort int                        `yaml:"container_port"`
	Env           map[string]string          `yaml:"env"`
	Readiness     string                     `yaml:"readiness"`
	Setup         string                     `yaml:"setup"`
	Teardown      string                     `yaml:"teardown"`
	SeedScript    string                     `yaml:"seed_script"`
	Migrations    map[string]MigrationConfig `yaml:"migrations"`
}

type MigrationConfig struct {
	Tool        string `yaml:"tool"`
	Location    string `yaml:"location"`
	SchemaAware bool   `yaml:"schema_aware"`
	Run         string `yaml:"run"`
	Notes       string `yaml:"notes"`
}

type Service struct {
	Path         string            `yaml:"path"`
	Subdir       string            `yaml:"subdir,omitempty"`
	PortBase     int               `yaml:"port_base"`
	Expose       bool              `yaml:"expose"`
	EnvFile      string            `yaml:"env_file"`
	EnvSample    string            `yaml:"env_sample,omitempty"`
	PortEnv      string            `yaml:"port_env,omitempty"`
	Modes        map[string]Mode   `yaml:"modes"`
	EnvOverrides map[string]string `yaml:"env_overrides"`
}

type Mode struct {
	Start       string `yaml:"start,omitempty"`
	Install     string `yaml:"install,omitempty"`
	ComposeFile string `yaml:"compose_file,omitempty"`
	ServiceName string `yaml:"service_name,omitempty"`
}

// RepoKey returns the last path component (e.g., "./fe" → "fe").
func (s Service) RepoKey() string {
	return filepath.Base(s.Path)
}

// Port returns the port for this service in a given slot.
func (s Service) Port(slot, offset int) int {
	return s.PortBase + slot*offset
}

// WorkDir returns the working directory inside a slot worktree.
func (s Service) WorkDir(slotDir string) string {
	dir := filepath.Join(slotDir, s.RepoKey())
	if s.Subdir != "" {
		dir = filepath.Join(dir, s.Subdir)
	}
	return dir
}

// MainDir returns the working directory in the main checkout.
func (s Service) MainDir(rootDir string) string {
	dir := filepath.Join(rootDir, s.Path)
	if s.Subdir != "" {
		dir = filepath.Join(dir, s.Subdir)
	}
	return dir
}

// FindConfig searches for worktree.yml walking up from cwd.
func FindConfig() (string, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	for {
		// Check root-level worktree.yml
		candidate := filepath.Join(dir, "worktree.yml")
		if fileExists(candidate) {
			return candidate, dir, nil
		}

		// Check .claude/worktree/worktree.yml
		candidate = filepath.Join(dir, ".claude", "worktree", "worktree.yml")
		if fileExists(candidate) {
			return candidate, dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("worktree.yml not found (searched from %s upward)", dir)
}

// Load finds and parses worktree.yml, returning config and project root.
func Load() (*Config, string, error) {
	cfgPath, rootDir, err := FindConfig()
	if err != nil {
		return nil, "", err
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, "", fmt.Errorf("parsing %s: %w", cfgPath, err)
	}

	// If found at .claude/worktree/worktree.yml, root is the ancestor
	cfgDir := filepath.Dir(cfgPath)
	if strings.HasSuffix(cfgDir, filepath.Join(".claude", "worktree")) {
		rootDir = filepath.Dir(filepath.Dir(cfgDir))
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, "", err
	}
	return &cfg, rootDir, nil
}

func (c *Config) applyDefaults() {
	if c.MaxSlots == 0 {
		c.MaxSlots = 3
	}
	if c.PortOffset == 0 {
		c.PortOffset = 100
	}
	if c.DefaultMode == "" {
		c.DefaultMode = "dev"
	}
	if c.BranchPrefix == "" {
		c.BranchPrefix = "feature/{name}"
	}
	if c.Nginx.Port == 0 {
		c.Nginx.Port = 80
	}
	if c.Nginx.Enabled && c.Nginx.SubdomainPattern == "" {
		c.Nginx.SubdomainPattern = "{name}.{svc}.localhost"
	}
	if c.Database.SchemaPrefix == "" {
		c.Database.SchemaPrefix = "feature_"
	}
}

func (c *Config) validate() error {
	if c.Database.Isolation == "database" {
		if c.Database.Image == "" {
			return fmt.Errorf("database.image is required when isolation is \"database\"")
		}
		if c.Database.ContainerPort == 0 {
			return fmt.Errorf("database.container_port is required when isolation is \"database\"")
		}
		if c.Database.PortBase == 0 {
			return fmt.Errorf("database.port_base is required when isolation is \"database\"")
		}
	}
	for name, svc := range c.Services {
		if svc.Path == "" {
			return fmt.Errorf("services.%s.path is required", name)
		}
		if svc.PortBase == 0 {
			return fmt.Errorf("services.%s.port_base is required", name)
		}
	}
	return nil
}

// WorktreesDir returns the .worktrees directory path.
func WorktreesDir(rootDir string) string {
	return filepath.Join(rootDir, ".worktrees")
}

// SlotDir returns the path for a specific slot.
func SlotDir(rootDir string, slot int) string {
	return filepath.Join(WorktreesDir(rootDir), fmt.Sprintf("slot-%d", slot))
}

// FindNginxDir returns the nginx config directory.
// Uses conf_dir from config if set, otherwise defaults to .claude/worktree/nginx.
func (c *Config) FindNginxDir(rootDir string) string {
	if c.Nginx.ConfDir != "" {
		return filepath.Join(rootDir, c.Nginx.ConfDir)
	}
	return filepath.Join(rootDir, ".claude", "worktree", "nginx")
}

// ResolveBranch returns the branch name for a given feature name.
func (c *Config) ResolveBranch(name string) string {
	return strings.ReplaceAll(c.BranchPrefix, "{name}", name)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
