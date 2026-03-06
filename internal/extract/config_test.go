package extract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "testdata", "test_config.env")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skip("testdata/test_config.env not found, skipping")
	}

	cfg, err := LoadConfig(fixturePath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.PlanHistory) != 1 {
		t.Fatalf("expected 1 plan history entry, got %d", len(cfg.PlanHistory))
	}
	ph := cfg.PlanHistory[0]
	if ph.Plan != "Max" {
		t.Errorf("expected plan 'Max', got %q", ph.Plan)
	}
	if ph.CostUSD != 93.00 {
		t.Errorf("expected cost_usd 93.00, got %f", ph.CostUSD)
	}
	if ph.BillingDay != 1 {
		t.Errorf("expected billing_day 1, got %d", ph.BillingDay)
	}
	if ph.End != nil {
		t.Errorf("expected end nil, got %v", ph.End)
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/.env")
	if err == nil {
		t.Error("expected error for nonexistent config")
	}
}

func TestParseEnvFile(t *testing.T) {
	// Create a temp env file
	tmp := t.TempDir()
	p := filepath.Join(tmp, ".env")
	content := `# Comment line
PLAN_NAME="Max"
PLAN_COST_USD='93.00'
  SPACED_KEY = spaced_value
EMPTY_VAL=
`
	os.WriteFile(p, []byte(content), 0644)

	env, err := parseEnvFile(p)
	if err != nil {
		t.Fatalf("parseEnvFile failed: %v", err)
	}

	tests := map[string]string{
		"PLAN_NAME":          "Max",
		"PLAN_COST_USD":      "93.00",
		"SPACED_KEY":         "spaced_value",
		"EMPTY_VAL":          "",
	}
	for k, want := range tests {
		if got := env[k]; got != want {
			t.Errorf("env[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestConfigFromEnv_MultiplePlans(t *testing.T) {
	env := map[string]string{
		"PLAN_NAME":          "Max",
		"PLAN_TIER":          "5x",
		"PLAN_START":         "2025-02-01",
		"PLAN_END":           "",
		"PLAN_COST_USD":      "93.00",
		"PLAN_BILLING_DAY":   "1",
		"PLAN_2_NAME":        "Pro",
		"PLAN_2_TIER":        "pro",
		"PLAN_2_START":       "2025-01-01",
		"PLAN_2_END":         "2025-01-31",
		"PLAN_2_COST_USD":    "20.00",
	}

	cfg, err := configFromEnv(env)
	if err != nil {
		t.Fatalf("configFromEnv failed: %v", err)
	}

	if len(cfg.PlanHistory) != 2 {
		t.Fatalf("expected 2 plan entries, got %d", len(cfg.PlanHistory))
	}
	if cfg.PlanHistory[0].Plan != "Max" {
		t.Errorf("expected first plan 'Max', got %q", cfg.PlanHistory[0].Plan)
	}
	if cfg.PlanHistory[1].Plan != "Pro" {
		t.Errorf("expected second plan 'Pro', got %q", cfg.PlanHistory[1].Plan)
	}
	if cfg.PlanHistory[1].End == nil || *cfg.PlanHistory[1].End != "2025-01-31" {
		t.Errorf("expected second plan end '2025-01-31', got %v", cfg.PlanHistory[1].End)
	}
}

func TestLoadLocale(t *testing.T) {
	locale, err := LoadLocale()
	if err != nil {
		t.Fatalf("LoadLocale() failed: %v", err)
	}
	if len(locale) == 0 {
		t.Error("expected non-empty locale data")
	}
}


func TestFindConfig_Explicit(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "..", "testdata", "test_config.env")
	if _, err := os.Stat(fixturePath); os.IsNotExist(err) {
		t.Skip("testdata/test_config.env not found, skipping")
	}

	path, err := FindConfig(fixturePath)
	if err != nil {
		t.Fatalf("FindConfig with explicit path failed: %v", err)
	}
	if path != fixturePath {
		t.Errorf("expected %q, got %q", fixturePath, path)
	}
}

func TestFindConfig_ExplicitNotFound(t *testing.T) {
	_, err := FindConfig("/nonexistent/.env")
	if err == nil {
		t.Error("expected error for nonexistent explicit config")
	}
}
