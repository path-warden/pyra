package llm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FromBundle(t *testing.T) {
	tmp := t.TempDir()
	cfgYAML := `api_endpoint: https://example.com/v1
api_token: tok-123
model: gpt-4o-mini
prompt_template: "Summarize: {{.Content}}"
local_model: phi3:mini
local_endpoint: http://localhost:11434
`
	path := filepath.Join(tmp, ConfigFileName)
	if err := os.WriteFile(path, []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadConfig(tmp)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIEndpoint != "https://example.com/v1" {
		t.Errorf("APIEndpoint = %q", cfg.APIEndpoint)
	}
	if cfg.APIToken != "tok-123" {
		t.Errorf("APIToken = %q", cfg.APIToken)
	}
	if cfg.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.LocalModel != "phi3:mini" {
		t.Errorf("LocalModel = %q", cfg.LocalModel)
	}
	if cfg.LocalEndpoint != "http://localhost:11434" {
		t.Errorf("LocalEndpoint = %q", cfg.LocalEndpoint)
	}
}

func TestLoadConfig_Missing(t *testing.T) {
	// Make sure we don't accidentally pick up a real ~/.config/pyra/llm.config
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfg, err := LoadConfig(tmp)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil zero config")
	} else if cfg.APIEndpoint != "" || cfg.APIToken != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestConfig_HasAPI(t *testing.T) {
	c := &Config{APIEndpoint: "x", APIToken: "y"}
	if !c.HasAPI() {
		t.Error("expected HasAPI true")
	}
	c2 := &Config{APIEndpoint: "x"}
	if c2.HasAPI() {
		t.Error("expected HasAPI false without token")
	}
}
