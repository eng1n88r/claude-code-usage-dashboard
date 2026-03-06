package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/eng1n88r/claude-code-usage-dashboard/internal/extract"
)

func RenderOverview(data *extract.DashboardData) string {
	var b strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	b.WriteString(title.Render("Overview"))
	b.WriteString("\n\n")

	label := func(s string) string {
		return muted.Render(fmt.Sprintf("%-10s", s))
	}

	// Models
	if len(data.ModelSummary) > 0 {
		top := data.ModelSummary[0]
		b.WriteString(fmt.Sprintf("  %s %d models, top: %s (%s)\n",
			label("Models:"),
			len(data.ModelSummary),
			top.Model,
			green.Render(fmtCost(top.Cost))))
	}

	// Activity
	totalDays := 0
	if len(data.DailyMessages) > 0 {
		for _, dm := range data.DailyMessages {
			if dm.Messages > 0 {
				totalDays++
			}
		}
	}
	peakHour := 0
	peakMsgs := 0
	for _, h := range data.Hourly {
		if h.Messages > peakMsgs {
			peakMsgs = h.Messages
			peakHour = h.Hour
		}
	}
	b.WriteString(fmt.Sprintf("  %s %d messages, %d active days, peak: %02d:00\n",
		label("Activity:"),
		data.KPI.TotalMessages, totalDays, peakHour))

	// Projects
	if len(data.Projects) > 0 {
		top := data.Projects[0]
		b.WriteString(fmt.Sprintf("  %s %d projects, top: %s (%s)\n",
			label("Projects:"),
			data.KPI.TotalProjects,
			top.Name,
			green.Render(fmtCost(top.Cost))))
	}

	// Sessions
	avgMsgs := 0.0
	if data.KPI.TotalSessions > 0 {
		avgMsgs = float64(data.KPI.TotalMessages) / float64(data.KPI.TotalSessions)
	}
	b.WriteString(fmt.Sprintf("  %s %d total, avg %.1f msgs/session\n",
		label("Sessions:"),
		data.KPI.TotalSessions, avgMsgs))

	// Plan
	if data.Plan.CurrentBilling != nil {
		cb := data.Plan.CurrentBilling
		b.WriteString(fmt.Sprintf("  %s %s @ %s/mo, ROI %.1fx, %d days left\n",
			label("Billing:"),
			cb.Plan,
			fmtCost(cb.PlanCostUSD),
			cb.ROIFactor,
			cb.DaysRemaining))
	}

	// System
	b.WriteString(fmt.Sprintf("  %s %d tools, %d plugins, %.1f MB storage\n",
		label("System:"),
		len(data.ToolSummary),
		len(data.System.Plugins.Installed),
		data.System.Storage.TotalMB))

	b.WriteString("\n")
	b.WriteString(muted.Render("  Use 1-8 to switch tabs, q to quit"))

	return b.String()
}

func fmtCost(v float64) string {
	return fmt.Sprintf("$%.2f", v)
}

func fmtTokens(v int) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fK", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
}
