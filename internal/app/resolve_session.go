package app

import (
	"strings"
	"time"

	"github.com/pbsladek/knotical/internal/model"
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

func persistSessionSystemPrompt(session *model.ChatSession, systemPrompt string) {
	if strings.TrimSpace(systemPrompt) == "" {
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
