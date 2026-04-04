package app

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/provider"
)

func (s *Service) RunRepl(ctx context.Context, req Request) error {
	runCtx, err := s.prepareReplRun(req)
	if err != nil {
		return err
	}
	s.deps.Printer.Header(fmt.Sprintf("Session %q - type 'exit' to quit, '\"\"\"' for multiline input", req.Repl))
	reader := bufio.NewReader(s.deps.Stdin)

	for {
		promptText, err := readReplPrompt(reader, s.deps.Printer)
		if err != nil {
			return err
		}
		if promptText == "" {
			continue
		}
		if isReplExit(promptText) {
			break
		}
		if err := s.runReplTurn(ctx, &runCtx, req, promptText); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) prepareReplRun(req Request) (replRunContext, error) {
	cfg, err := s.deps.LoadConfig()
	if err != nil {
		return replRunContext{}, err
	}
	modelID, systemPrompt, temperature, renderMarkdown, err := s.resolveModelAndSystem(req, cfg)
	if err != nil {
		return replRunContext{}, err
	}
	modelID = s.resolveAlias(modelID)
	providerName := provider.DetectProvider(modelID, cfg.DefaultProvider)
	apiKey, err := s.deps.ResolveAPIKey(providerName)
	if err != nil {
		return replRunContext{}, err
	}
	prov, err := s.deps.BuildProvider(providerName, apiKey, cfg.BaseURLForProvider(providerName), time.Duration(cfg.RequestTimeout)*time.Second)
	if err != nil {
		return replRunContext{}, err
	}
	session, err := s.deps.ChatStore.LoadOrCreate(req.Repl)
	if err != nil {
		return replRunContext{}, err
	}
	persistSessionSystemPrompt(&session, systemPrompt)
	systemPrompt = effectiveSessionSystemPrompt(session, systemPrompt)
	tempPtr, topPPtr := samplingPointers(temperature, req.TopP)
	return replRunContext{
		cfg:            cfg,
		modelID:        modelID,
		systemPrompt:   systemPrompt,
		renderMarkdown: renderMarkdown,
		providerName:   providerName,
		prov:           prov,
		session:        session,
		tempPtr:        tempPtr,
		topPPtr:        topPPtr,
	}, nil
}

func (s *Service) runReplTurn(ctx context.Context, runCtx *replRunContext, req Request, promptText string) error {
	runCtx.session.PushUser(promptText)
	execReq := provider.Request{
		Model:       runCtx.modelID,
		Messages:    runCtx.session.Messages,
		System:      runCtx.systemPrompt,
		Temperature: runCtx.tempPtr,
		TopP:        runCtx.topPPtr,
		MaxTokens:   4096,
		Stream:      runCtx.cfg.Stream && !req.NoStream && !runCtx.renderMarkdown,
	}
	completion, err := s.executePrompt(ctx, runCtx.prov, execReq)
	if err != nil {
		return err
	}
	if !execReq.Stream {
		s.deps.Printer.PrintResponse(completion.Content, runCtx.renderMarkdown)
	}
	runCtx.session.PushAssistant(completion.Content)
	if shouldLogRequest(runCtx.cfg, req) {
		if err := s.logPromptResult(logPromptResultInput{
			ModelID:      runCtx.modelID,
			ProviderName: runCtx.providerName,
			PromptText:   promptText,
			ResponseText: completion.Content,
			SystemPrompt: runCtx.systemPrompt,
			ChatName:     req.Repl,
			Completion:   completion,
		}); err != nil {
			return err
		}
	}
	if req.Repl == "temp" {
		return nil
	}
	if err := s.deps.ChatStore.Save(runCtx.session); err != nil {
		return err
	}
	return s.deps.WriteLastChat(req.Repl)
}

func readReplPrompt(reader *bufio.Reader, printer *output.Printer) (string, error) {
	printer.Prompt("> ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line != "\"\"\"" {
		return line, nil
	}
	lines := []string{}
	for {
		printer.Prompt("| ")
		blockLine, err := reader.ReadString('\n')
		if err != nil && blockLine == "" {
			return "", err
		}
		blockLine = strings.TrimRight(blockLine, "\r\n")
		if strings.TrimSpace(blockLine) == "\"\"\"" {
			break
		}
		lines = append(lines, blockLine)
	}
	return strings.Join(lines, "\n"), nil
}

func isReplExit(input string) bool {
	switch input {
	case "exit", "quit", "/exit", "/quit":
		return true
	default:
		return false
	}
}
