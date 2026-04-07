package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newLogsShowCommand(logStore *store.LogStore) *cobra.Command {
	return &cobra.Command{
		Use:  "show <id>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := logStore.Get(args[0])
			if err != nil {
				return err
			}
			if entry == nil {
				return fmt.Errorf("log entry %q not found", args[0])
			}
			output.Header("prompt")
			output.Println(entry.Prompt)
			output.Header("response")
			output.Println(entry.Response)
			if details := formatReductionDetails(entry.ReductionJSON); details != "" {
				output.Header("reduction")
				output.Println(details)
			}
			return nil
		},
	}
}

func newLogsClearCommand(logStore *store.LogStore) *cobra.Command {
	return &cobra.Command{
		Use: "clear",
		RunE: func(cmd *cobra.Command, args []string) error {
			return logStore.Clear()
		},
	}
}

func newLogsStatusCommand(logStore *store.LogStore) *cobra.Command {
	return &cobra.Command{
		Use: "status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := loadLogsStatus(logStore)
			if err != nil {
				return err
			}
			state := "OFF"
			if status.Enabled {
				state = "ON"
			}
			output.Println(fmt.Sprintf("Logging is %s for all prompts", state))
			output.Println(fmt.Sprintf("Found log database at %s", status.Path))
			output.Println(fmt.Sprintf("Number of conversations logged: %d", status.Conversations))
			output.Println(fmt.Sprintf("Number of responses logged:     %d", status.Responses))
			output.Println(fmt.Sprintf("Database file size:             %s", formatBytes(status.SizeBytes)))
			return nil
		},
	}
}

func newLogsBackupCommand(logStore *store.LogStore) *cobra.Command {
	return &cobra.Command{
		Use:   "backup [path]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Create a copy of the logs database",
		RunE: func(cmd *cobra.Command, args []string) error {
			destination, err := resolveLogsBackupPath(args)
			if err != nil {
				return err
			}
			if err := backupLogsDatabase(logStore, destination); err != nil {
				return err
			}
			output.Println(destination)
			return nil
		},
	}
}

func newLogsToggleCommand(use string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use: use,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.LogToDB = enabled
			return config.Save(cfg)
		},
	}
}

func newLogsPathCommand() *cobra.Command {
	return &cobra.Command{
		Use: "path",
		Run: func(cmd *cobra.Command, args []string) {
			output.Println(config.LogsDBPath())
		},
	}
}
