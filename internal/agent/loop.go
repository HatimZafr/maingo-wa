package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"maingo/internal/llm"
	"maingo/internal/session"
	"maingo/internal/tool"
)

const maxInputLength = 4000

type Config struct {
	SystemPrompt string
	MaxRounds    int
	WA           WhatsAppSender
	LLM          LLMClient
	Sessions     *session.Store
	Tools        *tool.Registry
}

type WhatsAppSender interface {
	SendReply(ctx context.Context, phone, text string) error
}

type LLMClient interface {
	Chat(ctx context.Context, messages []llm.ChatMessage, tools []llm.ToolDefinition) (*llm.ChatResponse, error)
}

type Agent struct {
	systemPrompt string
	maxRounds    int
	wa           WhatsAppSender
	llm          LLMClient
	sessions     *session.Store
	tools        *tool.Registry
}

func New(cfg Config) *Agent {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 5
	}
	return &Agent{
		systemPrompt: cfg.SystemPrompt,
		maxRounds:    cfg.MaxRounds,
		wa:           cfg.WA,
		llm:          cfg.LLM,
		sessions:     cfg.Sessions,
		tools:        cfg.Tools,
	}
}

func (a *Agent) HandleMessage(ctx context.Context, senderPhone string, messageText string) error {
	if len(messageText) > maxInputLength {
		return a.wa.SendReply(ctx, senderPhone, "Maaf, pesan terlalu panjang (maks 4000 karakter).")
	}

	history, err := a.sessions.Load(senderPhone)
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}

	llmMessages := buildMessages(a.systemPrompt, history, messageText)

	toolDefs := make([]llm.ToolDefinition, 0)
	for _, td := range a.tools.All() {
		toolDefs = append(toolDefs, toolDefToLLM(td))
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	for round := 0; round < a.maxRounds; round++ {
		resp, err := a.llm.Chat(ctx, llmMessages, toolDefs)
		if err != nil {
			_ = a.wa.SendReply(ctx, senderPhone, "Maaf, ada kendala teknis. Coba lagi nanti.")
			return fmt.Errorf("LLM call: %w", err)
		}

		msg := resp.Choices[0].Message
		llmMessages = append(llmMessages, msg)

		if len(msg.ToolCalls) == 0 {
			history = appendSessionMessages(history, messageText, msg.Content)
			if err := a.sessions.Save(senderPhone, history); err != nil {
				return fmt.Errorf("save session: %w", err)
			}

			reply := truncate(msg.Content, 4096)
			return a.wa.SendReply(ctx, senderPhone, reply)
		}

		// Execute tool calls
		for _, tc := range msg.ToolCalls {
			result, err := a.tools.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}
			result = truncate(result, 64<<10) // 64KB

			llmMessages = append(llmMessages, llm.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max rounds reached - send last response if available
	lastMsg := llmMessages[len(llmMessages)-1]
	if lastMsg.Content != "" {
		history = appendSessionMessages(history, messageText, lastMsg.Content)
		_ = a.sessions.Save(senderPhone, history)
		return a.wa.SendReply(ctx, senderPhone, truncate(lastMsg.Content, 4096))
	}

	return a.wa.SendReply(ctx, senderPhone, "Maaf, percakapan terlalu panjang. Silakan ulangi pertanyaan Anda.")
}

func buildMessages(systemPrompt string, history []session.Message, userText string) []llm.ChatMessage {
	var msgs []llm.ChatMessage

	if systemPrompt != "" {
		msgs = append(msgs, llm.ChatMessage{Role: "system", Content: systemPrompt})
	}

	for _, h := range history {
		msgs = append(msgs, sessionToLLM(h))
	}

	msgs = append(msgs, llm.ChatMessage{Role: "user", Content: userText})

	return msgs
}

func sessionToLLM(m session.Message) llm.ChatMessage {
	return llm.ChatMessage{
		Role:       m.Role,
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
	}
}

func appendSessionMessages(history []session.Message, userText, assistantText string) []session.Message {
	history = append(history, session.Message{Role: "user", Content: userText})
	history = append(history, session.Message{Role: "assistant", Content: assistantText})
	return history
}

func toolDefToLLM(td tool.ToolDef) llm.ToolDefinition {
	props := make(map[string]llm.PropertyDef)
	for k, v := range td.Parameters.Properties {
		props[k] = llm.PropertyDef{
			Type:        v.Type,
			Description: v.Description,
			Enum:        v.Enum,
		}
	}
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        td.Name,
			Description: td.Description,
			Parameters: llm.ParameterSchema{
				Type:       td.Parameters.Type,
				Properties: props,
				Required:   td.Parameters.Required,
			},
		},
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// json package needed by tool.Execute which calls parseArgs
var _ = json.Valid
