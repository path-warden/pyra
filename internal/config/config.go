// Package config loads pyra store configuration from .okf/config.yaml.
//
// Configuration governs the Canon authority tier: which directories hold Canon
// artifacts, the repository key used to mint artifact IDs, the ticketing
// provider used to format-lint external relationships, and the enforcement
// policy that classifies gate findings as blocking, advisory, or disabled.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultRepositoryKey is used to mint artifact IDs when none is configured.
const DefaultRepositoryKey = "OKF"

// DefaultSpecRoots returns the directories scanned for spec documents that can be
// projected into Canon (the local /spec layout and Kiro's). A fresh slice is
// returned each call so callers may mutate the result safely.
func DefaultSpecRoots() []string {
	return []string{"specs", ".kiro/specs"}
}

// Ticketing configures external relationship (e.g. "## Related Tickets") linting.
type Ticketing struct {
	Provider string `yaml:"provider" json:"provider"`
}

// Enforcement is the gate policy: rule codes are classified into blocking,
// advisory, or disabled. A code absent from all three keeps its rule's default
// severity.
type Enforcement struct {
	Blocking []string `yaml:"blocking" json:"blocking"`
	Advisory []string `yaml:"advisory" json:"advisory"`
	Disabled []string `yaml:"disabled" json:"disabled"`
}

// DefaultCodeRoots returns the directories code-intelligence operations search
// by default when no explicit path is given. A fresh slice is returned each call
// so callers may mutate the result safely.
func DefaultCodeRoots() []string {
	return []string{"."}
}

// Config is the resolved store configuration.
type Config struct {
	RepositoryKey string      `yaml:"repository_key" json:"repository_key"`
	CanonRoots    []string    `yaml:"canon_roots" json:"canon_roots"`
	SpecRoots     []string    `yaml:"spec_roots" json:"spec_roots"`
	CodeRoots     []string    `yaml:"code_roots" json:"code_roots"`
	Ticketing     Ticketing   `yaml:"ticketing" json:"ticketing"`
	Enforcement   Enforcement `yaml:"enforcement" json:"enforcement"`
}

// Default returns the configuration used when no .okf/config.yaml is present.
func Default() Config {
	return Config{
		RepositoryKey: DefaultRepositoryKey,
		CanonRoots:    []string{"canon"},
		SpecRoots:     DefaultSpecRoots(),
		CodeRoots:     DefaultCodeRoots(),
		Ticketing:     Ticketing{Provider: "github"},
		Enforcement:   Enforcement{},
	}
}

// Path returns the conventional config path for a store root.
func Path(storeRoot string) string {
	return filepath.Join(storeRoot, ".okf", "config.yaml")
}

// Load reads <storeRoot>/.okf/config.yaml. A missing file is not an error: the
// default configuration is returned. Any fields omitted in the file fall back to
// their defaults so partial config files are valid.
func Load(storeRoot string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path(storeRoot))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	// Unmarshal onto the defaults so unspecified keys keep their default values.
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}

	// An empty canon_roots in the file should still mean "the default root",
	// not "no canon at all", to avoid silently disabling the authority tier.
	if len(cfg.CanonRoots) == 0 {
		cfg.CanonRoots = Default().CanonRoots
	}
	// An omitted spec_roots likewise means "the defaults", not "no spec
	// discovery", so projection keeps working on an un-customized store.
	if len(cfg.SpecRoots) == 0 {
		cfg.SpecRoots = DefaultSpecRoots()
	}
	// An omitted code_roots means "the defaults", so code-intelligence search
	// keeps working on an un-customized store.
	if len(cfg.CodeRoots) == 0 {
		cfg.CodeRoots = DefaultCodeRoots()
	}
	if cfg.RepositoryKey == "" {
		cfg.RepositoryKey = DefaultRepositoryKey
	}
	return cfg, nil
}
