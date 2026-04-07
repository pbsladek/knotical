package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newLogsCommand() *cobra.Command {
	opts := &logsQueryOptions{}
	logStore := store.NewLogStore(config.LogsDBPath())
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View and search logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, err := buildLogFilter(*opts)
			if err != nil {
				return err
			}
			entries, err := logStore.Query(filter)
			if err != nil {
				return err
			}
			rendered, err := renderLogEntries(entries, logsRenderOptions{
				JSON:         opts.JSON,
				ResponseOnly: opts.ResponseOnly,
				Extract:      opts.Extract,
				ExtractLast:  opts.ExtractLast,
				Short:        opts.Short,
			})
			if err != nil {
				return err
			}
			if rendered != "" {
				output.Print(rendered)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&opts.Count, "count", "n", 10, "Number of recent entries to show")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Filter by model")
	cmd.Flags().StringVarP(&opts.Search, "search", "q", "", "Search substring")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "Render log entries as JSON")
	cmd.Flags().BoolVarP(&opts.ResponseOnly, "response", "r", false, "Print only the response text")
	cmd.Flags().BoolVarP(&opts.Extract, "extract", "x", false, "Extract the first fenced code block from responses")
	cmd.Flags().BoolVar(&opts.ExtractLast, "extract-last", false, "Extract the last fenced code block from responses")
	cmd.Flags().BoolVarP(&opts.Short, "short", "s", false, "Render a shortened summary view")
	cmd.Flags().BoolVarP(&opts.LatestConversation, "conversation", "c", false, "Show logs for the most recent conversation")
	cmd.Flags().StringVar(&opts.Conversation, "cid", "", "Show logs for a specific conversation ID")
	cmd.Flags().BoolVarP(&opts.Latest, "latest", "l", false, "Sort search results by latest first")
	cmd.Flags().StringVar(&opts.IDGT, "id-gt", "", "Show records with IDs greater than this value")
	cmd.Flags().StringVar(&opts.IDGTE, "id-gte", "", "Show records with IDs greater than or equal to this value")
	cmd.AddCommand(
		newLogsShowCommand(logStore),
		newLogsClearCommand(logStore),
		newLogsStatusCommand(logStore),
		newLogsBackupCommand(logStore),
		newLogsToggleCommand("on", true),
		newLogsToggleCommand("off", false),
		newLogsPathCommand(),
	)
	return cmd
}

type logsStatus struct {
	Enabled       bool
	Path          string
	Conversations int
	Responses     int
	SizeBytes     int64
}

type logsQueryOptions struct {
	Count              int
	Model              string
	Search             string
	JSON               bool
	ResponseOnly       bool
	Extract            bool
	ExtractLast        bool
	Short              bool
	LatestConversation bool
	Conversation       string
	Latest             bool
	IDGT               string
	IDGTE              string
}

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

func buildLogFilter(opts logsQueryOptions) (store.LogFilter, error) {
	if opts.LatestConversation && opts.Conversation != "" {
		return store.LogFilter{}, fmt.Errorf("--conversation and --cid cannot be used together")
	}
	if opts.IDGT != "" && opts.IDGTE != "" {
		return store.LogFilter{}, fmt.Errorf("--id-gt and --id-gte cannot be used together")
	}
	return store.LogFilter{
		Conversation:       opts.Conversation,
		LatestConversation: opts.LatestConversation,
		Model:              opts.Model,
		Search:             opts.Search,
		Latest:             opts.Latest,
		IDGT:               opts.IDGT,
		IDGTE:              opts.IDGTE,
		Limit:              opts.Count,
	}, nil
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
