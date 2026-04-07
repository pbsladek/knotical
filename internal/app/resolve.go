package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/ingest"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

func (s *Service) resolveChatName(req Request) (string, error) {
	chatName := req.Chat
	if req.ContinueLast && chatName == "" {
		return s.deps.ReadLastChat()
	}
	return chatName, nil
}

func (s *Service) loadSession(chatName string, systemPrompt string) (model.ChatSession, error) {
	if chatName == "" {
		return model.ChatSession{}, nil
	}
	session, err := s.deps.ChatStore.LoadOrCreate(chatName)
	if err != nil {
		return model.ChatSession{}, err
	}
	persistSessionSystemPrompt(&session, systemPrompt)
	return session, nil
}

func effectiveSessionSystemPrompt(session model.ChatSession, systemPrompt string) string {
	if systemPrompt != "" {
		return systemPrompt
	}
	for _, message := range session.Messages {
		if message.Role == model.RoleSystem {
			return message.Content
		}
	}
	return ""
}

func (s *Service) appendFragments(prompt string, names []string) (string, error) {
	if len(names) == 0 {
		return prompt, nil
	}
	parts := []string{prompt}
	for _, name := range names {
		fragment, err := s.deps.FragmentStore.Load(name)
		if err != nil {
			return "", err
		}
		parts = append(parts, fragment.Content)
	}
	return strings.Join(parts, "\n\n"), nil
}

func (s *Service) resolveAlias(modelID string) string {
	if s.deps.AliasStore == nil {
		return modelID
	}
	aliases, err := s.deps.AliasStore.Load()
	if err != nil {
		return modelID
	}
	if resolved, ok := aliases[modelID]; ok {
		return resolved
	}
	return modelID
}

func (s *Service) resolveModelAndSystem(req Request, cfg config.Config) (string, string, float64, bool, error) {
	state, err := s.resolveRequestState(req, cfg)
	if err != nil {
		return "", "", 0, false, err
	}
	return state.modelID, state.systemPrompt, state.temperature, state.renderMarkdown, nil
}

type requestState struct {
	providerName   string
	modelID        string
	systemPrompt   string
	temperature    float64
	renderMarkdown bool
}

func (s *Service) resolveRequestState(req Request, cfg config.Config) (requestState, error) {
	providerCfg := cfg.ProviderSettings()
	state := requestState{
		providerName:   providerCfg.DefaultProvider,
		modelID:        providerCfg.DefaultModel,
		systemPrompt:   req.System,
		temperature:    cfg.Temperature,
		renderMarkdown: cfg.PrettifyMarkdown,
	}
	if req.Model != "" {
		state.modelID = req.Model
	}
	if req.Temperature != 0 {
		state.temperature = req.Temperature
	}
	if req.Template != "" {
		template, err := s.deps.TemplateStore.Load(req.Template)
		if err != nil {
			return requestState{}, err
		}
		applyTemplateState(&state, req, template)
	}
	if err := s.applyModeState(&state, req, cfg); err != nil {
		return requestState{}, err
	}
	if req.NoMD || req.Extract {
		state.renderMarkdown = false
	}
	resolvedProvider, resolvedModel, err := provider.ResolveModel(s.resolveAlias(state.modelID), req.Provider, providerCfg.DefaultProvider)
	if err != nil {
		return requestState{}, err
	}
	state.providerName = resolvedProvider
	state.modelID = resolvedModel
	return state, nil
}

func applyTemplateState(state *requestState, req Request, template store.Template) {
	if req.Model == "" && template.Model != "" {
		state.modelID = template.Model
	}
	if state.systemPrompt == "" {
		state.systemPrompt = template.SystemPrompt
	}
	if req.Temperature == 0 && template.Temperature != nil {
		state.temperature = *template.Temperature
	}
}

func (s *Service) applyModeState(state *requestState, req Request, cfg config.Config) error {
	logCfg := cfg.LogAnalysisSettings()
	switch {
	case req.System != "":
		state.systemPrompt = req.System
	case req.Role != "":
		role, err := s.deps.RoleStore.Load(req.Role)
		if err != nil {
			return err
		}
		state.systemPrompt = role.SystemPrompt
		state.renderMarkdown = role.PrettifyMarkdown
	case req.Shell:
		if req.ExecuteMode == shell.ExecutionModeSandbox {
			state.systemPrompt = shell.SandboxSystemPrompt()
		} else {
			state.systemPrompt = shell.ShellSystemPrompt()
		}
		state.renderMarkdown = false
	case req.AnalyzeLogs:
		state.systemPrompt = logAnalysisSystemPrompt(logCfg)
		state.renderMarkdown = logCfg.Markdown
	case req.Code:
		state.systemPrompt = "Provide only code as output without any explanation or markdown formatting. Do not add backticks or language tags around the code."
		state.renderMarkdown = false
	case req.DescribeShell:
		state.systemPrompt = "Explain what the provided shell command does in plain English. Be concise and technical."
		state.renderMarkdown = false
	}
	return nil
}

