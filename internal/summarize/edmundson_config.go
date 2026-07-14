package summarize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// EdmundsonConfig holds the bonus/stigma/null word lists used by the
// Edmundson extractive summarizer's cue method. Words are matched
// case-insensitively against the (raw) words of each sentence.
//
// YAML format (the key "null" is reserved by YAML, so the file uses
// "null_words" instead):
//
//	bonus_words:
//	  - significant
//	  - important
//	stigma_words:
//	  - hardly
//	  - lateral
//	null_words:
//	  - example
//	  - clearly
type EdmundsonConfig struct {
	Bonus  []string `yaml:"bonus_words"`
	Stigma []string `yaml:"stigma_words"`
	Null   []string `yaml:"null_words"`
}

// EdmundsonConfigFileName is the on-disk filename searched for in standard
// locations (bundle directory, then ~/.config/pyra/).
const EdmundsonConfigFileName = "edmundson.config"

// LoadEdmundsonConfig searches for an Edmundson config in the following
// order:
//  1. explicitPath, if non-empty
//  2. bundlePath/edmundson.config, if bundlePath is non-empty
//  3. ~/.config/pyra/edmundson.config
//
// If none is found, an empty EdmundsonConfig is returned (caller should
// treat that as "no cue words configured"). Returning an empty config is
// not an error.
func LoadEdmundsonConfig(bundlePath, explicitPath string) (*EdmundsonConfig, error) {
	candidates := []string{}
	if explicitPath != "" {
		candidates = append(candidates, explicitPath)
	}
	if bundlePath != "" {
		candidates = append(candidates, filepath.Join(bundlePath, EdmundsonConfigFileName))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "pyra", EdmundsonConfigFileName))
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		cfg := &EdmundsonConfig{}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		// Normalize: trim whitespace and lowercase for comparison consistency.
		cfg.Bonus = normalizeWordList(cfg.Bonus)
		cfg.Stigma = normalizeWordList(cfg.Stigma)
		cfg.Null = normalizeWordList(cfg.Null)
		return cfg, nil
	}

	return &EdmundsonConfig{}, nil
}

// normalizeWordList trims whitespace, lowercases, and drops empty entries.
func normalizeWordList(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}
