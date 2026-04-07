package app

import (
	"context"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/ingest"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/schema"
)

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

type executePromptInput struct {
	Config         config.Config
	Request        Request
	Provider       provider.Provider
	ModelID        string
	SystemPrompt   string
	SchemaValue    map[string]any
	RenderMarkdown bool
	Messages       []model.Message
	Temperature    *float64
	TopP           *float64
}

func (s *Service) executePromptFlow(ctx context.Context, in executePromptInput) (string, model.CompletionResponse, bool, provider.Request, error) {
	responseText, cachedHit := s.lookupCachedPromptResponse(in)
	execReq := buildPromptRequest(in)
	if cachedHit {
		return cachedPromptResult(responseText, execReq)
	}

	completion, err := s.executePrompt(ctx, in.Provider, execReq)
	if err != nil {
		return "", model.CompletionResponse{}, false, provider.Request{}, err
	}
	return completion.Content, completion, false, execReq, nil
}

func (s *Service) lookupCachedPromptResponse(in executePromptInput) (string, bool) {
	if !in.Request.Cache {
		return "", false
	}
	cached, ok, err := s.deps.CacheStore.Get(in.ModelID, in.SystemPrompt, in.Messages, in.SchemaValue, in.Temperature, in.TopP)
	if err != nil || !ok {
		return "", false
	}
	return cached, true
}

func buildPromptRequest(in executePromptInput) provider.Request {
	return provider.Request{
		Model:       in.ModelID,
		Messages:    in.Messages,
		System:      in.SystemPrompt,
		Schema:      in.SchemaValue,
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      shouldStreamPrompt(in),
	}
}

func shouldStreamPrompt(in executePromptInput) bool {
	mode := in.Request.ModeOptions
	sampling := in.Request.SamplingOptions
	return in.Config.Stream &&
		!sampling.NoStream &&
		in.SchemaValue == nil &&
		!mode.Shell &&
		!mode.Code &&
		!in.RenderMarkdown
}

func cachedPromptResult(responseText string, execReq provider.Request) (string, model.CompletionResponse, bool, provider.Request, error) {
	execReq.Stream = false
	return responseText, model.CompletionResponse{}, true, execReq, nil
}

func (s *Service) preparePromptRun(ctx context.Context, req Request) (promptRunContext, error) {
	cfg, err := s.deps.LoadConfig()
	if err != nil {
		return promptRunContext{}, err
	}
	req = applyShellDefaults(req, cfg)
	state, err := s.resolvePromptExecutionState(req, cfg)
	if err != nil {
		return promptRunContext{}, err
	}
	schemaValue, err := schema.Load(req.Schema)
	if err != nil {
		return promptRunContext{}, err
	}
	runtimeCfg := cfg.ProviderRuntime(state.providerName)
	state.systemPrompt = applySchemaFallbackInstruction(state.systemPrompt, schemaValue, runtimeCfg.Capabilities)
	prov, resolvedProviderName, err := s.buildConfiguredProvider(cfg, runtimeCfg)
	if err != nil {
		return promptRunContext{}, err
	}
	ingested, err := s.processPromptInput(req)
	if err != nil {
		return promptRunContext{}, err
	}
	if ingested.NeedsSummarization {
		var err error
		ingested, err = s.applyPromptSummarization(ctx, cfg, req, prov, state.modelID, ingested)
		if err != nil {
			return promptRunContext{}, err
		}
	}
	promptText, err := s.appendFragments(s.preparePrompt(ingested.PromptText, req), req.Fragments)
	if err != nil {
		return promptRunContext{}, err
	}
	return s.buildPromptRunContext(cfg, req, state, schemaValue, runtimeCfg, prov, resolvedProviderName, ingested, promptText)
}

func (s *Service) applyPromptSummarization(ctx context.Context, cfg config.Config, req Request, prov provider.Provider, modelID string, ingested ingest.Result) (ingest.Result, error) {
	summarized, reduction, err := s.summarizeOversizedInput(ctx, cfg, req, prov, modelID, ingested)
	if err != nil {
		return ingest.Result{}, err
	}
	ingested.InputText = summarized
	ingested.PromptText = composeSummarizedPrompt(req, reduction, summarized)
	ingested.Reduction = reduction
	return ingested, nil
}

func (s *Service) buildPromptRunContext(cfg config.Config, req Request, state requestState, schemaValue map[string]any, runtimeCfg config.ProviderRuntime, prov provider.Provider, resolvedProviderName string, ingested ingest.Result, promptText string) (promptRunContext, error) {
	chatName, err := s.resolveChatName(req)
	if err != nil {
		return promptRunContext{}, err
	}
	session, err := s.loadSession(chatName, state.systemPrompt)
	if err != nil {
		return promptRunContext{}, err
	}
	state.systemPrompt = effectiveSessionSystemPrompt(session, state.systemPrompt)
	messages := append([]model.Message{}, session.Messages...)
	messages = append(messages, model.Message{Role: model.RoleUser, Content: promptText})
	tempPtr, topPPtr := samplingPointers(state.temperature, req.TopP)
	return promptRunContext{
		cfg:            cfg,
		promptText:     promptText,
		modelID:        state.modelID,
		systemPrompt:   state.systemPrompt,
		renderMarkdown: state.renderMarkdown,
		schemaValue:    schemaValue,
		providerName:   resolvedProviderName,
		providerCaps:   runtimeCfg.Capabilities,
		prov:           prov,
		reduction:      ingested.Reduction,
		chatName:       chatName,
		session:        session,
		messages:       messages,
		tempPtr:        tempPtr,
		topPPtr:        topPPtr,
	}, nil
}

func (s *Service) resolvePromptExecutionState(req Request, cfg config.Config) (requestState, error) {
	return s.resolveRequestState(req, cfg)
}

func (s *Service) processPromptInput(req Request) (ingest.Result, error) {
	pipeline := requestPipelineOptions(req)
	promptInput := req.PromptInput
	pipelineInput := req.PipelineInput
	return ingest.Process(ingest.Options{
		InstructionText: promptInput.PromptText,
		StdinText:       promptInput.StdinText,
		StdinMode:       promptInput.StdinMode,
		StdinLabel:      promptInput.StdinLabel,
		Profile:         pipeline.Profile,
		Shorthands:      append([]string(nil), pipeline.Shorthands...),
		Transforms:      append([]string(nil), pipeline.Transforms...),
		NoPipeline:      pipeline.NoPipeline,
		MaxInputBytes:   pipelineInput.MaxInputBytes,
		MaxInputLines:   pipelineInput.MaxInputLines,
		MaxInputTokens:  pipelineInput.MaxInputTokens,
		InputReduction:  pipelineInput.InputReduction,
		HeadLines:       pipelineInput.HeadLines,
		TailLines:       pipelineInput.TailLines,
		SampleLines:     pipelineInput.SampleLines,
	})
}
