package app

import (
	"encoding/json"
	"fmt"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/model"
	"github.com/pbsladek/knotical/internal/schema"
	"github.com/pbsladek/knotical/internal/store"
)

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
	Reduction    *model.ReductionMetadata
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
	if in.Reduction != nil {
		payload, err := json.Marshal(in.Reduction)
		if err != nil {
			return err
		}
		value := string(payload)
		entry.ReductionJSON = &value
	}
	if in.Completion.Usage != nil {
		inputTokens := int64(in.Completion.Usage.PromptTokens)
		outputTokens := int64(in.Completion.Usage.CompletionTokens)
		entry.InputTokens = &inputTokens
		entry.OutputTokens = &outputTokens
	}
	return s.deps.NewLogStore().Insert(entry)
}

func (s *Service) persistPromptArtifacts(runCtx promptRunContext, req Request, responseText string, completion model.CompletionResponse) error {
	if runCtx.chatName == "" {
		return nil
	}
	runCtx.session.PushUser(runCtx.promptText)
	runCtx.session.PushAssistant(responseText)
	if runCtx.chatName != "temp" {
		if err := s.deps.ChatStore.Save(runCtx.session); err != nil {
			return err
		}
		if err := s.deps.WriteLastChat(runCtx.chatName); err != nil {
			return err
		}
	}
	_ = completion
	return nil
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
			Reduction:    runCtx.reduction,
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
