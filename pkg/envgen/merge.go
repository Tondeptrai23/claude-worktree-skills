package envgen

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"claude-worktree-skill/pkg/config"
)

// MergeEnv copies main .env (secrets) + applies .env.overrides for each service in the slot.
// Also copies private_files.
func MergeEnv(slot int, cfg *config.Config, rootDir, slotDir string) error {
	for svcName, svc := range cfg.Services {
		workDir := svc.WorkDir(slotDir)
		if _, err := os.Stat(workDir); os.IsNotExist(err) {
			continue
		}

		overridesPath := filepath.Join(workDir, ".env.overrides")
		if _, err := os.Stat(overridesPath); os.IsNotExist(err) {
			continue
		}

		mainDir := svc.MainDir(rootDir)
		dstEnv := filepath.Join(workDir, ".env")

		// Step 1: Copy base .env from main checkout
		srcEnv := findEnvSource(mainDir, svc.EnvFile, svc.EnvSample)
		if srcEnv != "" {
			if err := copyFile(srcEnv, dstEnv); err != nil {
				return fmt.Errorf("copying %s: %w", srcEnv, err)
			}
			if strings.HasSuffix(srcEnv, ".sample") || strings.HasSuffix(srcEnv, ".example") {
				fmt.Printf("  [!] %s: using %s (secrets will be empty)\n", svcName, filepath.Base(srcEnv))
			}
		} else {
			os.WriteFile(dstEnv, []byte{}, 0644)
		}

		// Step 2: Apply overrides
		if err := applyOverrides(dstEnv, overridesPath); err != nil {
			return fmt.Errorf("applying overrides for %s: %w", svcName, err)
		}

		fmt.Printf("  [env] Merged %s/.env\n", svcName)
	}

	// Step 3: Copy private files
	for _, pf := range cfg.PrivateFiles {
		src := filepath.Join(rootDir, pf)
		dst := filepath.Join(slotDir, pf)
		if _, err := os.Stat(src); err == nil {
			os.MkdirAll(filepath.Dir(dst), 0755)
			copyFile(src, dst)
		}
	}

	return nil
}

// findEnvSource returns the path to the best available .env source file.
func findEnvSource(mainDir, envFile, envSample string) string {
	if envFile != "" {
		p := filepath.Join(mainDir, envFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try common sample names
	for _, name := range []string{envSample, ".env.sample", ".env.example"} {
		if name == "" {
			continue
		}
		p := filepath.Join(mainDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// applyOverrides reads .env.overrides and applies each key=value to the env file.
func applyOverrides(envPath, overridesPath string) error {
	overrides, err := parseEnvFile(overridesPath)
	if err != nil {
		return err
	}

	// Read existing env file
	existing, orderedKeys, lines, err := parseEnvFileWithOrder(envPath)
	if err != nil {
		return err
	}

	// Ensure trailing newline
	if len(lines) > 0 && lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}

	// Replace existing keys or mark as appended
	applied := make(map[string]bool)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		key := trimmed[:idx]
		if val, ok := overrides[key]; ok {
			lines[i] = key + "=" + val
			applied[key] = true
		}
	}

	// Append new keys
	for _, key := range orderedKeys {
		if applied[key] {
			continue
		}
		if _, exists := existing[key]; exists {
			continue
		}
		if val, ok := overrides[key]; ok {
			lines = append(lines, key+"="+val)
		}
	}

	_ = orderedKeys // suppress unused warning handled above

	// Write back
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(envPath, []byte(content), 0644)
}

// parseEnvFile reads a .env file into a key-value map, skipping comments.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
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
		result[line[:idx]] = line[idx+1:]
	}
	return result, nil
}

// parseEnvFileWithOrder reads an env file preserving line order.
func parseEnvFileWithOrder(path string) (map[string]string, []string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil, nil, nil
		}
		return nil, nil, nil, err
	}
	defer f.Close()

	kvs := make(map[string]string)
	var keys []string
	var lines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		key := trimmed[:idx]
		kvs[key] = trimmed[idx+1:]
		keys = append(keys, key)
	}

	return kvs, keys, lines, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
