package cli

import "github.com/pbsladek/knotical/internal/model"

type logsRenderOptions struct {
	JSON         bool
	ResponseOnly bool
	Extract      bool
	ExtractLast  bool
	Short        bool
}

func renderLogEntries(entries []model.LogEntry, opts logsRenderOptions) (string, error) {
	switch {
	case opts.JSON:
		return renderLogEntriesJSON(entries)
	case opts.ResponseOnly:
		return joinRendered(entries, func(entry model.LogEntry) string { return entry.Response }), nil
	case opts.Extract:
		return joinRendered(entries, func(entry model.LogEntry) string { return extractLogCodeBlock(entry.Response, false) }), nil
	case opts.ExtractLast:
		return joinRendered(entries, func(entry model.LogEntry) string { return extractLogCodeBlock(entry.Response, true) }), nil
	case opts.Short:
		return renderShortLogEntries(entries), nil
	default:
		return renderDefaultLogEntries(entries), nil
	}
}
