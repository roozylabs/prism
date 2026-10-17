package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type AnthropicAdapter struct{}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{}
}

func (a *AnthropicAdapter) BuildRequest(baseURL, apiKey string, req *ProviderRequest) (*http.Request, error) {
	var systemPrompt string
	var messages []map[string]interface{}

	for _, msg := range req.Messages {
		if role, ok := msg["role"].(string); ok && role == "system" {
			if content, ok := msg["content"].(string); ok {
				systemPrompt = content
			}
			continue
		}
		messages = append(messages, msg)
	}

	body := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   req.Stream,
	}
	if len(req.Tools) > 0 {
		openAiTools := ConvertGenericToolsToOpenAI(req.Tools)
		body["tools"] = ConvertOpenAIToolsToAnthropic(openAiTools)
	}
	if systemPrompt != "" {
		body["system"] = systemPrompt
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		body["temperature"] = req.Temperature
	}
	for k, v := range req.Extra {
		body[k] = v
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/messages"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	return httpReq, nil
}

func (a *AnthropicAdapter) ParseResponse(body io.Reader) (*ProviderResponse, error) {
	var raw struct {
		ID         string        `json:"id"`
		Model      string        `json:"model"`
		Content    []interface{} `json:"content"`
		StopReason string        `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	resp := &ProviderResponse{
		ID:    raw.ID,
		Model: raw.Model,
		Usage: Usage{
			PromptTokens:     raw.Usage.InputTokens,
			CompletionTokens: raw.Usage.OutputTokens,
			TotalTokens:      raw.Usage.InputTokens + raw.Usage.OutputTokens,
		},
	}

	if raw.Error != nil {
		resp.Error = &ProviderError{
			Type:    raw.Error.Type,
			Message: raw.Error.Message,
		}
		return resp, nil
	}

	var textContent string
	for _, block := range raw.Content {
		if blockMap, ok := block.(map[string]interface{}); ok {
			if blockType, _ := blockMap["type"].(string); blockType == "text" {
				if t, ok := blockMap["text"].(string); ok {
					textContent += t
				}
			}
		}
	}

	toolCalls := NormalizeAnthropicToolUse(raw.Content)

	msgMap := map[string]interface{}{
		"role":    "assistant",
		"content": textContent,
	}
	if len(toolCalls) > 0 {
		msgMap["tool_calls"] = toolCalls
	}

	finishReason := mapAnthropicStopReasonToOpenAI(raw.StopReason)
	if len(toolCalls) > 0 && (finishReason == "" || finishReason == "stop") {
		finishReason = "tool_calls"
	}

	resp.Choices = append(resp.Choices, Choice{
		Index:        0,
		Message:      msgMap,
		FinishReason: finishReason,
	})

	return resp, nil
}

func mapAnthropicStopReasonToOpenAI(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_calls"
	case "max_tokens":
		return "length"
	default:
		if reason != "" {
			return reason
		}
		return "stop"
	}
}

func (a *AnthropicAdapter) ParseStreamChunk(line []byte) (*ProviderResponse, bool) {
	lineStr := strings.TrimSpace(string(line))
	if !strings.HasPrefix(lineStr, "data:") {
		return nil, false
	}
	data := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))

	// Anthropic puts the input count under message_start's "message", the
	// output count under message_delta's top-level "usage" (cumulative), and
	// failures in an "error" event.
	var event struct {
		Type    string          `json:"type"`
		Delta   json.RawMessage `json:"delta"`
		Message struct {
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, false
	}

	switch event.Type {
	case "message_start":
		return &ProviderResponse{
			Usage: Usage{PromptTokens: event.Message.Usage.InputTokens},
		}, false

	case "error":
		msg := event.Error.Message
		if msg == "" {
			msg = "upstream reported an error"
		}
		return &ProviderResponse{
			Error: &ProviderError{Type: event.Error.Type, Message: msg},
		}, false

	case "content_block_delta":
		var delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		_ = json.Unmarshal(event.Delta, &delta)
		return &ProviderResponse{
			Choices: []Choice{{
				Index: 0,
				Delta: map[string]interface{}{
					"role":    "assistant",
					"content": delta.Text,
				},
			}},
		}, false

	case "message_delta":
		var delta struct {
			StopReason string `json:"stop_reason"`
		}
		_ = json.Unmarshal(event.Delta, &delta)
		return &ProviderResponse{
			Choices: []Choice{{
				Index:        0,
				FinishReason: mapAnthropicStopReasonToOpenAI(delta.StopReason),
			}},
			Usage: Usage{PromptTokens: event.Usage.InputTokens, CompletionTokens: event.Usage.OutputTokens},
		}, false

	case "message_stop":
		return nil, true

	default:
		return nil, false
	}
}

func (a *AnthropicAdapter) SupportsStreaming() bool {
	return true
}
