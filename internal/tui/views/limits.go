package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/eng1n88r/claude-code-usage-dashboard/internal/extract"
)

func RenderLimits(data *extract.DashboardData) string {
	var b strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)

	ul := data.UsageLimits
	if ul == nil {
		b.WriteString(title.Render("Usage Limits"))
		b.WriteString("\n\n")
		b.WriteString(muted.Render("  No usage data available. Run some Claude Code sessions first."))
		b.WriteString("\n")
		return b.String()
	}

	// Session Window
	b.WriteString(title.Render("Current Session (5-hour window)"))
	b.WriteString("\n\n")

	if ul.CurrentSession != nil {
		sw := ul.CurrentSession
		bar := makeProgressBar(sw.PctUsed, 40)
		b.WriteString(fmt.Sprintf("  %s  %.1f%%\n", bar, sw.PctUsed))
		b.WriteString(fmt.Sprintf("  %s / %s credits used\n",
			bright.Render(fmtCredits(sw.CreditsUsed)),
			fmtCredits(sw.Limit)))
		b.WriteString(fmt.Sprintf("  Remaining: %s credits\n", fmtCredits(sw.RemainingCredits)))
		b.WriteString(fmt.Sprintf("  Time remaining: %d min\n", sw.TimeRemainingMin))
		b.WriteString(fmt.Sprintf("  At current rate: projected %s (%.1f%%)\n",
			fmtCredits(sw.ProjectedAtRate),
			float64(sw.ProjectedAtRate)/float64(sw.Limit)*100))
	} else {
		b.WriteString(muted.Render("  No active session window"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Weekly Window
	b.WriteString(title.Render("Current Week (7-day rolling)"))
	b.WriteString("\n\n")

	if ul.CurrentWeek != nil {
		ww := ul.CurrentWeek
		bar := makeProgressBar(ww.PctUsed, 40)
		b.WriteString(fmt.Sprintf("  %s  %.1f%%\n", bar, ww.PctUsed))
		b.WriteString(fmt.Sprintf("  %s / %s credits used\n",
			bright.Render(fmtCredits(ww.CreditsUsed)),
			fmtCredits(ww.Limit)))
		b.WriteString(fmt.Sprintf("  Remaining: %s credits\n", fmtCredits(ww.RemainingCredits)))
	}
	b.WriteString("\n")

	// Daily Credit Usage
	if len(ul.DailyCredits) > 0 {
		b.WriteString(title.Render("Daily Credits (last 7 days)"))
		b.WriteString("\n\n")

		maxCredits := 0
		for _, dc := range ul.DailyCredits {
			if dc.Credits > maxCredits {
				maxCredits = dc.Credits
			}
		}

		barWidth := 30
		for _, dc := range ul.DailyCredits {
			filled := 0
			if maxCredits > 0 {
				filled = int(float64(dc.Credits) / float64(maxCredits) * float64(barWidth))
			}
			bar := ""
			for i := 0; i < filled; i++ {
				bar += "█"
			}
			for i := filled; i < barWidth; i++ {
				bar += "░"
			}
			// Show just month and day from the date
			dateLabel := dc.Date
			if len(dc.Date) >= 10 {
				dateLabel = dc.Date[5:10] // MM-DD
			}
			b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
				muted.Render(dateLabel),
				lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render(bar),
				fmtCredits(dc.Credits)))
		}
		b.WriteString("\n")
	}

	// Model Credit Distribution
	if len(ul.ModelCredits) > 0 {
		b.WriteString(title.Render("Credits by Model (last 7 days)"))
		b.WriteString("\n\n")

		barWidth := 30
		for _, mc := range ul.ModelCredits {
			filled := int(mc.Pct / 100 * float64(barWidth))
			if filled > barWidth {
				filled = barWidth
			}
			bar := ""
			for i := 0; i < filled; i++ {
				bar += "█"
			}
			for i := filled; i < barWidth; i++ {
				bar += "░"
			}
			b.WriteString(fmt.Sprintf("  %-12s %s  %5.1f%%  %s\n",
				mc.Model,
				lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render(bar),
				mc.Pct,
				fmtCredits(mc.Credits)))
		}
		b.WriteString("\n")
	}

	// Credit Rates Reference
	if len(ul.CreditRates) > 0 {
		b.WriteString(title.Render("Credit Rates (per token)"))
		b.WriteString("\n\n")

		var rows [][]string
		for model, cr := range ul.CreditRates {
			rows = append(rows, []string{
				model,
				fmt.Sprintf("%.4f", cr.Input),
				fmt.Sprintf("%.4f", cr.Output),
			})
		}

		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Padding(0, 1)
				}
				return lipgloss.NewStyle().Padding(0, 1)
			}).
			Headers("Model", "Input Rate", "Output Rate").
			Rows(rows...)

		b.WriteString(t.String())
		b.WriteString("\n\n")
	}

	// Plan summary footer
	tierLabel := ul.PlanTier
	switch tierLabel {
	case "5x":
		tierLabel = "Max 5x"
	case "20x":
		tierLabel = "Max 20x"
	case "pro":
		tierLabel = "Pro"
	}
	b.WriteString(muted.Render(fmt.Sprintf("  Plan: %s | Session: %s | Weekly: %s",
		tierLabel, fmtCredits(ul.SessionLimit), fmtCredits(ul.WeeklyLimit))))
	b.WriteString("\n")

	return b.String()
}

func makeProgressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	var colorStr string
	switch {
	case pct >= 90:
		colorStr = "196" // red
	case pct >= 70:
		colorStr = "214" // orange
	default:
		colorStr = "82" // green
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorStr)).Render(bar)
}

func fmtCredits(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