func logAnalysisSystemPrompt(cfg config.LogAnalysisSettings) string {
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		return strings.TrimSpace(cfg.SystemPrompt)
	}
	return "You are analyzing operational logs. Be concise and technical. Identify the most likely root cause, cite the strongest evidence from the logs, and suggest the next diagnostic or remediation steps. If the logs are inconclusive, say what is missing."
}

func (s *Service) executePrompt(ctx context.Context, prov provider.Provider, req provider.Request) (model.CompletionResponse, error) {
	var responseText string
	var usage *model.TokenUsage
	if req.Stream {
		err := prov.Stream(ctx, req, func(chunk model.StreamChunk) error {
			if chunk.Delta != "" {
				s.deps.Printer.Print(chunk.Delta)
				responseText += chunk.Delta
			}
			if chunk.Usage != nil {
				usage = chunk.Usage
			}
			return nil
		})
		s.deps.Printer.Println("")
		return model.CompletionResponse{Content: responseText, Model: req.Model, Usage: usage}, err
	}
	return prov.Complete(ctx, req)
}

func defaultResolveAPIKey(providerName string) (string, error) {
	if providerName == "ollama" || strings.HasSuffix(providerName, "-cli") {
		return "", nil
	}
	return store.NewKeyManager(config.KeysFilePath()).Require(providerName)
}

func applyShellDefaults(req Request, cfg config.Config) Request {
	if req.Shell {
		req = applyShellExecutionDefaults(req, cfg.ShellSettings())
	}
	return applyInputDefaults(req, cfg)
}

func applyShellExecutionDefaults(req Request, shellCfg config.ShellSettings) Request {
	req.ExecuteMode = defaultExecutionMode(req.ExecuteMode, shellCfg.ExecuteMode)
	req.SandboxRuntime = defaultString(req.SandboxRuntime, shellCfg.Runtime)
	req.SandboxImage = defaultString(req.SandboxImage, shellCfg.Image)
	req.SandboxNetwork = defaultBool(req.SandboxNetwork, shellCfg.Network)
	req.SandboxWrite = defaultBool(req.SandboxWrite, shellCfg.Write)
	return req
}

func applyInputDefaults(req Request, cfg config.Config) Request {
	req = applyIngestDefaults(req, cfg.IngestSettings())
	req = applySummarizeDefaults(req, cfg.SummarizeSettings())
	return applyAnalyzeLogDefaults(req, cfg.LogAnalysisSettings())
}

func applyIngestDefaults(req Request, ingestCfg config.IngestSettings) Request {
	req.MaxInputBytes = defaultInt(req.MaxInputBytes, ingestCfg.MaxInputBytes)
	req.MaxInputLines = defaultInt(req.MaxInputLines, ingestCfg.MaxInputLines)
	req.MaxInputTokens = defaultInt(req.MaxInputTokens, ingestCfg.MaxInputTokens)
	req.InputReduction = defaultString(req.InputReduction, ingestCfg.InputReductionMode)
	req.HeadLines = defaultInt(req.HeadLines, ingestCfg.DefaultHeadLines)
	req.TailLines = defaultInt(req.TailLines, ingestCfg.DefaultTailLines)
	req.SampleLines = defaultInt(req.SampleLines, ingestCfg.DefaultSampleLines)
	return req
}

func applySummarizeDefaults(req Request, summarizeCfg config.SummarizeSettings) Request {
	if req.SummarizeChunkTokens == 0 && summarizeCfg.ChunkTokens > 0 {
		req.SummarizeChunkTokens = summarizeCfg.ChunkTokens
	}
	if req.SummarizeIntermediateModel == "" && strings.TrimSpace(summarizeCfg.IntermediateModel) != "" {
		req.SummarizeIntermediateModel = strings.TrimSpace(summarizeCfg.IntermediateModel)
	}
	return req
}

