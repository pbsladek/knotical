package model

import "time"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionResponse struct {
	Content string      `json:"content"`
	Model   string      `json:"model"`
	Usage   *TokenUsage `json:"usage,omitempty"`
}

type StreamChunk struct {
	Delta string      `json:"delta"`
	Usage *TokenUsage `json:"usage,omitempty"`
	Done  bool        `json:"done"`
}

type ChatSession struct {
	Name      string    `json:"name"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewChatSession(name string) ChatSession {
	now := time.Now().UTC()
	return ChatSession{
		Name:      name,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (s *ChatSession) Push(msg Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now().UTC()
}

func (s *ChatSession) PushUser(content string) {
	s.Push(Message{Role: RoleUser, Content: content})
}

func (s *ChatSession) PushAssistant(content string) {
	s.Push(Message{Role: RoleAssistant, Content: content})
}

func (s *ChatSession) PushSystem(content string) {
	s.Push(Message{Role: RoleSystem, Content: content})
}

type LogEntry struct {
	ID            string    `json:"id"`
	Conversation  *string   `json:"conversation,omitempty"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	SystemPrompt  *string   `json:"system_prompt,omitempty"`
	SchemaJSON    *string   `json:"schema_json,omitempty"`
	FragmentsJSON *string   `json:"fragments_json,omitempty"`
	ReductionJSON *string   `json:"reduction_json,omitempty"`
	Prompt        string    `json:"prompt"`
	Response      string    `json:"response"`
	InputTokens   *int64    `json:"input_tokens,omitempty"`
	OutputTokens  *int64    `json:"output_tokens,omitempty"`
	DurationMS    *int64    `json:"duration_ms,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type ReductionMetadata struct {
	Profile           string   `json:"profile,omitempty"`
	Transforms        []string `json:"transforms,omitempty"`
	Shorthands        []string `json:"shorthands,omitempty"`
	StdinLabel        string   `json:"stdin_label,omitempty"`
	Mode              string   `json:"mode,omitempty"`
	OriginalBytes     int      `json:"original_bytes,omitempty"`
	OriginalLines     int      `json:"original_lines,omitempty"`
	OriginalTokens    int      `json:"original_tokens,omitempty"`
	FinalBytes        int      `json:"final_bytes,omitempty"`
	FinalLines        int      `json:"final_lines,omitempty"`
	FinalTokens       int      `json:"final_tokens,omitempty"`
	DroppedLines      int      `json:"dropped_lines,omitempty"`
	UniqueGroups      int      `json:"unique_groups,omitempty"`
	PipelineApplied   bool     `json:"pipeline_applied,omitempty"`
	Steps             []string `json:"steps,omitempty"`
	Summarized        bool     `json:"summarized,omitempty"`
	SummaryChunks     int      `json:"summary_chunks,omitempty"`
	IntermediateModel string   `json:"intermediate_model,omitempty"`
}
