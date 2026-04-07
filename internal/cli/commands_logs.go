package cli

import (
	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/store"
)

func newLogsCommand() *cobra.Command {
	opts := &logsQueryOptions{}
	logStore := newLogsStore()
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View and search logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogsQuery(cmd, logStore, *opts)
		},
	}
	registerLogsQueryFlags(cmd, opts)
	addLogsSubcommands(cmd, logStore)
	return cmd
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

func registerLogsQueryFlags(cmd *cobra.Command, opts *logsQueryOptions) {
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
}

func addLogsSubcommands(cmd *cobra.Command, logStore *store.LogStore) {
	cmd.AddCommand(
		newLogsShowCommand(logStore),
		newLogsClearCommand(logStore),
		newLogsStatusCommand(logStore),
		newLogsBackupCommand(logStore),
		newLogsToggleCommand("on", true),
		newLogsToggleCommand("off", false),
		newLogsPathCommand(),
	)
}
