package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/schema"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

func (s *Service) RunPrompt(ctx context.Context, req Request) error {
	runCtx, err := s.preparePromptRun(req)
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
	return in.Config.Stream &&
		!in.Request.NoStream &&
		in.SchemaValue == nil &&
		!in.Request.Shell &&
		!in.Request.Code &&
		!in.RenderMarkdown
}

func cachedPromptResult(responseText string, execReq provider.Request) (string, model.CompletionResponse, bool, provider.Request, error) {
	execReq.Stream = false
	return responseText, model.CompletionResponse{}, true, execReq, nil
}

type renderPromptResultInput struct {
	Request        Request
	Config         config.Config
	Provider       provider.Provider
	ModelID        string
	PromptText     string
	ResponseText   string
	RenderMarkdown bool
	Temperature    *float64
	TopP           *float64
	ExecRequest    provider.Request
}

func (s *Service) renderPromptResult(ctx context.Context, in renderPromptResultInput) error {
	switch {
	case in.Request.Shell:
		return s.renderShellResult(ctx, in)
	case in.Request.Code || in.Request.Schema != "" || in.Request.DescribeShell:
		s.deps.Printer.PrintResponse(in.ResponseText, false)
	case !in.ExecRequest.Stream:
		s.deps.Printer.PrintResponse(in.ResponseText, in.RenderMarkdown)
	}
	return nil
}

func (s *Service) renderShellResult(ctx context.Context, in renderPromptResultInput) error {
	s.deps.Printer.PrintResponse(in.ResponseText, false)
	report := shell.AnalyzeCommand(in.ResponseText)
	if report.HighRisk {
		s.deps.Printer.Warn("High-risk shell command detected: " + strings.Join(report.Reasons, "; "))
	}
	if in.Request.ExecuteMode != "" {
		return s.executeShellCommand(in.Request, in.ResponseText, report)
	}
	if !in.Request.Interaction {
		return nil
	}
	if s.deps.PromptAction == nil {
		return fmt.Errorf("shell prompt action is not configured")
	}
	action, err := s.deps.PromptAction(shell.PromptOptions{
		HasSandbox: shell.SandboxRuntimeAvailable(in.Request.SandboxRuntime),
		Risk:       report,
	})
	if err != nil {
		return err
	}
	switch action {
	case shell.ActionExecuteHost:
		return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeHost), in.ResponseText, report)
	case shell.ActionExecuteSafe:
		return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeSafe), in.ResponseText, report)
	case shell.ActionExecuteSandbox:
		return s.executeInteractiveSandboxCommand(ctx, in, report)
	case shell.ActionDescribe:
		return s.describeShellCommand(ctx, in)
	default:
		return nil
	}
}

