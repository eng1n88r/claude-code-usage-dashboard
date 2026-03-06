package extract

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LoadPlans loads plan markdown files from ~/.claude/.
func LoadPlans(paths Paths) []PlanFile {
	var plans []PlanFile

	plansDir := filepath.Join(paths.ClaudeDir, "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return plans
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fullPath := filepath.Join(plansDir, entry.Name())
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		// Read file for title and line count
		title := strings.TrimSuffix(entry.Name(), ".md")
		lineCount := 0

		f, err := os.Open(fullPath)
		if err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				lineCount++
				line := scanner.Text()
				if strings.HasPrefix(line, "# ") && title == strings.TrimSuffix(entry.Name(), ".md") {
					title = strings.TrimSpace(line[2:])
				}
			}
			f.Close()
		}

		// Use modification time (st_mtime) — st_ctime on Linux is inode change, not creation
		modTime := fi.ModTime().UTC()

		plans = append(plans, PlanFile{
			Filename: entry.Name(),
			Slug:     strings.TrimSuffix(entry.Name(), ".md"),
			Title:    title,
			Created:  modTime.Format(time.RFC3339),
			Modified: modTime.Format(time.RFC3339),
			SizeKB:   math.Round(float64(fi.Size())/1024*10) / 10,
			Lines:    lineCount,
		})
	}

	return plans
}

// LoadTodos loads todo/task JSON files from ~/.claude/.
func LoadTodos(paths Paths) TodoStats {
	var stats TodoStats

	todosDir := filepath.Join(paths.ClaudeDir, "todos")
	entries, err := os.ReadDir(todosDir)
	if err != nil {
		return stats
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(todosDir, entry.Name()))
		if err != nil {
			continue
		}

		var items []struct {
			Status string `json:"status"`
		}
		if err := unmarshalJSONArray(data, &items); err != nil {
			continue
		}

		stats.Files++
		for _, item := range items {
			stats.Total++
			switch item.Status {
			case "completed":
				stats.Completed++
			case "pending", "in_progress":
				stats.Pending++
			}
		}
	}

	return stats
}

func unmarshalJSONArray(data []byte, v interface{}) error {
	if len(data) == 0 || data[0] != '[' {
		return json.Unmarshal([]byte("[]"), v)
	}
	return json.Unmarshal(data, v)
}
