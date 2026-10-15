package proxy

import (
	"testing"
)

func msg(content string) map[string]interface{} {
	return map[string]interface{}{"role": "user", "content": content}
}

func TestClassifyRequest_CodingWithCodeBlock(t *testing.T) {
	req := []map[string]interface{}{
		msg("Fix this React component:\n```tsx\nfunction App() { return <div>hi</div> }\n```\nIt rerenders too often."),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskCoding {
		t.Errorf("expected coding, got %s", chars.Task)
	}
	if !chars.HasCodeBlocks {
		t.Error("expected HasCodeBlocks=true")
	}
}

func TestClassifyRequest_CodingWithFileExtension(t *testing.T) {
	req := []map[string]interface{}{
		msg("The bug is in src/utils/parser.go at line 42. Can you fix it?"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskCoding {
		t.Errorf("expected coding, got %s", chars.Task)
	}
}

func TestClassifyRequest_Translation(t *testing.T) {
	req := []map[string]interface{}{
		msg("Translate this to Japanese: Hello world"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskTranslation {
		t.Errorf("expected translation, got %s", chars.Task)
	}
}

func TestClassifyRequest_Summarization(t *testing.T) {
	req := []map[string]interface{}{
		msg("Summarize this article in 3 bullet points: [long article text]"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskSummarize {
		t.Errorf("expected summarization, got %s", chars.Task)
	}
}

func TestClassifyRequest_Extraction(t *testing.T) {
	req := []map[string]interface{}{
		msg("Extract all email addresses from this text and output as JSON format"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskExtraction {
		t.Errorf("expected extraction, got %s", chars.Task)
	}
}

func TestClassifyRequest_Reasoning(t *testing.T) {
	req := []map[string]interface{}{
		msg("Explain why the sky is blue and how light scattering works"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskReasoning {
		t.Errorf("expected reasoning, got %s", chars.Task)
	}
}

func TestClassifyRequest_Writing(t *testing.T) {
	req := []map[string]interface{}{
		msg("Write a blog post about AI gateways and their benefits"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskWriting {
		t.Errorf("expected writing, got %s", chars.Task)
	}
}

func TestClassifyRequest_General(t *testing.T) {
	req := []map[string]interface{}{
		msg("Hi there"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskGeneral {
		t.Errorf("expected general, got %s", chars.Task)
	}
}

func TestClassifyRequest_ComplexityLow(t *testing.T) {
	req := []map[string]interface{}{
		msg("Hi"),
	}
	chars := ClassifyRequest(req)
	if chars.Complexity != ComplexityLow {
		t.Errorf("expected low complexity, got %s", chars.Complexity)
	}
}

func TestClassifyRequest_ComplexityHigh(t *testing.T) {
	longText := ""
	for i := 0; i < 200; i++ {
		longText += "This is a long paragraph of text that adds up. "
	}
	req := make([]map[string]interface{}, 15)
	for i := range req {
		req[i] = msg(longText)
	}
	chars := ClassifyRequest(req)
	if chars.Complexity != ComplexityHigh {
		t.Errorf("expected high complexity, got %s (score from %d msgs)", chars.Complexity, len(req))
	}
}

func TestClassifyRequest_EmptyMessages(t *testing.T) {
	chars := ClassifyRequest([]map[string]interface{}{})
	if chars.Task != TaskGeneral {
		t.Errorf("expected general for empty messages, got %s", chars.Task)
	}
	if chars.Complexity != ComplexityLow {
		t.Errorf("expected low complexity for empty messages, got %s", chars.Complexity)
	}
}

func TestClassifyRequest_EstimatedTokens(t *testing.T) {
	content := "Write a long blog post about testing."
	req := []map[string]interface{}{msg(content)}
	chars := ClassifyRequest(req)

	expectedInput := (len(content) + 3) / 4               // ASCII text, rounded up
	expectedOutputRatio := OutputRatioByTask(TaskWriting) // 0.55
	expectedEstimated := int(float64(expectedInput) * (1 + expectedOutputRatio))

	if chars.EstimatedTokens < expectedEstimated-2 || chars.EstimatedTokens > expectedEstimated+2 {
		t.Errorf("estimated tokens: expected ~%d, got %d", expectedEstimated, chars.EstimatedTokens)
	}
}

func TestOutputRatioByTask(t *testing.T) {
	tests := []struct {
		task     TaskType
		expected float64
	}{
		{TaskCoding, 0.35},
		{TaskReasoning, 0.40},
		{TaskWriting, 0.55},
		{TaskTranslation, 0.30},
		{TaskSummarize, 0.25},
		{TaskExtraction, 0.20},
		{TaskGeneral, 0.30},
	}
	for _, tt := range tests {
		got := OutputRatioByTask(tt.task)
		if got != tt.expected {
			t.Errorf("OutputRatioByTask(%s): expected %v, got %v", tt.task, tt.expected, got)
		}
	}
}

func TestCountCodeBlocks(t *testing.T) {
	text := "Here is code:\n```go\nfmt.Println()\n```\nAnd more:\n```python\nprint()\n```"
	count := countCodeBlocks(text)
	if count != 2 {
		t.Errorf("expected 2 code blocks, got %d", count)
	}
}

func TestHasStackTrace(t *testing.T) {
	text := "goroutine 1 [running]:\nmain.main()\n\t/main.go:12 +0x20\npanic: runtime error"
	if !hasStackTrace(text) {
		t.Error("expected stack trace detection")
	}
}

func TestHasGitDiff(t *testing.T) {
	text := "--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n-old line\n+new line"
	if !hasGitDiff(text) {
		t.Error("expected git diff detection")
	}
}

func TestClassifyRequest_IgnoresSystemPrompt(t *testing.T) {
	req := []map[string]interface{}{
		{"role": "system", "content": "You are a coding agent. Use tools to edit files like src/main.go and run `go test`.\n```go\nfunc main() {}\n```"},
		msg("Translate this paragraph into Indonesian: good morning everyone."),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskTranslation {
		t.Errorf("expected translation from the user message, got %s", chars.Task)
	}
}

func TestClassifyRequest_UsesLatestUserMessage(t *testing.T) {
	req := []map[string]interface{}{
		msg("Write a blog post about testing."),
		{"role": "assistant", "content": "Here is a draft..."},
		msg("Now fix this bug in src/app.ts"),
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskCoding {
		t.Errorf("expected coding from the latest user message, got %s", chars.Task)
	}
}

func TestClassifyRequest_ArrayContent(t *testing.T) {
	req := []map[string]interface{}{
		{"role": "user", "content": []interface{}{
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "https://example.com/a.png"}},
			map[string]interface{}{"type": "text", "text": "Summarize the key points of this chart."},
		}},
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskSummarize {
		t.Errorf("expected summarization from text parts, got %s", chars.Task)
	}
}

func TestClassifyRequest_TieBreakIsDeterministic(t *testing.T) {
	// translation and summarization both score 3; translation has priority.
	req := []map[string]interface{}{msg("translate this summary to English")}
	for i := 0; i < 200; i++ {
		if got := ClassifyRequest(req).Task; got != TaskTranslation {
			t.Fatalf("run %d: expected translation, got %s", i, got)
		}
	}
}

func TestClassifyRequest_NoUserMessageFallsBackToAllText(t *testing.T) {
	req := []map[string]interface{}{
		{"role": "system", "content": "Summarize the conversation so far."},
	}
	chars := ClassifyRequest(req)
	if chars.Task != TaskSummarize {
		t.Errorf("expected summarization fallback, got %s", chars.Task)
	}
}

func TestClassifyRequest_SystemPromptDoesNotRaiseComplexity(t *testing.T) {
	longSystem := ""
	for i := 0; i < 500; i++ {
		longSystem += "Always follow the repository conventions. "
	}
	req := []map[string]interface{}{
		{"role": "system", "content": longSystem},
		msg("Rename this variable please."),
	}
	chars := ClassifyRequest(req)
	if chars.Complexity != ComplexityLow {
		t.Errorf("expected low complexity despite a long system prompt, got %s", chars.Complexity)
	}
	if chars.ContextTokens < len(longSystem)/4 {
		t.Errorf("context tokens must still count the system prompt, got %d", chars.ContextTokens)
	}
}
