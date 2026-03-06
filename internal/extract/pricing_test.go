package extract

import (
	"math"
	"testing"
)

func TestGetPricing_KnownModel(t *testing.T) {
	p := GetPricing("claude-opus-4-5-20251101")
	if p.Display != "Opus 4.5" {
		t.Errorf("expected display 'Opus 4.5', got %q", p.Display)
	}
	if p.Input != 5.00 {
		t.Errorf("expected input 5.00, got %f", p.Input)
	}
}

func TestGetPricing_UnknownModel(t *testing.T) {
	p := GetPricing("claude-nonexistent-99")
	if p.Display != "Unknown" {
		t.Errorf("expected display 'Unknown', got %q", p.Display)
	}
	if p.Input != 5.00 {
		t.Errorf("expected default input 5.00, got %f", p.Input)
	}
}

func TestGetModelDisplay(t *testing.T) {
	tests := []struct {
		modelID string
		want    string
	}{
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-opus-4-5-20251101", "Opus 4.5"},
		{"claude-sonnet-4-5-20250929", "Sonnet 4.5"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
		{"claude-future-model", "Unknown"},
	}
	for _, tt := range tests {
		got := GetModelDisplay(tt.modelID)
		if got != tt.want {
			t.Errorf("GetModelDisplay(%q) = %q, want %q", tt.modelID, got, tt.want)
		}
	}
}

func TestCalcCost(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		input, output int
		cacheRead     int
		cacheCreation int
		wantCost      float64
	}{
		{
			name:   "opus 4.5 basic",
			model:  "claude-opus-4-5-20251101",
			input:  1000, output: 500,
			cacheRead: 200, cacheCreation: 100,
			// 1000*5/1M + 500*25/1M + 200*0.5/1M + 100*6.25/1M
			// = 0.005 + 0.0125 + 0.0001 + 0.000625 = 0.018225
			wantCost: 0.018225,
		},
		{
			name:   "haiku basic",
			model:  "claude-haiku-4-5-20251001",
			input:  500, output: 100,
			cacheRead: 0, cacheCreation: 0,
			// 500*1/1M + 100*5/1M = 0.0005 + 0.0005 = 0.001
			wantCost: 0.001,
		},
		{
			name:      "zero tokens",
			model:     "claude-opus-4-6",
			input:     0, output: 0,
			cacheRead: 0, cacheCreation: 0,
			wantCost:  0.0,
		},
		{
			name:          "unknown model uses default pricing",
			model:         "claude-mystery",
			input:         1_000_000, output: 0,
			cacheRead:     0,
			cacheCreation: 0,
			wantCost:      5.00,
		},
		{
			name:   "large token counts",
			model:  "claude-opus-4-6",
			input:  100_000, output: 50_000,
			cacheRead: 500_000, cacheCreation: 10_000,
			// 100000*5/1M + 50000*25/1M + 500000*0.5/1M + 10000*6.25/1M
			// = 0.5 + 1.25 + 0.25 + 0.0625 = 2.0625
			wantCost: 2.0625,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcCost(tt.model, tt.input, tt.output, tt.cacheRead, tt.cacheCreation)
			if math.Abs(got-tt.wantCost) > 0.000001 {
				t.Errorf("CalcCost() = %f, want %f", got, tt.wantCost)
			}
		})
	}
}

func TestCalcCredits(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		input, output int
		want          int
	}{
		{
			name: "opus basic",
			model: "claude-opus-4-6",
			input: 1000, output: 500,
			// ceil(1000 * 10/15 + 500 * 50/15) = ceil(666.67 + 1666.67) = ceil(2333.33) = 2334
			want: 2334,
		},
		{
			name: "haiku basic",
			model: "claude-haiku-4-5-20251001",
			input: 1000, output: 500,
			// ceil(1000 * 2/15 + 500 * 10/15) = ceil(133.33 + 333.33) = ceil(466.67) = 467
			want: 467,
		},
		{
			name: "sonnet basic",
			model: "claude-sonnet-4-5-20250929",
			input: 1000, output: 500,
			// ceil(1000 * 6/15 + 500 * 30/15) = ceil(400 + 1000) = 1400
			want: 1400,
		},
		{
			name: "unknown model uses opus rates",
			model: "claude-mystery",
			input: 1000, output: 500,
			want: 2334,
		},
		{
			name: "zero tokens",
			model: "claude-opus-4-6",
			input: 0, output: 0,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcCredits(tt.model, tt.input, tt.output)
			if got != tt.want {
				t.Errorf("CalcCredits(%q, %d, %d) = %d, want %d", tt.model, tt.input, tt.output, got, tt.want)
			}
		})
	}
}

func TestInferTier(t *testing.T) {
	tests := []struct {
		tier, plan string
		want       string
	}{
		{"5x", "Max", "5x"},
		{"20x", "Max", "20x"},
		{"", "Max", "5x"},
		{"", "Pro", "pro"},
		{"pro", "Pro", "pro"},
		{"", "Unknown", "pro"},
	}
	for _, tt := range tests {
		got := InferTier(tt.tier, tt.plan)
		if got != tt.want {
			t.Errorf("InferTier(%q, %q) = %q, want %q", tt.tier, tt.plan, got, tt.want)
		}
	}
}

func TestGetPlanLimits(t *testing.T) {
	pl := GetPlanLimits("5x")
	if pl.Session != 3_300_000 {
		t.Errorf("5x session limit = %d, want 3300000", pl.Session)
	}
	if pl.Weekly != 41_666_700 {
		t.Errorf("5x weekly limit = %d, want 41666700", pl.Weekly)
	}

	// Unknown tier defaults to pro
	pl = GetPlanLimits("unknown")
	if pl.Session != 550_000 {
		t.Errorf("unknown tier session limit = %d, want 550000", pl.Session)
	}
}

func TestProjectDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/projects/myapp", "projects/myapp"},
		{"C:\\Users\\me\\work\\dashboard", "work/dashboard"},
		{"/single", "single"},
		{"", "Unknown"},
		{"/a/b/c/d/e", "d/e"},
		{"/trailing/slash/", "trailing/slash"},
	}
	for _, tt := range tests {
		got := ProjectDisplayName(tt.input)
		if got != tt.want {
			t.Errorf("ProjectDisplayName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
