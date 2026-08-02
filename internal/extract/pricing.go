package extract

import "math"

// ModelPricing holds per-million-token rates for a model.
type ModelPricing struct {
	Input        float64
	Output       float64
	CacheRead    float64
	CacheWrite5m float64
	CacheWrite1h float64
	Display      string
}

var pricing = map[string]ModelPricing{
	"claude-fable-5": {
		Input: 10.00, Output: 50.00,
		CacheRead: 1.00, CacheWrite5m: 12.50, CacheWrite1h: 20.00,
		Display: "Fable 5",
	},
	"claude-mythos-5": {
		Input: 10.00, Output: 50.00,
		CacheRead: 1.00, CacheWrite5m: 12.50, CacheWrite1h: 20.00,
		Display: "Mythos 5",
	},
	"claude-opus-5": {
		Input: 5.00, Output: 25.00,
		CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
		Display: "Opus 5",
	},
	"claude-opus-4-8": {
		Input: 5.00, Output: 25.00,
		CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
		Display: "Opus 4.8",
	},
	"claude-opus-4-7": {
		Input: 5.00, Output: 25.00,
		CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
		Display: "Opus 4.7",
	},
	"claude-opus-4-6": {
		Input: 5.00, Output: 25.00,
		CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
		Display: "Opus 4.6",
	},
	"claude-opus-4-5-20251101": {
		Input: 5.00, Output: 25.00,
		CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
		Display: "Opus 4.5",
	},
	// Introductory pricing through 2026-08-31; standard $3/$15 (cache 0.30/3.75/6.00) from 2026-09-01.
	"claude-sonnet-5": {
		Input: 2.00, Output: 10.00,
		CacheRead: 0.20, CacheWrite5m: 2.50, CacheWrite1h: 4.00,
		Display: "Sonnet 5",
	},
	"claude-sonnet-4-6": {
		Input: 3.00, Output: 15.00,
		CacheRead: 0.30, CacheWrite5m: 3.75, CacheWrite1h: 6.00,
		Display: "Sonnet 4.6",
	},
	"claude-sonnet-4-5-20250929": {
		Input: 3.00, Output: 15.00,
		CacheRead: 0.30, CacheWrite5m: 3.75, CacheWrite1h: 6.00,
		Display: "Sonnet 4.5",
	},
	"claude-haiku-4-5-20251001": {
		Input: 1.00, Output: 5.00,
		CacheRead: 0.10, CacheWrite5m: 1.25, CacheWrite1h: 2.00,
		Display: "Haiku 4.5",
	},
	"claude-opus-4-1-20250805": {
		Input: 15.00, Output: 75.00,
		CacheRead: 1.50, CacheWrite5m: 18.75, CacheWrite1h: 30.00,
		Display: "Opus 4.1",
	},
	"claude-opus-4-20250514": {
		Input: 15.00, Output: 75.00,
		CacheRead: 1.50, CacheWrite5m: 18.75, CacheWrite1h: 30.00,
		Display: "Opus 4",
	},
	"claude-sonnet-4-20250514": {
		Input: 3.00, Output: 15.00,
		CacheRead: 0.30, CacheWrite5m: 3.75, CacheWrite1h: 6.00,
		Display: "Sonnet 4",
	},
	"claude-3-5-haiku-20241022": {
		Input: 0.80, Output: 4.00,
		CacheRead: 0.08, CacheWrite5m: 1.00, CacheWrite1h: 1.60,
		Display: "Haiku 3.5",
	},
}

var defaultPricing = ModelPricing{
	Input: 5.00, Output: 25.00,
	CacheRead: 0.50, CacheWrite5m: 6.25, CacheWrite1h: 10.00,
	Display: "Unknown",
}

// GetPricing returns the pricing for a model, falling back to default.
func GetPricing(modelID string) ModelPricing {
	if p, ok := pricing[modelID]; ok {
		return p
	}
	return defaultPricing
}

// GetModelDisplay returns the human-readable display name for a model.
func GetModelDisplay(modelID string) string {
	return GetPricing(modelID).Display
}

// CalcCost calculates the cost for a single API call based on token counts.
// Uses the standard cache write rate (cache_write_5m) for all cache creation
// tokens, matching Claude Code's own cost calculation.
func CalcCost(modelID string, inputTokens, outputTokens, cacheRead, cacheCreation int) float64 {
	p := GetPricing(modelID)
	return float64(inputTokens)*p.Input/1_000_000 +
		float64(outputTokens)*p.Output/1_000_000 +
		float64(cacheRead)*p.CacheRead/1_000_000 +
		float64(cacheCreation)*p.CacheWrite5m/1_000_000
}

