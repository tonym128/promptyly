package config

import (
	"testing"
)

func TestLoadConfigAgyDefaultsAndEnv(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("PROMPTYLY_DEFAULT_PROVIDER", "agy")
	t.Setenv("PROMPTYLY_AGY_MODEL", "gemini-4-hydrogen-medium")
	t.Setenv("PROMPTYLY_AGY_EFFORT", "medium")
	t.Setenv("PROMPTYLY_CHECK_REMOTE_FIRST", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.DefaultProvider != "agy" {
		t.Errorf("expected DefaultProvider=agy, got %q", cfg.DefaultProvider)
	}
	if cfg.CheckRemoteFirst {
		t.Errorf("expected CheckRemoteFirst=false")
	}

	prov, provCfg := cfg.GetActiveProvider()
	if prov != "agy" {
		t.Errorf("expected active provider=agy, got %q", prov)
	}
	if provCfg.Model != "gemini-4-hydrogen-medium" {
		t.Errorf("expected model=gemini-4-hydrogen-medium, got %q", provCfg.Model)
	}
	if provCfg.Effort != "medium" {
		t.Errorf("expected effort=medium, got %q", provCfg.Effort)
	}
}
