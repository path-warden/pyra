package summarize

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEdmundsonConfig_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "custom.yaml")
	body := `bonus_words:
  - significant
  - Important
stigma_words:
  - hardly
null_words:
  - example
`
	if err := os.WriteFile(cfgPath, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadEdmundsonConfig("", cfgPath)
	if err != nil {
		t.Fatalf("LoadEdmundsonConfig: %v", err)
	}
	// Normalization should lowercase and trim.
	wantBonus := []string{"significant", "important"}
	if !equalSlices(cfg.Bonus, wantBonus) {
		t.Errorf("Bonus = %v, want %v", cfg.Bonus, wantBonus)
	}
	if !equalSlices(cfg.Stigma, []string{"hardly"}) {
		t.Errorf("Stigma = %v", cfg.Stigma)
	}
	if !equalSlices(cfg.Null, []string{"example"}) {
		t.Errorf("Null = %v", cfg.Null)
	}
}

func TestLoadEdmundsonConfig_BundleDir(t *testing.T) {
	bundle := t.TempDir()
	body := `bonus_words: [alpha, beta]
stigma_words: [gamma]
`
	if err := os.WriteFile(filepath.Join(bundle, EdmundsonConfigFileName), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("HOME", t.TempDir()) // ensure no home config interferes
	cfg, err := LoadEdmundsonConfig(bundle, "")
	if err != nil {
		t.Fatalf("LoadEdmundsonConfig: %v", err)
	}
	if !equalSlices(cfg.Bonus, []string{"alpha", "beta"}) {
		t.Errorf("Bonus = %v", cfg.Bonus)
	}
}

func TestLoadEdmundsonConfig_HomeDir(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "pyra")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `bonus_words: [home-bonus]
`
	if err := os.WriteFile(filepath.Join(cfgDir, EdmundsonConfigFileName), []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("HOME", home)
	cfg, err := LoadEdmundsonConfig("", "")
	if err != nil {
		t.Fatalf("LoadEdmundsonConfig: %v", err)
	}
	if !equalSlices(cfg.Bonus, []string{"home-bonus"}) {
		t.Errorf("Bonus = %v", cfg.Bonus)
	}
}

func TestLoadEdmundsonConfig_Missing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := LoadEdmundsonConfig(t.TempDir(), "")
	if err != nil {
		t.Fatalf("expected nil error for missing config, got %v", err)
	}
	if len(cfg.Bonus) != 0 || len(cfg.Stigma) != 0 || len(cfg.Null) != 0 {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoadEdmundsonConfig_PrecedenceExplicitOverBundle(t *testing.T) {
	bundle := t.TempDir()
	if err := os.WriteFile(filepath.Join(bundle, EdmundsonConfigFileName), []byte("bonus_words: [from-bundle]\n"), 0644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	explicit := filepath.Join(t.TempDir(), "explicit.yaml")
	if err := os.WriteFile(explicit, []byte("bonus_words: [from-explicit]\n"), 0644); err != nil {
		t.Fatalf("write explicit: %v", err)
	}
	cfg, err := LoadEdmundsonConfig(bundle, explicit)
	if err != nil {
		t.Fatalf("LoadEdmundsonConfig: %v", err)
	}
	if !equalSlices(cfg.Bonus, []string{"from-explicit"}) {
		t.Errorf("expected explicit to win, got %v", cfg.Bonus)
	}
}

func TestLoadEdmundsonConfig_InvalidYAML(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("bonus_words: [unclosed"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadEdmundsonConfig("", cfgPath)
	if err == nil {
		t.Error("expected parse error")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
