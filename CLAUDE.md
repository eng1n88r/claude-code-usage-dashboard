# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go CLI/TUI dashboard that parses local Claude Code session transcripts (`~/.claude/`), calculates hypothetical API costs, and displays interactive usage statistics. Built with Cobra (CLI), Bubble Tea v2 (TUI), and Lipgloss v2 (styling). Requires Go 1.25+.

## Common Commands

```bash
# Build
go build -o claude-dashboard ./cmd/claude-dashboard

# Run
./claude-dashboard

# Run tests
go test ./...

# Run a single test
go test ./internal/extract -run TestPricingCalculation

# Release snapshot (requires goreleaser)
goreleaser release --snapshot --clean
```

## Architecture

**Data flow:** CLI (`cmd/claude-dashboard/main.go`) → extraction pipeline (`internal/extract/`) → JSON output or TUI display (`internal/tui/`).

### `cmd/claude-dashboard/main.go`
Cobra CLI entry point. Three modes: interactive TUI (default), `--json` dump, `--all`/`--section` non-interactive text output. The `extract` subcommand runs extraction only. Version injected via ldflags (`-X main.version`). Auto-refreshes data if >10 minutes stale.

### `internal/extract/`
- **`extract.go`** — Main orchestrator (`Run()`). Calls all parsers, builds `DashboardData`.
- **`sessions.go`** — Parses JSONL transcript files from `~/.claude/projects/*/`. Extracts tokens, costs, tools, credit events per session.
- **`pricing.go`** — Model pricing table and cost calculation. Maps model IDs to display names and per-token rates. Credit rates for usage limits.
- **`billing.go`** — Plan/billing period analysis. Builds `PlanAnalysis` with ROI, savings, period breakdowns.
- **`config.go`** — Loads `.env` config file. Supports multiple plan history entries (numbered suffixes `_2`, `_3`). Embeds English locale JSON via `go:embed`.
- **`types.go`** — All data structures: `DashboardData`, `SessionOutput`, `UsageLimits`, etc.
- **`storage.go`** — Calculates `~/.claude/` directory sizes.
- **`history.go`** / **`plugins.go`** / **`plans.go`** — Load supplementary data (file history, installed plugins, plan files).

### `internal/tui/`
- **`app.go`** — Root Bubble Tea model with 8 tabs, viewport scrolling, async extraction with spinner.
- **`views/`** — Each tab is a separate file (overview, tokens, plan, limits, activity, projects, sessions, system). Renderers return styled strings.
- **`print.go`** — Non-interactive text renderers (`RenderOverviewText`, etc.) used by `--all`/`--section` flags.
- **`helpers.go`** — Formatting utilities (cost, tokens, percentages, bar charts).
- **`styles.go`** — Lipgloss v2 color palette and style definitions.
- **`windows.go`** — Windows-specific UTF-8 console setup.

### `internal/output/`
- **`json.go`** — Writes `dashboard_data.json` to output directory and dumps to stdout.

## Key Patterns

- Configuration via `.env` file (not Go env vars). Copy `.env.example` to `.env`.
- English locale embedded at compile time (`internal/extract/locales/en.json`).
- Credit-based usage limits tracking with 5-hour session windows and 7-day rolling weekly windows.
- All costs are hypothetical API-equivalent costs, not actual charges.
- Cross-platform: builds for linux/darwin/windows × amd64/arm64 via GoReleaser.
