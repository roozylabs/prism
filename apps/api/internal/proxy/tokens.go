package proxy

import (
	"encoding/json"
	"unicode/utf8"
)

// estimatedImageTokens approximates the prompt tokens of one image part.
// Providers charge roughly 85-1600 tokens per image depending on size and
// detail; this sits in the middle so context-window and cost checks are not
// blind to images.
const estimatedImageTokens = 1000

// estimateTextTokens approximates the token count of text: about four ASCII
// characters per token, and about one token per non-ASCII character (CJK,
// emoji and accented letters tokenize far less densely than English).
func estimateTextTokens(s string) int {
	ascii, other := 0, 0
	for i := 0; i < len(s); {
		if s[i] < utf8.RuneSelf {
			ascii++
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		other++
		i += size
	}
	return (ascii+3)/4 + other
}

// estimateMessagesTokens approximates the prompt tokens of chat messages:
// text in string or array content, image parts, and assistant tool calls.
func estimateMessagesTokens(messages []map[string]interface{}) int {
	total := 0
	for _, m := range messages {
		total += estimateTextTokens(messageText(m))
		if parts, ok := m["content"].([]interface{}); ok {
			for _, p := range parts {
				if part, ok := p.(map[string]interface{}); ok && (part["type"] == "image_url" || part["type"] == "image") {
					total += estimatedImageTokens
				}
			}
		}
		if calls, ok := m["tool_calls"]; ok {
			total += estimateJSONTokens(calls)
		}
	}
	return total
}

// estimateJSONTokens approximates the tokens of a value sent as JSON, such
// as tool definitions or tool-call arguments.
func estimateJSONTokens(v interface{}) int {
	if v == nil {
		return 0
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return estimateTextTokens(string(raw))
}

// estimatePromptTokens approximates the prompt tokens of a whole request:
// its messages plus the tool definitions sent alongside them. Never below 1.
func estimatePromptTokens(req *ProxyRequest) int {
	if req == nil {
		return 1
	}
	tokens := estimateMessagesTokens(req.Messages)
	if len(req.Tools) > 0 {
		tokens += estimateJSONTokens(req.Tools)
	}
	return max(tokens, 1)
}
