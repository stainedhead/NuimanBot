package storage

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	bytesPerMB = 1024 * 1024
	bytesPerGB = 1024 * 1024 * 1024

	// Subdirectory paths for data counting
	conversationsSubdir = "conversations"
	notesSubdir         = "notes"
	memoryCellsPath     = "memory/cells"
)

// StorageMetrics contains storage health and usage metrics.
type StorageMetrics struct {
	Status          string      `json:"status"`
	DataDirectory   string      `json:"data_directory"`
	TotalSizeMB     float64     `json:"total_size_mb"`
	DiskAvailableGB float64     `json:"disk_available_gb"`
	DiskUsedPercent float64     `json:"disk_used_percent"`
	Writable        bool        `json:"writable"`
	Users           UserMetrics `json:"users"`
	Data            DataMetrics `json:"data"`
}

// UserMetrics contains user-related metrics.
type UserMetrics struct {
	TotalCount  int `json:"total_count"`
	ActiveCount int `json:"active_count"`
}

// DataMetrics contains data-related metrics.
type DataMetrics struct {
	Conversations int `json:"conversations"`
	Notes         int `json:"notes"`
	MemoryCells   int `json:"memory_cells"`
}

// GetStorageMetrics collects storage health and usage metrics for the
// given data directory.
func GetStorageMetrics(dataDir string) (*StorageMetrics, error) {
	if dataDir == "" {
		return nil, errors.New("data directory cannot be empty")
	}

	metrics := &StorageMetrics{
		DataDirectory: dataDir,
	}

	// Check if directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		metrics.Status = "unhealthy"
		return metrics, nil
	}

	// Calculate directory size
	metrics.TotalSizeMB = calculateDirSizeMB(dataDir)

	// Get disk space info
	metrics.DiskAvailableGB, metrics.DiskUsedPercent = getDiskStats(dataDir)

	// Check if writable
	metrics.Writable = isWritable(dataDir)

	// Count users
	metrics.Users = countUsers(dataDir)

	// Count data items
	metrics.Data = countDataItems(dataDir)

	// Determine overall status
	metrics.Status = "healthy"
	if !metrics.Writable {
		metrics.Status = "unhealthy"
	}

	return metrics, nil
}

// calculateDirSizeMB walks the directory tree and returns total size in MB.
func calculateDirSizeMB(dir string) float64 {
	var totalBytes int64
	//nolint:errcheck // Best-effort size calculation; individual errors skipped in callback
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			totalBytes += info.Size()
		}
		return nil
	})
	return float64(totalBytes) / bytesPerMB
}

// getDiskStats returns available disk space in GB and used percentage.
func getDiskStats(dir string) (availableGB float64, usedPercent float64) {
	var stat unix.Statfs_t
	if err := unix.Statfs(dir, &stat); err != nil {
		return 0, 0
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)

	availableGB = float64(availBytes) / bytesPerGB

	if totalBytes > 0 {
		usedBytes := totalBytes - availBytes
		usedPercent = (float64(usedBytes) / float64(totalBytes)) * 100
	}

	return availableGB, usedPercent
}

// isWritable checks if the directory is writable by attempting to create
// and remove a temporary file.
func isWritable(dir string) bool {
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	_ = f.Close()           //nolint:errcheck // Best-effort cleanup in writability check
	_ = os.Remove(testFile) //nolint:errcheck // Best-effort cleanup in writability check
	return true
}

// countUsers reads users.json and counts total and active users.
func countUsers(dataDir string) UserMetrics {
	usersFilePath := filepath.Join(dataDir, usersFile)

	data, err := os.ReadFile(usersFilePath)
	if err != nil {
		return UserMetrics{}
	}

	var file usersFileForMetrics
	if err := json.Unmarshal(data, &file); err != nil {
		return UserMetrics{}
	}

	active := 0
	for _, user := range file.Users {
		if user.Enabled {
			active++
		}
	}

	return UserMetrics{
		TotalCount:  len(file.Users),
		ActiveCount: active,
	}
}

// usersFileForMetrics is a lightweight struct for reading user metrics
// without pulling in the full domain model.
type usersFileForMetrics struct {
	Users []userForMetrics `json:"users"`
}

// userForMetrics captures only the fields needed for metrics counting.
type userForMetrics struct {
	UserID  string `json:"userID"`
	Enabled bool   `json:"enabled"`
}

// countDataItems counts conversations, notes, and memory cells.
func countDataItems(dataDir string) DataMetrics {
	return DataMetrics{
		Conversations: countConversations(dataDir),
		Notes:         countNotes(dataDir),
		MemoryCells:   countMemoryCells(dataDir),
	}
}

// countConversations counts conversations by reading conversation index files
// from each user's conversations directory.
func countConversations(dataDir string) int {
	total := 0
	usersPath := filepath.Join(dataDir, usersDir)

	entries, err := os.ReadDir(usersPath)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		indexPath := filepath.Join(usersPath, entry.Name(), conversationsSubdir, "index.json")
		total += countConversationsFromIndex(indexPath)
	}

	return total
}

// countConversationsFromIndex reads a conversation index file and returns
// the number of conversations it contains.
func countConversationsFromIndex(indexPath string) int {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return 0
	}

	var index struct {
		Conversations map[string]interface{} `json:"conversations"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return 0
	}

	return len(index.Conversations)
}

// countNotes counts .json note files (excluding index.json) across all
// user notes directories.
func countNotes(dataDir string) int {
	total := 0
	usersPath := filepath.Join(dataDir, usersDir)

	entries, err := os.ReadDir(usersPath)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		notesDir := filepath.Join(usersPath, entry.Name(), notesSubdir)
		total += countJSONFiles(notesDir)
	}

	return total
}

// countMemoryCells counts .json cell files in the memory/cells directory.
func countMemoryCells(dataDir string) int {
	cellsDir := filepath.Join(dataDir, memoryCellsPath)
	return countJSONFiles(cellsDir)
}

// countJSONFiles counts .json files in a directory, excluding index.json.
func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && name != "index.json" {
			count++
		}
	}

	return count
}
