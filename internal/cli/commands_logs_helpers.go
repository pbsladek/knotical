package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/store"
)

var (
	errConflictingConversationFilters = errors.New("--conversation and --cid cannot be used together")
	errConflictingIDFilters           = errors.New("--id-gt and --id-gte cannot be used together")
)

type logsStatus struct {
	Enabled       bool
	Path          string
	Conversations int
	Responses     int
	SizeBytes     int64
}

func loadLogsStatus(logStore *store.LogStore) (logsStatus, error) {
	cfg, err := config.Load()
	if err != nil {
		return logsStatus{}, err
	}
	status := logsStatus{
		Enabled: cfg.LogToDB,
		Path:    config.LogsDBPath(),
	}
	info, err := os.Stat(status.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return logsStatus{}, err
	}
	status.SizeBytes = info.Size()
	status.Responses, err = logStore.Count()
	if err != nil {
		return logsStatus{}, err
	}
	status.Conversations, err = logStore.CountConversations()
	if err != nil {
		return logsStatus{}, err
	}
	return status, nil
}

func formatBytes(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.2f KB", float64(size)/unit)
}

func resolveLogsBackupPath(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return filepath.Join(config.ConfigDir(), fmt.Sprintf("logs-backup-%s.db", time.Now().UTC().Format("20060102-150405"))), nil
}

func backupLogsDatabase(logStore *store.LogStore, destination string) error {
	return logStore.Backup(destination)
}
