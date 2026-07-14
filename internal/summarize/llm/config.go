package llm

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds LLM provider settings parsed from llm.config (YAML).
type Config struct {
	APIEndpoint    string `yaml:"api_endpoint"`
	APIToken       string `yaml:"api_token"`
	Model          string `yaml:"model"`
	PromptTemplate string `yaml:"prompt_template"`
	LocalModel     string `yaml:"local_model"`
	LocalEndpoint  string `yaml:"local_endpoint"`
}

// ConfigFileName is the on-disk name of the LLM config file.
const ConfigFileName = "llm.config"

// LoadConfig searches for llm.config in (1) the bundle directory, (2) the
// user config directory (~/.config/pyra/), and parses the first one found.
// If no config is found, an empty Config is returned (callers should treat
// that as "use platform-native LLM").
func LoadConfig(bundlePath string) (*Config, error) {
	candidates := []string{}
	if bundlePath != "" {
		candidates = append(candidates, filepath.Join(bundlePath, ConfigFileName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "pyra", ConfigFileName))
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		cfg := &Config{}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		return cfg, nil
	}

	// No config file present — return a zero-value Config.
	return &Config{}, nil
}

// HasAPI reports whether the config specifies a complete external API.
func (c *Config) HasAPI() bool {
	return c.APIEndpoint != "" && c.APIToken != ""
}