// ProjectDisplayName extracts a short display name from a project path.
func ProjectDisplayName(projectPath string) string {
	if projectPath == "" {
		return "Unknown"
	}
	// Normalize backslashes
	p := projectPath
	for i := range p {
		if p[i] == '\\' {
			p = p[:i] + "/" + p[i+1:]
		}
	}
	// Trim trailing slash
	for len(p) > 0 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	if p == "" {
		return projectPath
	}
	// Split and take last 2 parts
	parts := splitPath(p)
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return projectPath
}

// CreditRate holds per-token credit rates for usage limit calculations.
type CreditRate struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// Credit rates follow she-llac.com/claude-limits: credit = (USD price per MTok) × 2/15.
// e.g. Opus input $5 → 10/15, output $25 → 50/15; Fable 5 $10/$50 → 20/15, 100/15.
var creditRates = map[string]CreditRate{
	"claude-fable-5":           {Input: 20.0 / 15, Output: 100.0 / 15},
	"claude-mythos-5":          {Input: 20.0 / 15, Output: 100.0 / 15},
	"claude-opus-5":            {Input: 10.0 / 15, Output: 50.0 / 15},
	"claude-opus-4-8":          {Input: 10.0 / 15, Output: 50.0 / 15},
	"claude-opus-4-7":          {Input: 10.0 / 15, Output: 50.0 / 15},
	"claude-opus-4-6":          {Input: 10.0 / 15, Output: 50.0 / 15},
	"claude-opus-4-5-20251101": {Input: 10.0 / 15, Output: 50.0 / 15},
	"claude-opus-4-1-20250805": {Input: 30.0 / 15, Output: 150.0 / 15},
	"claude-opus-4-20250514":   {Input: 30.0 / 15, Output: 150.0 / 15},
	// Sonnet 5 credits use the standard $3/$15 price, not the introductory
	// billing rate — usage limits track model tier, not promotions.
	"claude-sonnet-5":            {Input: 6.0 / 15, Output: 30.0 / 15},
	"claude-sonnet-4-6":          {Input: 6.0 / 15, Output: 30.0 / 15},
	"claude-sonnet-4-5-20250929": {Input: 6.0 / 15, Output: 30.0 / 15},
	"claude-sonnet-4-20250514":   {Input: 6.0 / 15, Output: 30.0 / 15},
	"claude-haiku-4-5-20251001":  {Input: 2.0 / 15, Output: 10.0 / 15},
	"claude-3-5-haiku-20241022":  {Input: 1.6 / 15, Output: 8.0 / 15},
}

var defaultCreditRate = CreditRate{Input: 10.0 / 15, Output: 50.0 / 15} // Opus fallback

// PlanLimits holds credit limits for a subscription tier.
type PlanLimits struct {
	Session int // 5-hour window
	Weekly  int // 7-day rolling
}

var planLimitsMap = map[string]PlanLimits{
	"pro": {Session: 550_000, Weekly: 5_000_000},
	"5x":  {Session: 3_300_000, Weekly: 41_666_700},
	"20x": {Session: 11_000_000, Weekly: 83_333_300},
}

// CalcCredits calculates credits consumed for a single API call.
func CalcCredits(modelID string, inputTokens, outputTokens int) int {
	r := defaultCreditRate
	if cr, ok := creditRates[modelID]; ok {
		r = cr
	}
	return int(math.Ceil(float64(inputTokens)*r.Input + float64(outputTokens)*r.Output))
}

// GetCreditRate returns the credit rate for a model.
func GetCreditRate(modelID string) CreditRate {
	if cr, ok := creditRates[modelID]; ok {
		return cr
	}
	return defaultCreditRate
}

// GetPlanLimits returns credit limits for the given tier.
func GetPlanLimits(tier string) PlanLimits {
	if pl, ok := planLimitsMap[tier]; ok {
		return pl
	}
	return planLimitsMap["pro"]
}

// InferTier determines the plan tier from config fields.
func InferTier(tier, planName string) string {
	if tier != "" {
		return tier
	}
	switch planName {
	case "Max":
		return "5x"
	case "Pro":
		return "pro"
	default:
		return "pro"
	}
}

func splitPath(p string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if i > start {
				parts = append(parts, p[start:i])
			}
			start = i + 1
		}
	}
	if start < len(p) {
		parts = append(parts, p[start:])
	}
	return parts
}
