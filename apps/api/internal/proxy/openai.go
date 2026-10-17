package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type OpenAIAdapter struct{}

func NewOpenAIAdapter() *OpenAIAdapter {
	return &OpenAIAdapter{}
}

func (a *OpenAIAdapter) BuildRequest(baseURL, apiKey string, req *ProviderRequest) (*http.Request, error) {
	messages := SanitizeMessagesForGoogle(req.Messages)

	body := map[string]interface{}{
		"model":    req.Model,
		"messages": messages,
		"stream":   req.Stream,
	}
	if len(req.Tools) > 0 {
		body["tools"] = ConvertGenericToolsToOpenAI(req.Tools)
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}
	if req.Stream {
		body["stream_options"] = map[string]interface{}{
			"include_usage": true,
		}
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

	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	return httpReq, nil
}

func (a *OpenAIAdapter) ParseResponse(body io.Reader) (*ProviderResponse, error) {
	var raw struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int                    `json:"index"`
			Message      map[string]interface{} `json:"message"`
			FinishReason string                 `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, err
	}

	resp := &ProviderResponse{
		ID:    raw.ID,
		Model: raw.Model,
		Usage: Usage{
			PromptTokens:     raw.Usage.PromptTokens,
			CompletionTokens: raw.Usage.CompletionTokens,
			TotalTokens:      raw.Usage.TotalTokens,
		},
	}

	if raw.Error != nil {
		resp.Error = &ProviderError{
			Type:    raw.Error.Type,
			Message: raw.Error.Message,
			Code:    raw.Error.Code,
		}
		return resp, nil
	}

	for _, c := range raw.Choices {
		resp.Choices = append(resp.Choices, Choice{
			Index:        c.Index,
			Message:      c.Message,
			FinishReason: c.FinishReason,
		})
	}

	return resp, nil
}

func (a *OpenAIAdapter) ParseStreamChunk(line []byte) (*ProviderResponse, bool) {
	lineStr := strings.TrimSpace(string(line))
	if !strings.HasPrefix(lineStr, "data:") {
		return nil, false
	}
	data := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))
	if data == "[DONE]" {
		return nil, true
	}

	var chunk struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int                    `json:"index"`
			Delta        map[string]interface{} `json:"delta"`
			FinishReason *string                `json:"finish_reason"`
		} `json:"choices"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, false
	}

	resp := &ProviderResponse{
		ID:    chunk.ID,
		Model: chunk.Model,
	}

	// An error object in the stream means the provider gave up on the
	// request; the engine decides whether to fail over or tell the client.
	if chunk.Error != nil && (chunk.Error.Message != "" || chunk.Error.Type != "") {
		msg := chunk.Error.Message
		if msg == "" {
			msg = "upstream reported an error"
		}
		resp.Error = &ProviderError{Type: chunk.Error.Type, Message: msg, Code: chunk.Error.Code}
		return resp, false
	}

	for _, c := range chunk.Choices {
		choice := Choice{
			Index: c.Index,
			Delta: c.Delta,
		}
		if c.FinishReason != nil {
			choice.FinishReason = *c.FinishReason
		}
		resp.Choices = append(resp.Choices, choice)
	}

	if chunk.Usage != nil {
		resp.Usage = Usage{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}
	}

	return resp, false
}

func (a *OpenAIAdapter) SupportsStreaming() bool {
	return true
}
