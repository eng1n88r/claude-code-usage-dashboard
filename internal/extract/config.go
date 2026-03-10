package extract

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:generate cp ../../.env.example env.example

//go:embed locales/*.json
var localeFS embed.FS

// parseEnvFile reads a .env file and returns key-value pairs.
// Skips comments (#) and empty lines. Handles quoted values.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	env := make(map[string]string)
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
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		env[key] = val
	}
	return env, scanner.Err()
}

// configFromEnv converts a flat env map to a Config struct.
func configFromEnv(env map[string]string) (*Config, error) {
	cfg := &Config{}

	// Parse primary plan (no suffix)
	if name := env["PLAN_NAME"]; name != "" {
		plan := parsePlanFromEnv(env, "")
		cfg.PlanHistory = append(cfg.PlanHistory, plan)
	}

	// Parse numbered plans: PLAN_2_*, PLAN_3_*, ...
	for i := 2; ; i++ {
		prefix := fmt.Sprintf("PLAN_%d_", i)
		if _, ok := env[prefix+"NAME"]; !ok {
			break
		}
		plan := parsePlanFromEnv(env, prefix)
		cfg.PlanHistory = append(cfg.PlanHistory, plan)
	}

	return cfg, nil
}

func parsePlanFromEnv(env map[string]string, prefix string) PlanConfig {
	// For primary plan, keys are PLAN_NAME, PLAN_TIER, etc.
	// For numbered plans, prefix is e.g. "PLAN_2_"
	key := func(field string) string {
		if prefix == "" {
			return "PLAN_" + field
		}
		return prefix + field
	}

	pc := PlanConfig{
		Plan:  env[key("NAME")],
		Tier:  env[key("TIER")],
		Start: env[key("START")],
	}

	if end := env[key("END")]; end != "" {
		pc.End = &end
	}
	if v := env[key("COST_USD")]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			pc.CostUSD = f
		}
	}
	if v := env[key("BILLING_DAY")]; v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			pc.BillingDay = d
		}
	}
	return pc
}

// LoadConfig reads a .env config file from the given path.
// If configPath is empty, returns DefaultConfig.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return DefaultConfig(), nil
	}
	env, err := parseEnvFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("config not found: %s — run 'claude-dashboard init' to create one", configPath)
	}
	return configFromEnv(env)
}

// FindConfig looks for .env in standard locations:
// 1. Explicit path (if provided)
// 2. CWD
// 3. ~/.config/claude-dashboard/
// Returns empty string (no error) if no config file is found — defaults will be used.
func FindConfig(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("config not found at %s", explicit)
	}

	// CWD
	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, ".env")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}

	// ~/.config/claude-dashboard/
	home, err := os.UserHomeDir()
	if err == nil {
		p = filepath.Join(home, ".config", "claude-dashboard", ".env")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// No config found — will use defaults
	return "", nil
}

// DefaultConfig returns a Config with sensible defaults for first-time users.
func DefaultConfig() *Config {
	return &Config{
		PlanHistory: []PlanConfig{
			{
				Plan:       "Pro",
				Tier:       "pro",
				Start:      "2026-01-01",
				CostUSD:    20.00,
				BillingDay: 1,
			},
		},
	}
}

//go:embed env.example
var envExample []byte

// ConfigDir returns the default config directory path (~/.config/claude-dashboard/).
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "claude-dashboard")
}

// WriteDefaultConfig writes the embedded .env.example to the given path.
func WriteDefaultConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return os.WriteFile(path, envExample, 0o644)
}

// LoadLocale loads the English locale JSON from the embedded FS.
// Returns the raw JSON bytes (passed through to DashboardData.Locale).
func LoadLocale() (json.RawMessage, error) {
	data, err := localeFS.ReadFile("locales/en.json")
	if err != nil {
		return nil, fmt.Errorf("locale file not found: en.json")
	}
	return json.RawMessage(data), nil
}

// ClaudeDir returns the path to ~/.claude/
func ClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// Paths holds all resolved file/directory paths for data extraction.
type Paths struct {
	ClaudeDir     string
	ProjectsDir   string
	DotClaudeJSON string
	StatsCache    string
	HistoryJSONL  string
	OutputDir     string
	DashboardData string
	DashboardHTML string
	TemplateHTML  string
}

// ResolvePaths builds the primary Paths from the Claude home directory.
func ResolvePaths(baseDir string) Paths {
	home, _ := os.UserHomeDir()
	claudeDir := filepath.Join(home, ".claude")
	if baseDir != "" {
		claudeDir = baseDir
	}
	dotClaude := filepath.Join(home, ".claude.json")

	return Paths{
		ClaudeDir:     claudeDir,
		ProjectsDir:   filepath.Join(claudeDir, "projects"),
		DotClaudeJSON: dotClaude,
		StatsCache:    filepath.Join(claudeDir, "stats-cache.json"),
		HistoryJSONL:  filepath.Join(claudeDir, "history.jsonl"),
	}
}

