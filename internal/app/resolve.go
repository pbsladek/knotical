package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/config"
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
	modelID        string
	systemPrompt   string
	temperature    float64
	renderMarkdown bool
}

func (s *Service) resolveRequestState(req Request, cfg config.Config) (requestState, error) {
	state := requestState{
		modelID:        cfg.DefaultModel,
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
	if err := s.applyModeState(&state, req); err != nil {
		return requestState{}, err
	}
	if req.NoMD || req.Extract {
		state.renderMarkdown = false
	}
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

func (s *Service) applyModeState(state *requestState, req Request) error {
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
	case req.Code:
		state.systemPrompt = "Provide only code as output without any explanation or markdown formatting. Do not add backticks or language tags around the code."
		state.renderMarkdown = false
	case req.DescribeShell:
		state.systemPrompt = "Explain what the provided shell command does in plain English. Be concise and technical."
		state.renderMarkdown = false
	}
	return nil
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
	if providerName == "ollama" {
		return "", nil
	}
	return store.NewKeyManager(config.KeysFilePath()).Require(providerName)
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

func applySchemaFallbackInstruction(systemPrompt string, schemaValue map[string]any, providerName string) string {
	if schemaValue == nil || providerName == "openai" || providerName == "gemini" {
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