func applyAnalyzeLogDefaults(req Request, logCfg config.LogAnalysisSettings) Request {
	if !req.AnalyzeLogs {
		return req
	}
	if req.Profile == "" && strings.TrimSpace(logCfg.DefaultProfile) != "" {
		req.Profile = strings.TrimSpace(logCfg.DefaultProfile)
	}
	if req.Schema == "" && strings.TrimSpace(logCfg.Schema) != "" {
		req.Schema = strings.TrimSpace(logCfg.Schema)
	}
	if req.StdinLabel == "" || req.StdinLabel == "input" {
		req.StdinLabel = "logs"
	}
	return req
}

func defaultInt(current int, fallback int) int {
	if current == 0 && fallback > 0 {
		return fallback
	}
	return current
}

func defaultString(current string, fallback string) string {
	if current == "" && fallback != "" {
		return fallback
	}
	return current
}

func defaultBool(current bool, fallback bool) bool {
	if !current && fallback {
		return true
	}
	return current
}

func defaultExecutionMode(current shell.ExecutionMode, fallback string) shell.ExecutionMode {
	if current == "" && fallback != "" {
		return shell.ExecutionMode(fallback)
	}
	return current
}

func requestPipelineOptions(req Request) ingest.PipelineOptions {
	pipeline := req.PipelineInput
	shorthands := make([]string, 0, 4)
	if pipeline.Clean {
		shorthands = append(shorthands, "clean")
	}
	if pipeline.Dedupe {
		shorthands = append(shorthands, "dedupe")
	}
	if pipeline.Unique {
		shorthands = append(shorthands, "unique")
	}
	if pipeline.K8s {
		shorthands = append(shorthands, "k8s")
	}
	return ingest.PipelineOptions{
		Profile:    pipeline.Profile,
		Shorthands: shorthands,
		Transforms: append([]string(nil), pipeline.Transforms...),
		NoPipeline: pipeline.NoPipeline,
	}
}

func (s *Service) buildConfiguredProvider(cfg config.Config, runtime config.ProviderRuntime) (provider.Provider, string, error) {
	if runtime.Transport == "cli" {
		prov, err := s.deps.BuildCLIProvider(runtime.Name, provider.CLIConfig(runtime.CLI))
		if err != nil {
			return nil, "", err
		}
		return prov, runtime.Name, nil
	}
	providerCfg := cfg.ProviderSettings()
	apiKey, err := s.deps.ResolveAPIKey(runtime.Name)
	if err != nil {
		return nil, "", err
	}
	prov, err := s.deps.BuildProvider(runtime.Name, apiKey, runtime.BaseURL, providerCfg.RequestTimeout)
	if err != nil {
		return nil, "", err
	}
	return prov, runtime.Name, nil
}

func persistSessionSystemPrompt(session *model.ChatSession, systemPrompt string) {
	if systemPrompt == "" {
		return
	}
	for idx, message := range session.Messages {
		if message.Role == model.RoleSystem {
			session.Messages[idx].Content = systemPrompt
			session.UpdatedAt = time.Now().UTC()
			return
		}
	}
	session.PushSystem(systemPrompt)
}

func applySchemaFallbackInstruction(systemPrompt string, schemaValue map[string]any, caps config.ProviderCapabilities) string {
	if schemaValue == nil || caps.NativeSchema {
		return systemPrompt
	}
	schemaJSON, _ := json.Marshal(schemaValue)
	instruction := fmt.Sprintf("Respond with valid JSON matching this schema: %s. No other text.", string(schemaJSON))
	if systemPrompt != "" {
		return systemPrompt + "\n\n" + instruction
	}
	return instruction
}

func samplingPointers(temperature float64, topP float64) (*float64, *float64) {
	var tempPtr *float64
	if temperature > 0 {
		tempPtr = &temperature
	}
	var topPPtr *float64
	if topP != 1 {
		topPPtr = &topP
	}
	return tempPtr, topPPtr
}

func extractCodeBlock(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	inBlock := false
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inBlock {
				break
			}
			inBlock = true
			continue
		}
		if inBlock {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return text
	}
	return strings.Join(lines, "\n")
}

func (s *Service) preparePrompt(prompt string, req Request) string {
	if !req.DescribeShell {
		return prompt
	}
	return "Explain what this shell command does:\n\n" + strings.TrimSpace(prompt)
}
