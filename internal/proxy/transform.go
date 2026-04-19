package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Chat Completions request → Responses request

type chatCompletionsRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessage   `json:"messages"`
	Stream              bool            `json:"stream"`
	Temperature         *float64        `json:"temperature,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	Tools               json.RawMessage `json:"tools,omitempty"`
	ToolChoice          json.RawMessage `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool           `json:"parallel_tool_calls,omitempty"`
	Stop                json.RawMessage `json:"stop,omitempty"`
	Seed                *int            `json:"seed,omitempty"`
	User                string          `json:"user,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responsesRequest struct {
	Model        string          `json:"model"`
	Instructions string          `json:"instructions"`
	Input        json.RawMessage `json:"input"`
	Stream       bool            `json:"stream"`
	Store        bool            `json:"store"`
	Tools        json.RawMessage `json:"tools,omitempty"`
	ToolChoice   json.RawMessage `json:"tool_choice,omitempty"`
	Reasoning    *reasoning      `json:"reasoning,omitempty"`
	Text         json.RawMessage `json:"text,omitempty"`
}

type reasoning struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
}

func chatCompletionsToResponses(body []byte) ([]byte, error) {
	var req chatCompletionsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("failed to parse chat completions request: %w", err)
	}

	rr := responsesRequest{
		Model:      req.Model,
		Stream:     true, // Codex API requires stream=true
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
		Reasoning:  &reasoning{Effort: "medium", Summary: "auto"},
	}

	// response_format → text format
	if req.ResponseFormat != nil {
		rr.Text = req.ResponseFormat
	}

	// Extract system message as instructions, rest as input
	var inputMessages []chatMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if rr.Instructions != "" {
				rr.Instructions += "\n"
			}
			rr.Instructions += msg.Content
		} else {
			inputMessages = append(inputMessages, msg)
		}
	}
	if rr.Instructions == "" {
		rr.Instructions = "You are a helpful assistant."
	}

	inputJSON, err := json.Marshal(inputMessages)
	if err != nil {
		return nil, err
	}
	rr.Input = inputJSON

	return json.Marshal(rr)
}

// Responses response → Chat Completions response (non-streaming)

type responsesResponse struct {
	ID     string           `json:"id"`
	Model  string           `json:"model"`
	Output []responseOutput `json:"output"`
	Usage  *responsesUsage  `json:"usage,omitempty"`
}

type responseOutput struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

type responseContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type chatCompletionsResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []chatChoice       `json:"choices"`
	Usage   *chatUsage         `json:"usage,omitempty"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func responsesToChatCompletions(body []byte) ([]byte, error) {
	var rr responsesResponse
	if err := json.Unmarshal(body, &rr); err != nil {
		return nil, fmt.Errorf("failed to parse responses: %w", err)
	}

	// Extract text content from output
	var content string
	for _, out := range rr.Output {
		if out.Type == "message" && out.Content != nil {
			var parts []responseContentPart
			if err := json.Unmarshal(out.Content, &parts); err == nil {
				for _, p := range parts {
					if p.Type == "output_text" {
						content += p.Text
					}
				}
			}
		}
	}

	resp := chatCompletionsResponse{
		ID:      rr.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   rr.Model,
		Choices: []chatChoice{
			{
				Index:        0,
				Message:      chatMessage{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
	}

	if rr.Usage != nil {
		resp.Usage = &chatUsage{
			PromptTokens:     rr.Usage.InputTokens,
			CompletionTokens: rr.Usage.OutputTokens,
			TotalTokens:      rr.Usage.TotalTokens,
		}
	}

	return json.Marshal(resp)
}

// SSE streaming: Responses events → Chat Completions chunk events

type chatCompletionsChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []chatChunkChoice `json:"choices"`
}

type chatChunkChoice struct {
	Index        int              `json:"index"`
	Delta        chatChunkDelta   `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type chatChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

func makeSSETransformer(model string) func(eventType, data string) (string, string, bool) {
	id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	sentRole := false

	return func(eventType, data string) (string, string, bool) {
		switch eventType {
		case "response.output_item.added":
			if !sentRole {
				sentRole = true
				chunk := chatCompletionsChunk{
					ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
					Choices: []chatChunkChoice{{Index: 0, Delta: chatChunkDelta{Role: "assistant"}}},
				}
				out, _ := json.Marshal(chunk)
				return "", string(out), true
			}
			return "", "", false

		case "response.output_text.delta":
			var ev struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil || ev.Delta == "" {
				return "", "", false
			}
			chunk := chatCompletionsChunk{
				ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
				Choices: []chatChunkChoice{{Index: 0, Delta: chatChunkDelta{Content: ev.Delta}}},
			}
			out, _ := json.Marshal(chunk)
			return "", string(out), true

		case "response.completed":
			stop := "stop"
			chunk := chatCompletionsChunk{
				ID: id, Object: "chat.completion.chunk", Created: created, Model: model,
				Choices: []chatChunkChoice{{Index: 0, Delta: chatChunkDelta{}, FinishReason: &stop}},
			}
			out, _ := json.Marshal(chunk)
			return "", string(out), true

		default:
			return "", "", false
		}
	}
}

// collectSSE reads a streaming SSE response and collects the full text content and usage.
func collectSSE(r io.Reader) (string, *chatUsage) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var content strings.Builder
	var usage *chatUsage

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var ev struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
					TotalTokens  int `json:"total_tokens"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "response.output_text.delta":
			content.WriteString(ev.Delta)
		case "response.completed":
			u := ev.Response.Usage
			if u.TotalTokens > 0 {
				usage = &chatUsage{
					PromptTokens:     u.InputTokens,
					CompletionTokens: u.OutputTokens,
					TotalTokens:      u.TotalTokens,
				}
			}
		}
	}

	return content.String(), usage
}

func buildChatCompletionsResponse(model, content string, usage *chatUsage) chatCompletionsResponse {
	return chatCompletionsResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []chatChoice{
			{
				Index:        0,
				Message:      chatMessage{Role: "assistant", Content: content},
				FinishReason: "stop",
			},
		},
		Usage: usage,
	}
}
