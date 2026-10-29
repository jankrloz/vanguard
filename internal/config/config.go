package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Security SecurityConfig `toml:"security"`
	Packages PackagesConfig `toml:"packages"`
}

type SecurityConfig struct {
	MinSeverity   string `toml:"min_severity"`
	AutoSelectCVE bool   `toml:"auto_select_cve"`
}

type PackagesConfig struct {
	Pinned          []string `toml:"pinned"`
	IgnoredManagers []string `toml:"ignored_managers"`
}

func DefaultConfig() Config {
	return Config{
		Security: SecurityConfig{
			MinSeverity:   "HIGH",
			AutoSelectCVE: true,
		},
		Packages: PackagesConfig{
			Pinned:          []string{},
			IgnoredManagers: []string{},
		},
	}
}

func ConfigPath() string {
	if dir := os.Getenv("VANGUARD_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "vanguard", "config.toml")
}

func Load() (Config, error) {
	cfg := DefaultConfig()
	path := ConfigPath()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = Save(cfg)
		return cfg, nil
	}

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

func Save(cfg Config) error {
	path := ConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(cfg)
}

func (c *Config) TogglePin(pkgID string) (bool, error) {
	isPinned := false
	var updated []string

	for _, p := range c.Packages.Pinned {
		if p == pkgID {
			isPinned = true
		} else {
			updated = append(updated, p)
		}
	}

	if !isPinned {
		updated = append(updated, pkgID)
		isPinned = true
	} else {
		isPinned = false
	}

	c.Packages.Pinned = updated
	err := Save(*c)
	return isPinned, err
}

func (c *Config) IsPinned(pkgID string) bool {
	for _, p := range c.Packages.Pinned {
		if p == pkgID {
			return true
		}
	}
	return false
}

// EnsureMPMConfig primes MPM's configuration on macOS/Linux so that
// managers requiring elevation (such as system Ruby gems in /Library/Ruby)
// have sudo escalation enabled, preventing FilePermissionError aborts.
func EnsureMPMConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	var dir string
	if runtime.GOOS == "darwin" {
		dir = filepath.Join(home, "Library", "Application Support", "mpm")
	} else {
		dir = filepath.Join(home, ".config", "mpm")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	cfgFile := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(cfgFile)
	content := string(data)
	if err == nil && strings.Contains(content, "[mpm.overrides.gem]") {
		return nil
	}

	f, err := os.OpenFile(cfgFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	snippet := "\n[mpm.overrides.gem]\nsudo = true\n"
	_, err = f.WriteString(snippet)
	return err
}