func (s *Service) executeShellCommand(req Request, command string, report shell.RiskReport) error {
	execReq := shell.ExecutionRequest{
		Command: command,
		Mode:    req.ExecuteMode,
		Sandbox: shell.SandboxConfig{
			Runtime: req.SandboxRuntime,
			Image:   req.SandboxImage,
			Network: req.SandboxNetwork,
			Write:   req.SandboxWrite,
		},
	}
	if execReq.Mode == shell.ExecutionModeHost && report.HighRisk && !req.ForceRiskyShell {
		if !req.Interaction {
			return fmt.Errorf("refusing high-risk host shell execution without --force-risky-shell")
		}
		if s.deps.ConfirmShell == nil {
			return fmt.Errorf("shell confirmation is not configured")
		}
		ok, err := s.deps.ConfirmShell(execReq.Mode, report)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if execReq.Mode == shell.ExecutionModeSafe && report.HighRisk {
		return fmt.Errorf("safe shell execution refuses high-risk commands: %s", strings.Join(report.Reasons, "; "))
	}
	if s.deps.ExecuteShell == nil {
		return fmt.Errorf("shell execution is not configured")
	}
	return s.deps.ExecuteShell(execReq)
}

func (s *Service) executeInteractiveSandboxCommand(ctx context.Context, in renderPromptResultInput, report shell.RiskReport) error {
	command := in.ResponseText
	if !shell.HostCompatibleWithSandbox() {
		regenerated, err := s.generateSandboxCommand(ctx, in)
		if err != nil {
			return err
		}
		command = regenerated
		report = shell.AnalyzeCommand(command)
		s.deps.Printer.Header("sandbox command")
		s.deps.Printer.PrintResponse(command, false)
		if report.HighRisk {
			s.deps.Printer.Warn("High-risk shell command detected: " + strings.Join(report.Reasons, "; "))
		}
	}
	return s.executeShellCommand(withExecuteMode(in.Request, shell.ExecutionModeSandbox), command, report)
}

func (s *Service) generateSandboxCommand(ctx context.Context, in renderPromptResultInput) (string, error) {
	resp, err := s.executePrompt(ctx, in.Provider, provider.Request{
		Model:       in.ModelID,
		Messages:    []model.Message{{Role: model.RoleUser, Content: in.PromptText}},
		System:      shell.SandboxSystemPrompt(),
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func withExecuteMode(req Request, mode shell.ExecutionMode) Request {
	req.ExecuteMode = mode
	return req
}

func (s *Service) describeShellCommand(ctx context.Context, in renderPromptResultInput) error {
	describeReq := provider.Request{
		Model:       in.ModelID,
		Messages:    []model.Message{{Role: model.RoleUser, Content: "Describe what this command does: " + in.ResponseText}},
		System:      "Explain what the shell command does in plain English.",
		Temperature: in.Temperature,
		TopP:        in.TopP,
		MaxTokens:   4096,
		Stream:      in.Config.Stream && !in.Request.NoStream,
	}
	describeResponse, err := s.executePrompt(ctx, in.Provider, describeReq)
	if err != nil {
		return err
	}
	s.deps.Printer.PrintResponse(describeResponse.Content, true)
	return nil
}

type logPromptResultInput struct {
	ModelID      string
	ProviderName string
	PromptText   string
	ResponseText string
	SystemPrompt string
	SchemaValue  map[string]any
	Fragments    []string
	ChatName     string
	Completion   model.CompletionResponse
}

func (s *Service) logPromptResult(in logPromptResultInput) error {
	now := s.deps.Now()
	durationMS := int64(0)
	entry := model.LogEntry{
		Model:      in.ModelID,
		Provider:   in.ProviderName,
		Prompt:     in.PromptText,
		Response:   in.ResponseText,
		CreatedAt:  now,
		DurationMS: &durationMS,
	}
	if in.ChatName != "" {
		entry.Conversation = &in.ChatName
	}
	if in.SystemPrompt != "" {
		entry.SystemPrompt = &in.SystemPrompt
	}
	if len(in.Fragments) > 0 {
		payload, err := json.Marshal(in.Fragments)
		if err != nil {
			return err
		}
		value := string(payload)
		entry.FragmentsJSON = &value
	}
	if in.SchemaValue != nil {
		payload, err := json.Marshal(in.SchemaValue)
		if err != nil {
			return err
		}
		value := string(payload)
		entry.SchemaJSON = &value
	}
	if in.Completion.Usage != nil {
		inputTokens := int64(in.Completion.Usage.PromptTokens)
		outputTokens := int64(in.Completion.Usage.CompletionTokens)
		entry.InputTokens = &inputTokens
		entry.OutputTokens = &outputTokens
	}
	return s.deps.NewLogStore().Insert(entry)
}

func (s *Service) preparePromptRun(req Request) (promptRunContext, error) {
	cfg, err := s.deps.LoadConfig()
	if err != nil {
		return promptRunContext{}, err
	}
	promptText, err := s.appendFragments(s.preparePrompt(req.PromptText, req), req.Fragments)
	if err != nil {
		return promptRunContext{}, err
	}
	modelID, systemPrompt, temperature, renderMarkdown, err := s.resolveModelAndSystem(req, cfg)
	if err != nil {
		return promptRunContext{}, err
	}
	schemaValue, err := schema.Load(req.Schema)
	if err != nil {
		return promptRunContext{}, err
	}
	modelID = s.resolveAlias(modelID)
	providerName := provider.DetectProvider(modelID, cfg.DefaultProvider)
	systemPrompt = applySchemaFallbackInstruction(systemPrompt, schemaValue, providerName)
	apiKey, err := s.deps.ResolveAPIKey(providerName)
	if err != nil {
		return promptRunContext{}, err
	}
	prov, err := s.deps.BuildProvider(providerName, apiKey, cfg.BaseURLForProvider(providerName), time.Duration(cfg.RequestTimeout)*time.Second)
	if err != nil {
		return promptRunContext{}, err
	}
	chatName, err := s.resolveChatName(req)
	if err != nil {
		return promptRunContext{}, err
	}
	session, err := s.loadSession(chatName, systemPrompt)
	if err != nil {
		return promptRunContext{}, err
	}
	systemPrompt = effectiveSessionSystemPrompt(session, systemPrompt)
	messages := append([]model.Message{}, session.Messages...)
	messages = append(messages, model.Message{Role: model.RoleUser, Content: promptText})
	tempPtr, topPPtr := samplingPointers(temperature, req.TopP)
	return promptRunContext{
		cfg:            cfg,
		promptText:     promptText,
		modelID:        modelID,
		systemPrompt:   systemPrompt,
		renderMarkdown: renderMarkdown,
		schemaValue:    schemaValue,
		providerName:   providerName,
		prov:           prov,
		chatName:       chatName,
		session:        session,
		messages:       messages,
		tempPtr:        tempPtr,
		topPPtr:        topPPtr,
	}, nil
}

func (s *Service) persistPromptArtifacts(runCtx promptRunContext, req Request, responseText string, completion model.CompletionResponse) error {
	if runCtx.chatName == "" {
		return nil
	}
	runCtx.session.PushUser(runCtx.promptText)
	runCtx.session.PushAssistant(responseText)
	if runCtx.chatName == "temp" {
		return nil
	}
	if err := s.deps.ChatStore.Save(runCtx.session); err != nil {
		return err
	}
	return s.deps.WriteLastChat(runCtx.chatName)
}

func (s *Service) transformPromptResponse(req Request, schemaValue map[string]any, responseText string) (string, error) {
	if req.Extract {
		responseText = extractCodeBlock(responseText)
	}
	if schemaValue == nil {
		return responseText, nil
	}
	return schema.PrettyValidateResponse(schemaValue, responseText)
}

func (s *Service) finalizePromptRun(runCtx promptRunContext, req Request, responseText string, completion model.CompletionResponse, cachedHit bool) error {
	if req.Cache && !cachedHit {
		_ = s.deps.CacheStore.Set(runCtx.modelID, runCtx.systemPrompt, runCtx.messages, runCtx.schemaValue, runCtx.tempPtr, runCtx.topPPtr, responseText)
	}
	if err := s.persistPromptArtifacts(runCtx, req, responseText, completion); err != nil {
		return err
	}
	if shouldLogRequest(runCtx.cfg, req) {
		if err := s.logPromptResult(logPromptResultInput{
			ModelID:      runCtx.modelID,
			ProviderName: runCtx.providerName,
			PromptText:   runCtx.promptText,
			ResponseText: responseText,
			SystemPrompt: runCtx.systemPrompt,
			SchemaValue:  runCtx.schemaValue,
			Fragments:    req.Fragments,
			ChatName:     runCtx.chatName,
			Completion:   completion,
		}); err != nil {
			return err
		}
	}
	if req.Save == "" {
		return nil
	}
	template := store.Template{Name: req.Save, Model: runCtx.modelID, SystemPrompt: runCtx.systemPrompt}
	if runCtx.tempPtr != nil {
		template.Temperature = runCtx.tempPtr
	}
	if err := s.deps.TemplateStore.Save(template); err != nil {
		return err
	}
	s.deps.Printer.Success(fmt.Sprintf("Template %q saved.", req.Save))
	return nil
}

func shouldLogRequest(cfg config.Config, req Request) bool {
	if req.Log {
		return true
	}
	if req.NoLog {
		return false
	}
	return cfg.LogToDB
}
