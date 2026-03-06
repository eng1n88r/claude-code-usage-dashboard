package tui

import (
	"github.com/eng1n88r/claude-code-usage-dashboard/internal/extract"
	"github.com/eng1n88r/claude-code-usage-dashboard/internal/tui/views"
)

// Non-interactive render functions for --all / --section output.

func RenderKPIText(data *extract.DashboardData) string {
	return renderKPILine(data)
}

func RenderOverviewText(data *extract.DashboardData) string {
	return views.RenderOverview(data)
}

func RenderTokensText(data *extract.DashboardData, width int) string {
	return views.RenderTokens(data, width)
}

func RenderActivityText(data *extract.DashboardData, width int) string {
	return views.RenderActivity(data, width)
}

func RenderProjectsText(data *extract.DashboardData, limit int) string {
	return views.RenderProjects(data, limit)
}

func RenderSessionsText(data *extract.DashboardData, limit int) string {
	return views.RenderSessions(data, limit)
}

func RenderBillingText(data *extract.DashboardData) string {
	return views.RenderBilling(data)
}

func RenderSystemText(data *extract.DashboardData) string {
	return views.RenderSystem(data)
}

func RenderLimitsText(data *extract.DashboardData) string {
	return views.RenderLimits(data)
}
