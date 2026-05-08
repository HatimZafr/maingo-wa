package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatNoTools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []Choice{{
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Halo! Ada yang bisa dibantu?",
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", "test-model", 0.7)
	messages := []ChatMessage{{Role: "user", Content: "Halo"}}

	resp, err := client.Chat(context.Background(), messages, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Halo! Ada yang bisa dibantu?" {
		t.Errorf("got %q", resp.Choices[0].Message.Content)
	}
}

func TestChatWithToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{
			Choices: []Choice{{
				Message: ChatMessage{
					Role: "assistant",
					ToolCalls: []ToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: FunctionCall{
							Name:      "echo",
							Arguments: `{"text": "hello"}`,
						},
					}},
				},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", "test-model", 0.7)
	messages := []ChatMessage{{Role: "user", Content: "Echo hello"}}
	tools := []ToolDefinition{{
		Type: "function",
		Function: FunctionDef{
			Name:        "echo",
			Description: "Echo text",
		},
	}}

	resp, err := client.Chat(context.Background(), messages, tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Choices[0].Message.ToolCalls))
	}
	if resp.Choices[0].Message.ToolCalls[0].Function.Name != "echo" {
		t.Errorf("got %q", resp.Choices[0].Message.ToolCalls[0].Function.Name)
	}
}

func TestChatHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", "test-model", 0.7)
	_, err := client.Chat(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestChatUnreachable(t *testing.T) {
	client := NewClient("http://127.0.0.1:9999", "key", "model", 0.7)
	_, err := client.Chat(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for unreachable")
	}
}
