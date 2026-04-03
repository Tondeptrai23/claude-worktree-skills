package slot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SlotMeta stores metadata for a worktree slot.
type SlotMeta struct {
	Slot        int                        `yaml:"slot"`
	FeatureName string                     `yaml:"feature_name"`
	CreatedAt   string                     `yaml:"created_at"`
	Mode        string                     `yaml:"mode"`
	Services    map[string]SlotServiceMeta `yaml:"services"`
	DBSchema    string                     `yaml:"db_schema,omitempty"`
}

// SlotServiceMeta stores per-service metadata in a slot.
type SlotServiceMeta struct {
	Branch  string `yaml:"branch"`
	Port    int    `yaml:"port"`
	RepoKey string `yaml:"repo_key"`
}

const metaFileName = ".slot-meta.yml"
const legacyMetaFileName = ".slot-meta"

// MetaPath returns the path to the slot metadata file.
func MetaPath(slotDir string) string {
	return filepath.Join(slotDir, metaFileName)
}

// NewMeta creates a new SlotMeta with current timestamp.
func NewMeta(slot int, name, mode string) *SlotMeta {
	return &SlotMeta{
		Slot:        slot,
		FeatureName: name,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Mode:        mode,
		Services:    make(map[string]SlotServiceMeta),
	}
}

// Write saves the metadata to disk as YAML.
func (m *SlotMeta) Write(slotDir string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling slot meta: %w", err)
	}
	return os.WriteFile(MetaPath(slotDir), data, 0644)
}

// Load reads slot metadata from a directory.
// Tries YAML format first, falls back to legacy bash format.
func Load(slotDir string) (*SlotMeta, error) {
	// Try YAML format
	yamlPath := filepath.Join(slotDir, metaFileName)
	if data, err := os.ReadFile(yamlPath); err == nil {
		var meta SlotMeta
		if err := yaml.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", yamlPath, err)
		}
		return &meta, nil
	}

	// Try legacy bash format
	legacyPath := filepath.Join(slotDir, legacyMetaFileName)
	if _, err := os.Stat(legacyPath); err == nil {
		return loadLegacy(legacyPath)
	}

	return nil, fmt.Errorf("no slot metadata found in %s", slotDir)
}

// loadLegacy parses the old bash KEY=VALUE format.
func loadLegacy(path string) (*SlotMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	kvs := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		kvs[line[:idx]] = line[idx+1:]
	}

	meta := &SlotMeta{
		Services: make(map[string]SlotServiceMeta),
	}

	if v, ok := kvs["SLOT"]; ok {
		meta.Slot, _ = strconv.Atoi(v)
	}
	meta.FeatureName = kvs["FEATURE_NAME"]
	meta.CreatedAt = kvs["CREATED_AT"]
	meta.DBSchema = kvs["DB_SCHEMA"]

	// Map legacy port/branch keys to services
	legacyMap := map[string]struct{ portKey, branchKey string }{
		"be":              {"BE_PORT", "BE_BRANCH"},
		"genai":           {"GENAI_PORT", "GENAI_BRANCH"},
		"fe-app":          {"FE_APP_PORT", "FE_BRANCH"},
		"fe-presentation": {"FE_PRES_PORT", "FE_BRANCH"},
		"fe-admin":        {"FE_ADMIN_PORT", "FE_BRANCH"},
	}

	for svcName, keys := range legacyMap {
		portStr, hasPort := kvs[keys.portKey]
		branch, hasBranch := kvs[keys.branchKey]
		if hasPort || hasBranch {
			port, _ := strconv.Atoi(portStr)
			repoKey := strings.SplitN(svcName, "-", 2)[0] // "fe-app" → "fe"
			meta.Services[svcName] = SlotServiceMeta{
				Branch:  branch,
				Port:    port,
				RepoKey: repoKey,
			}
		}
	}

	return meta, nil
}

// DiscoverSlots finds all active slot directories and loads their metadata.
func DiscoverSlots(worktreesDir string) ([]*SlotMeta, error) {
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var slots []*SlotMeta
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "slot-") {
			continue
		}
		slotDir := filepath.Join(worktreesDir, entry.Name())
		meta, err := Load(slotDir)
		if err != nil {
			continue // skip invalid slots
		}
		slots = append(slots, meta)
	}
	return slots, nil
}
