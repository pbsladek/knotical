package cli

import (
	"github.com/pbsladek/knotical/internal/app"
	"github.com/pbsladek/knotical/internal/shell"
)

func toAppRequest(opts rootOptions, prompt promptSource) app.Request {
	req := opts.Request
	req.PromptText = prompt.instructionText
	req.StdinText = prompt.stdinText
	req.Transforms = append([]string(nil), req.Transforms...)
	req.Fragments = append([]string(nil), req.Fragments...)
	req.ExecuteMode = shell.ExecutionMode(opts.Execute)
	return req
}
