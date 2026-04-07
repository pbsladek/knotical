package app

import (
	"strings"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/provider"
	"github.com/pbsladek/knotical/internal/shell"
	"github.com/pbsladek/knotical/internal/store"
)

type requestState struct {
	providerName   string
	modelID        string
	systemPrompt   string
	temperature    float64
	renderMarkdown bool
}

func (s *Service) resolveModelAndSystem(req Request, cfg config.Config) (string, string, float64, bool, error) {
	state, err := s.resolveRequestState(req, cfg)
	if err != nil {
		return "", "", 0, false, err
	}
	return state.modelID, state.systemPrompt, state.temperature, state.renderMarkdown, nil
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
