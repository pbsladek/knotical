package app

import "context"

func (s *Service) RunPrompt(ctx context.Context, req Request) error {
	runCtx, err := s.preparePromptRun(ctx, req)
	if err != nil {
		return err
	}

	responseText, completion, cachedHit, execReq, err := s.executePromptFlow(ctx, executePromptInput{
		Config:         runCtx.cfg,
		Request:        req,
		Provider:       runCtx.prov,
		ModelID:        runCtx.modelID,
		SystemPrompt:   runCtx.systemPrompt,
		SchemaValue:    runCtx.schemaValue,
		RenderMarkdown: runCtx.renderMarkdown,
		Messages:       runCtx.messages,
		Temperature:    runCtx.tempPtr,
		TopP:           runCtx.topPPtr,
	})
	if err != nil {
		return err
	}

	responseText, err = s.transformPromptResponse(req, runCtx.schemaValue, responseText)
	if err != nil {
		return err
	}
	if err := s.renderPromptResult(ctx, renderPromptResultInput{
		Request:        req,
		Config:         runCtx.cfg,
		Provider:       runCtx.prov,
		ModelID:        runCtx.modelID,
		PromptText:     runCtx.promptText,
		ResponseText:   responseText,
		RenderMarkdown: runCtx.renderMarkdown,
		Temperature:    runCtx.tempPtr,
		TopP:           runCtx.topPPtr,
		ExecRequest:    execReq,
	}); err != nil {
		return err
	}
	return s.finalizePromptRun(runCtx, req, responseText, completion, cachedHit)
}
