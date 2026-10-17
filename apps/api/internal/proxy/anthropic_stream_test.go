package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicStreamUsageComesFromMessageStartAndMessageDelta(t *testing.T) {
	a := &AnthropicAdapter{}

	start, done := a.ParseStreamChunk([]byte(`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":25,"output_tokens":1}}}`))
	require.False(t, done)
	require.NotNil(t, start)
	assert.Equal(t, 25, start.Usage.PromptTokens)

	text, _ := a.ParseStreamChunk([]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`))
	require.NotNil(t, text)
	assert.Equal(t, "Hi", text.Choices[0].Delta["content"])

	delta, done := a.ParseStreamChunk([]byte(`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`))
	require.False(t, done)
	require.NotNil(t, delta)
	assert.Equal(t, 42, delta.Usage.CompletionTokens)
	assert.Equal(t, "stop", delta.Choices[0].FinishReason)

	_, done = a.ParseStreamChunk([]byte(`data: {"type":"message_stop"}`))
	assert.True(t, done)
}

func TestAnthropicStreamErrorEventIsReported(t *testing.T) {
	a := &AnthropicAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
	require.False(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Error)
	assert.Equal(t, "overloaded_error", chunk.Error.Type)
	assert.Equal(t, "Overloaded", chunk.Error.Message)
}

func TestAnthropicStreamIgnoresPingsAndEventLines(t *testing.T) {
	a := &AnthropicAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`event: ping`))
	assert.Nil(t, chunk)
	assert.False(t, done)
	chunk, done = a.ParseStreamChunk([]byte(`data: {"type":"ping"}`))
	assert.Nil(t, chunk)
	assert.False(t, done)
}

func TestOpenAIStreamErrorObjectIsReported(t *testing.T) {
	a := &OpenAIAdapter{}

	chunk, done := a.ParseStreamChunk([]byte(`data: {"error":{"message":"The server had an error","type":"server_error","code":"overloaded"}}`))
	require.False(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Error)
	assert.Equal(t, "server_error", chunk.Error.Type)
	assert.Equal(t, "overloaded", chunk.Error.Code)

	// A normal chunk is unaffected.
	chunk, _ = a.ParseStreamChunk([]byte(`data: {"id":"1","choices":[{"index":0,"delta":{"content":"hi"}}]}`))
	require.NotNil(t, chunk)
	assert.Nil(t, chunk.Error)
	assert.Equal(t, "hi", chunk.Choices[0].Delta["content"])
}
