package proxy

import "testing"

func TestEstimateTextTokens(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"ascii rounds up", "hello", 2},
		{"ascii exact", "abcdefgh", 2},
		{"cjk one per char", "你好世界", 4},
		{"mixed", "hi 你好", 1 + 2},
	}
	for _, c := range cases {
		if got := estimateTextTokens(c.text); got != c.want {
			t.Errorf("%s: estimateTextTokens(%q) = %d, want %d", c.name, c.text, got, c.want)
		}
	}
}

func TestEstimateMessagesTokens_ArrayContentAndImages(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": []interface{}{
			map[string]interface{}{"type": "text", "text": "abcdefgh"},
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "https://example.com/a.png"}},
		}},
	}
	if got, want := estimateMessagesTokens(msgs), 2+estimatedImageTokens; got != want {
		t.Errorf("expected text plus one image (%d), got %d", want, got)
	}
}

func TestEstimateMessagesTokens_ToolCalls(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "assistant", "content": nil, "tool_calls": []interface{}{
			map[string]interface{}{"id": "call_1", "type": "function", "function": map[string]interface{}{
				"name": "search", "arguments": `{"query":"prism gateway pricing"}`,
			}},
		}},
	}
	if got := estimateMessagesTokens(msgs); got == 0 {
		t.Error("tool call arguments should count toward the prompt")
	}
}

func TestEstimatePromptTokens_IncludesTools(t *testing.T) {
	req := &ProxyRequest{Messages: []map[string]interface{}{msg("hi")}}
	without := estimatePromptTokens(req)

	req.Tools = []interface{}{map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "get_weather",
			"description": "Get the current weather for a city",
			"parameters":  map[string]interface{}{"type": "object", "properties": map[string]interface{}{"city": map[string]interface{}{"type": "string"}}},
		},
	}}
	if with := estimatePromptTokens(req); with <= without+10 {
		t.Errorf("tool definitions should add tokens: %d without, %d with", without, with)
	}
	if got := estimatePromptTokens(nil); got != 1 {
		t.Errorf("nil request should estimate 1 token, got %d", got)
	}
}

func TestClassifyRequest_ImageRaisesContextTokens(t *testing.T) {
	text := []map[string]interface{}{msg("Describe this picture.")}
	withImage := []map[string]interface{}{{"role": "user", "content": []interface{}{
		map[string]interface{}{"type": "text", "text": "Describe this picture."},
		map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "https://example.com/a.png"}},
	}}}

	if a, b := ClassifyRequest(text).ContextTokens, ClassifyRequest(withImage).ContextTokens; b < a+estimatedImageTokens {
		t.Errorf("image should add ~%d tokens: %d vs %d", estimatedImageTokens, a, b)
	}
}

func TestWithExtraContext(t *testing.T) {
	c := RequestCharacteristics{Task: TaskCoding, ContextTokens: 100, EstimatedTokens: 135}
	got := c.withExtraContext(100)
	if got.ContextTokens != 200 || got.EstimatedTokens != 270 {
		t.Errorf("expected 200 context / 270 estimated, got %d / %d", got.ContextTokens, got.EstimatedTokens)
	}
	if same := c.withExtraContext(0); same != c {
		t.Errorf("zero extra tokens should not change characteristics, got %+v", same)
	}
}
