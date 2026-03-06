package extract

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

// Run performs the full data extraction pipeline and returns the dashboard data.
func Run(cfg *Config, quiet bool) (*DashboardData, error) {
	paths := ResolvePaths("")

	if !quiet {
		fmt.Fprintln(os.Stderr, "Extracting Claude Code usage data...")
	}

	// Load locale
	locale, err := LoadLocale()
	if err != nil {
		return nil, fmt.Errorf("loading locale: %w", err)
	}

	// Parse locale for weekday names
	var localeMap map[string]interface{}
	_ = json.Unmarshal(locale, &localeMap)
	weekdayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	if wdn, ok := localeMap["weekdays"].([]interface{}); ok && len(wdn) == 7 {
		for i, w := range wdn {
			if s, ok := w.(string); ok {
				weekdayNames[i] = s
			}
		}
	}

	// Load .claude.json for account info
	dotClaude := loadDotClaude(paths)

	// Parse session transcripts
	if !quiet {
		fmt.Fprintln(os.Stderr, "Parsing session transcripts...")
	}
	sessions := ParseSessionTranscripts(paths, quiet)

	// Load supplementary data
	plans := LoadPlans(paths)
	plugins := LoadPlugins(paths)
	todos := LoadTodos(paths)
	fileHistory := LoadFileHistoryStats(paths)
	storage := CalcStorage(paths)

	// Determine plan tier from config
	tier := "pro"
	if len(cfg.PlanHistory) > 0 {
		latest := cfg.PlanHistory[len(cfg.PlanHistory)-1]
		tier = InferTier(latest.Tier, latest.Plan)
	}

	// Build aggregated data
	data := buildDashboardData(sessions, dotClaude, locale, weekdayNames, cfg.PlanHistory,
		plans, plugins, todos, fileHistory, storage, tier)

	return data, nil
}

func loadDotClaude(paths Paths) map[string]interface{} {
	data, err := os.ReadFile(paths.DotClaudeJSON)
	if err != nil {
		return make(map[string]interface{})
	}
	var obj map[string]interface{}
	if json.Unmarshal(data, &obj) != nil {
		return make(map[string]interface{})
	}
	return obj
}

