package cli

import (
	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newLogsStore() *store.LogStore {
	return store.NewLogStore(config.LogsDBPath())
}

func runLogsQuery(cmd *cobra.Command, logStore *store.LogStore, opts logsQueryOptions) error {
	filter, err := buildLogFilter(opts)
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
}

func buildLogFilter(opts logsQueryOptions) (store.LogFilter, error) {
	if opts.LatestConversation && opts.Conversation != "" {
		return store.LogFilter{}, errConflictingConversationFilters
	}
	if opts.IDGT != "" && opts.IDGTE != "" {
		return store.LogFilter{}, errConflictingIDFilters
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
