package extract

import (
	"math"
	"os"
	"path/filepath"
	"sort"
)

// CalcStorage calculates storage breakdown for ~/.claude/.
func CalcStorage(paths Paths) StorageData {
	breakdown := make(map[string]int64)
	var total int64

	entries, err := os.ReadDir(paths.ClaudeDir)
	if err == nil {
		for _, entry := range entries {
			fullPath := filepath.Join(paths.ClaudeDir, entry.Name())
			if entry.IsDir() {
				sz := dirSize(fullPath)
				breakdown[entry.Name()+"/"] = sz
				total += sz
			} else {
				fi, err := entry.Info()
				if err == nil {
					breakdown[entry.Name()] = fi.Size()
					total += fi.Size()
				}
			}
		}
	}

	// Sort by size descending
	type kv struct {
		name string
		size int64
	}
	var sorted_ []kv
	for k, v := range breakdown {
		if v > 0 {
			sorted_ = append(sorted_, kv{k, v})
		}
	}
	sort.Slice(sorted_, func(i, j int) bool {
		return sorted_[i].size > sorted_[j].size
	})

	var items []StorageItem
	for _, kv := range sorted_ {
		items = append(items, StorageItem{
			Name:   kv.name,
			SizeMB: math.Round(float64(kv.size)/1048576*100) / 100,
		})
	}

	return StorageData{
		TotalMB: math.Round(float64(total)/1048576*10) / 10,
		Items:   items,
	}
}

// LoadFileHistoryStats counts files in file-history directories.
func LoadFileHistoryStats(paths Paths) FileHistoryData {
	var totalFiles, sessions int
	var totalSize int64

	fhDir := filepath.Join(paths.ClaudeDir, "file-history")
	entries, err := os.ReadDir(fhDir)
	if err != nil {
		return FileHistoryData{}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessions++

		sessDir := filepath.Join(fhDir, entry.Name())
		files, err := os.ReadDir(sessDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			totalFiles++
			if fi, err := f.Info(); err == nil {
				totalSize += fi.Size()
			}
		}
	}

	return FileHistoryData{
		TotalFiles:    totalFiles,
		TotalSessions: sessions,
		TotalSizeMB:   math.Round(float64(totalSize)/1048576*10) / 10,
	}
}

func dirSize(path string) int64 {
	var size int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err == nil {
			size += fi.Size()
		}
		return nil
	})
	return size
}