func buildDashboardData(
	sessions map[string]*rawSession,
	dotClaude map[string]interface{},
	locale json.RawMessage,
	weekdayNames []string,
	planHistory []PlanConfig,
	plans []PlanFile,
	plugins PluginsData,
	todos TodoStats,
	fileHistory FileHistoryData,
	storage StorageData,
	tier string,
) *DashboardData {
	var sessionList []SessionOutput

	dailyCosts := make(map[string]map[string]float64)
	dailyMessages := make(map[string]int)
	dailySessions := make(map[string]int)
	hourlyMessages := make(map[int]int)
	weekdayMessages := make(map[int]int)

	type projectAccum struct {
		Sessions, Messages                             int
		Cost                                           float64
		InputTokens, OutputTokens                      int
		CacheReadTokens, CacheWriteTokens              int
		FileSize                                       int64
	}
	projectStats := make(map[string]*projectAccum)

	type modelTotals struct {
		InputTokens, OutputTokens                 int
		CacheReadTokens, CacheWriteTokens         int
		Cost                                      float64
		Calls                                     int
	}
	modelTotalsMap := make(map[string]*modelTotals)

	var totalCost float64
	var totalInput, totalOutput, totalMessages int

	for sid, sess := range sessions {
		if len(sess.Timestamps) == 0 {
			continue
		}

		sort.Slice(sess.Timestamps, func(i, j int) bool {
			return sess.Timestamps[i] < sess.Timestamps[j]
		})

		startTs := sess.Timestamps[0]
		endTs := sess.Timestamps[len(sess.Timestamps)-1]

		startDt := time.UnixMilli(startTs).UTC()
		endDt := time.UnixMilli(endTs).UTC()
		dateStr := startDt.Format("2006-01-02")
		hour := startDt.Hour()
		weekday := int(startDt.Weekday()+6) % 7 // Monday=0

		durationS := float64(endTs-startTs) / 1000

		var sessionCost float64
		var sessionInput, sessionOutput, sessionCacheRead, sessionCacheWrite, sessionCalls int
		modelBreakdown := make(map[string]ModelBreakdown)

		for model, mdata := range sess.Models {
			sessionCost += mdata.Cost
			sessionInput += mdata.InputTokens
			sessionOutput += mdata.OutputTokens
			sessionCacheRead += mdata.CacheRead
			sessionCacheWrite += mdata.CacheCreation
			sessionCalls += mdata.Calls

			displayModel := GetModelDisplay(model)

			// Daily costs
			if dailyCosts[dateStr] == nil {
				dailyCosts[dateStr] = make(map[string]float64)
			}
			dailyCosts[dateStr][displayModel] += mdata.Cost

			// Model totals
			mt := modelTotalsMap[displayModel]
			if mt == nil {
				mt = &modelTotals{}
				modelTotalsMap[displayModel] = mt
			}
			mt.InputTokens += mdata.InputTokens
			mt.OutputTokens += mdata.OutputTokens
			mt.CacheReadTokens += mdata.CacheRead
			mt.CacheWriteTokens += mdata.CacheCreation
			mt.Cost += mdata.Cost
			mt.Calls += mdata.Calls

			modelBreakdown[displayModel] = ModelBreakdown{
				Cost:         math.Round(mdata.Cost*10000) / 10000,
				OutputTokens: mdata.OutputTokens,
				Calls:        mdata.Calls,
			}
		}

		totalCost += sessionCost
		totalInput += sessionInput
		totalOutput += sessionOutput
		totalMessages += sess.MessageCount

		projName := ProjectDisplayName(sess.ProjectPath)
		ps := projectStats[projName]
		if ps == nil {
			ps = &projectAccum{}
			projectStats[projName] = ps
		}
		ps.Sessions++
		ps.Messages += sess.MessageCount
		ps.Cost += sessionCost
		ps.InputTokens += sessionInput
		ps.OutputTokens += sessionOutput
		ps.CacheReadTokens += sessionCacheRead
		ps.CacheWriteTokens += sessionCacheWrite
		ps.FileSize += sess.FileSize

		dailyMessages[dateStr] += sess.MessageCount
		dailySessions[dateStr]++
		hourlyMessages[hour] += sess.UserMsgCount
		weekdayMessages[weekday] += sess.UserMsgCount

		// Primary model = highest output tokens
		primaryModel := "Unknown"
		maxOutput := 0
		for model, mdata := range sess.Models {
			if mdata.OutputTokens > maxOutput {
				maxOutput = mdata.OutputTokens
				primaryModel = GetModelDisplay(model)
			}
		}

		sessionList = append(sessionList, SessionOutput{
			SessionID:      sid,
			Project:        projName,
			ProjectDir:     sess.ProjectDir,
			Date:           dateStr,
			Start:          startDt.Format(time.RFC3339),
			End:            endDt.Format(time.RFC3339),
			DurationMin:    math.Round(durationS/60*10) / 10,
			Cost:           math.Round(sessionCost*10000) / 10000,
			Messages:       sess.MessageCount,
			UserMessages:   sess.UserMsgCount,
			AssistMessages: sess.AssistMsgCount,
			InputTokens:    sessionInput,
			OutputTokens:   sessionOutput,
			CacheRead:      sessionCacheRead,
			CacheWrite:     sessionCacheWrite,
			APICalls:       sessionCalls,
			PrimaryModel:   primaryModel,
			ModelBreakdown: modelBreakdown,
			Tools:          sess.Tools,
			FirstPrompt:    sess.FirstPrompt,
			Slug:           sess.Slug,
			FileSizeMB:     math.Round(float64(sess.FileSize)/1048576*100) / 100,
		})
	}

	// Sort sessions by start time
	sort.Slice(sessionList, func(i, j int) bool {
		return sessionList[i].Start < sessionList[j].Start
	})

	// Collect all dates and models
	allDatesSet := make(map[string]bool)
	for d := range dailyCosts {
		allDatesSet[d] = true
	}
	for d := range dailyMessages {
		allDatesSet[d] = true
	}
	var allDates []string
	for d := range allDatesSet {
		allDates = append(allDates, d)
	}
	sort.Strings(allDates)

	var allModels []string
	for m := range modelTotalsMap {
		allModels = append(allModels, m)
	}
	sort.Strings(allModels)

	// Build daily cost series + cumulative
	var dailyCostSeries []map[string]any
	var cumulativeSeries []CumulativeCost
	cumulativeCost := 0.0

	for _, d := range allDates {
		entry := map[string]any{"date": d}
		dayTotal := 0.0
		for _, m := range allModels {
			val := 0.0
			if dailyCosts[d] != nil {
				val = dailyCosts[d][m]
			}
			entry[m] = math.Round(val*10000) / 10000
			dayTotal += val
		}
		entry["total"] = math.Round(dayTotal*10000) / 10000
		dailyCostSeries = append(dailyCostSeries, entry)

		cumulativeCost += dayTotal
		cumulativeSeries = append(cumulativeSeries, CumulativeCost{
			Date: d,
			Cost: math.Round(cumulativeCost*100) / 100,
		})
	}

	// Daily messages series
	var dailyMsgSeries []DailyMessage
	for _, d := range allDates {
		dailyMsgSeries = append(dailyMsgSeries, DailyMessage{
			Date:     d,
			Messages: dailyMessages[d],
			Sessions: dailySessions[d],
		})
	}

	// Hourly distribution
	var hourlyDist []HourlyDist
	for h := 0; h < 24; h++ {
		hourlyDist = append(hourlyDist, HourlyDist{Hour: h, Messages: hourlyMessages[h]})
	}

	// Weekday distribution
	var weekdayDist []WeekdayDist
	for i := 0; i < 7; i++ {
		weekdayDist = append(weekdayDist, WeekdayDist{
			Day:      weekdayNames[i],
			Messages: weekdayMessages[i],
		})
	}

	// Project list sorted by cost descending
	var projectList []ProjectSummary
	for name, ps := range projectStats {
		projectList = append(projectList, ProjectSummary{
			Name:             name,
			Sessions:         ps.Sessions,
			Messages:         ps.Messages,
			Cost:             math.Round(ps.Cost*100) / 100,
			InputTokens:      ps.InputTokens,
			OutputTokens:     ps.OutputTokens,
			CacheReadTokens:  ps.CacheReadTokens,
			CacheWriteTokens: ps.CacheWriteTokens,
			FileSizeMB:       math.Round(float64(ps.FileSize)/1048576*10) / 10,
		})
	}
	sort.Slice(projectList, func(i, j int) bool {
		return projectList[i].Cost > projectList[j].Cost
	})

	// Model summary sorted by cost descending
	var modelSummary []ModelSummary
	for mname, mt := range modelTotalsMap {
		modelSummary = append(modelSummary, ModelSummary{
			Model:            mname,
			Cost:             math.Round(mt.Cost*100) / 100,
			InputTokens:      mt.InputTokens,
			OutputTokens:     mt.OutputTokens,
			CacheReadTokens:  mt.CacheReadTokens,
			CacheWriteTokens: mt.CacheWriteTokens,
			Calls:            mt.Calls,
		})
	}
	sort.Slice(modelSummary, func(i, j int) bool {
		return modelSummary[i].Cost > modelSummary[j].Cost
	})

	// Cost by token type
	costByType := make(map[string]float64)
	for displayName, mt := range modelTotalsMap {
		// Find model ID by display name
		modelID := ""
		for mid, mp := range pricing {
			if mp.Display == displayName {
				modelID = mid
				break
			}
		}
		p := GetPricing(modelID)
		costByType["input"] += float64(mt.InputTokens) * p.Input / 1_000_000
		costByType["output"] += float64(mt.OutputTokens) * p.Output / 1_000_000
		costByType["cache_read"] += float64(mt.CacheReadTokens) * p.CacheRead / 1_000_000
		costByType["cache_write"] += float64(mt.CacheWriteTokens) * p.CacheWrite5m / 1_000_000
	}
	for k := range costByType {
		costByType[k] = math.Round(costByType[k]*100) / 100
	}

	// Tool summary
	globalTools := make(map[string]int)
	for _, s := range sessionList {
		for name, count := range s.Tools {
			globalTools[name] += count
		}
	}
	var toolSummary []ToolCount
	for name, count := range globalTools {
		toolSummary = append(toolSummary, ToolCount{Name: name, Count: count})
	}
	sort.Slice(toolSummary, func(i, j int) bool {
		return toolSummary[i].Count > toolSummary[j].Count
	})

	// Account from .claude.json
	var account Account
	if oauth, ok := dotClaude["oauthAccount"].(map[string]interface{}); ok {
		account.Name, _ = oauth["displayName"].(string)
		account.Email, _ = oauth["emailAddress"].(string)
	}

	// Plan analysis
	planAnalysis := BuildPlanAnalysis(planHistory, dailyCostSeries, sessionList)

	firstSession := ""
	lastSession := ""
	if len(allDates) > 0 {
		firstSession = allDates[0]
		lastSession = allDates[len(allDates)-1]
	}

	usageLimits := buildUsageLimits(sessions, tier)

	return &DashboardData{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Locale:        locale,
		Account:       account,
		KPI: KPI{
			TotalCost:      math.Round(totalCost*100) / 100,
			ActualPlanCost: planAnalysis.TotalPlanCost,
			TotalSessions:  len(sessionList),
			TotalMessages:  totalMessages,
			TotalOutput:    totalOutput,
			TotalInput:     totalInput,
			FirstSession:   firstSession,
			LastSession:    lastSession,
			TotalProjects:  len(projectList),
		},
		Plan:          planAnalysis,
		UsageLimits:   usageLimits,
		DailyCosts:    dailyCostSeries,
		Cumulative:    cumulativeSeries,
		DailyMessages: dailyMsgSeries,
		Hourly:        hourlyDist,
		Weekday:       weekdayDist,
		Models:        allModels,
		ModelSummary:  modelSummary,
		CostByType:    costByType,
		Projects:      projectList,
		Sessions:      sessionList,
		ToolSummary:   toolSummary,
		System: System{
			Plans:       plans,
			Plugins:     plugins,
			Todos:       todos,
			FileHistory: fileHistory,
			Storage:     storage,
		},
	}
}

// buildUsageLimits calculates credit consumption for session and weekly windows.
func buildUsageLimits(sessions map[string]*rawSession, tier string) *UsageLimits {
	// Collect all credit events across sessions
	var allEvents []creditEvent
	for _, sess := range sessions {
		allEvents = append(allEvents, sess.CreditEvents...)
	}

	if len(allEvents) == 0 {
		return nil
	}

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	now := time.Now().UTC()
	nowMs := now.UnixMilli()

	limits := GetPlanLimits(tier)

	// --- Session window (5h) ---
	// Find the most recent session's first timestamp, window is [start, start+5h]
	var latestSessionStart int64
	for _, sess := range sessions {
		if len(sess.Timestamps) > 0 {
			sort.Slice(sess.Timestamps, func(i, j int) bool {
				return sess.Timestamps[i] < sess.Timestamps[j]
			})
			first := sess.Timestamps[0]
			if first > latestSessionStart {
				latestSessionStart = first
			}
		}
	}

	var sessionWindow *SessionWindow
	fiveHoursMs := int64(5 * 60 * 60 * 1000)
	windowEnd := latestSessionStart + fiveHoursMs

	if latestSessionStart > 0 && windowEnd >= nowMs {
		// Active session window
		sessionCredits := 0
		for _, evt := range allEvents {
			if evt.Timestamp >= latestSessionStart && evt.Timestamp <= windowEnd {
				sessionCredits += evt.Credits
			}
		}
		pct := 0.0
		if limits.Session > 0 {
			pct = math.Round(float64(sessionCredits)/float64(limits.Session)*1000) / 10
		}

		remaining := limits.Session - sessionCredits
		if remaining < 0 {
			remaining = 0
		}

		timeRemainingMin := int(float64(windowEnd-nowMs) / 60000)
		if timeRemainingMin < 0 {
			timeRemainingMin = 0
		}

		// Project usage at current rate
		elapsedMin := float64(nowMs-latestSessionStart) / 60000
		projected := sessionCredits
		if elapsedMin > 0 {
			rate := float64(sessionCredits) / elapsedMin
			projected = int(rate * 300) // 5h = 300min
		}

		wStart := time.UnixMilli(latestSessionStart).UTC()
		wEnd := time.UnixMilli(windowEnd).UTC()

		sessionWindow = &SessionWindow{
			CreditsUsed:      sessionCredits,
			Limit:            limits.Session,
			PctUsed:          pct,
			WindowStart:      wStart.Format(time.RFC3339),
			WindowEnd:        wEnd.Format(time.RFC3339),
			RemainingCredits: remaining,
			ProjectedAtRate:  projected,
			TimeRemainingMin: timeRemainingMin,
		}
	}

	// --- Weekly window (7d rolling) ---
	weekAgoMs := nowMs - 7*24*60*60*1000
	weeklyCredits := 0
	for _, evt := range allEvents {
		if evt.Timestamp >= weekAgoMs && evt.Timestamp <= nowMs {
			weeklyCredits += evt.Credits
		}
	}
	weekPct := 0.0
	if limits.Weekly > 0 {
		weekPct = math.Round(float64(weeklyCredits)/float64(limits.Weekly)*1000) / 10
	}
	weekRemaining := limits.Weekly - weeklyCredits
	if weekRemaining < 0 {
		weekRemaining = 0
	}

	weekWindow := &WeekWindow{
		CreditsUsed:      weeklyCredits,
		Limit:            limits.Weekly,
		PctUsed:          weekPct,
		WindowStart:      time.UnixMilli(weekAgoMs).UTC().Format(time.RFC3339),
		WindowEnd:        now.Format(time.RFC3339),
		RemainingCredits: weekRemaining,
	}

	// --- Daily credit breakdown (last 7 days) ---
	dailyMap := make(map[string]int)
	for _, evt := range allEvents {
		if evt.Timestamp >= weekAgoMs {
			day := time.UnixMilli(evt.Timestamp).UTC().Format("2006-01-02")
			dailyMap[day] += evt.Credits
		}
	}
	var dailyCredits []DailyCredits
	var days []string
	for d := range dailyMap {
		days = append(days, d)
	}
	sort.Strings(days)
	for _, d := range days {
		dailyCredits = append(dailyCredits, DailyCredits{Date: d, Credits: dailyMap[d]})
	}

	// --- Model credit breakdown (weekly) ---
	modelMap := make(map[string]int)
	for _, evt := range allEvents {
		if evt.Timestamp >= weekAgoMs {
			display := GetModelDisplay(evt.Model)
			modelMap[display] += evt.Credits
		}
	}
	totalWeekCredits := weeklyCredits
	if totalWeekCredits == 0 {
		totalWeekCredits = 1 // avoid division by zero
	}
	var modelCredits []ModelCredits
	for model, credits := range modelMap {
		pct := math.Round(float64(credits)/float64(totalWeekCredits)*1000) / 10
		modelCredits = append(modelCredits, ModelCredits{
			Model:   model,
			Credits: credits,
			Pct:     pct,
		})
	}
	sort.Slice(modelCredits, func(i, j int) bool {
		return modelCredits[i].Credits > modelCredits[j].Credits
	})

	// --- Credit rates for display ---
	rates := make(map[string]CreditRate)
	for modelID, cr := range creditRates {
		display := GetModelDisplay(modelID)
		rates[display] = cr
	}

	return &UsageLimits{
		PlanTier:       tier,
		SessionLimit:   limits.Session,
		WeeklyLimit:    limits.Weekly,
		CurrentSession: sessionWindow,
		CurrentWeek:    weekWindow,
		DailyCredits:   dailyCredits,
		ModelCredits:   modelCredits,
		CreditRates:    rates,
	}
}
