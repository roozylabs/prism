package proxy

import (
	"math"
	"strings"
	"unicode"
)

type TaskType string

const (
	TaskCoding       TaskType = "coding"
	TaskReasoning    TaskType = "reasoning"
	TaskWriting      TaskType = "writing"
	TaskTranslation  TaskType = "translation"
	TaskSummarize    TaskType = "summarization"
	TaskExtraction   TaskType = "extraction"
	TaskGeneral      TaskType = "general"
)

type Complexity string

const (
	ComplexityLow    Complexity = "low"
	ComplexityMedium Complexity = "medium"
	ComplexityHigh   Complexity = "high"
)

// OutputRatioByTask returns the expected output-to-input ratio for each task type.
func OutputRatioByTask(task TaskType) float64 {
	switch task {
	case TaskCoding:
		return 0.35
	case TaskReasoning:
		return 0.40
	case TaskWriting:
		return 0.55
	case TaskTranslation:
		return 0.30
	case TaskSummarize:
		return 0.25
	case TaskExtraction:
		return 0.20
	default:
		return 0.30
	}
}

type RequestCharacteristics struct {
	Task           TaskType
	Complexity     Complexity
	ContextTokens  int
	HasCodeBlocks  bool
	EstimatedTokens int // input * (1 + outputRatio)
}

// extractMessages returns the concatenated text content from chat messages.
func extractMessages(messages []map[string]interface{}) string {
	var sb strings.Builder
	for _, m := range messages {
		if content, ok := m["content"].(string); ok {
			sb.WriteString(content)
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

// messageText returns a message's text, from a string content or from the
// "text" parts of an array content.
func messageText(m map[string]interface{}) string {
	switch content := m["content"].(type) {
	case string:
		return content
	case []interface{}:
		var parts []string
		for _, p := range content {
			part, ok := p.(map[string]interface{})
			if !ok || part["type"] != "text" {
				continue
			}
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// taskText returns the text the task type is detected from: the latest user
// message with text. System prompts and earlier turns are left out so that a
// client's fixed system prompt (e.g. a coding agent's) does not decide the task
// of every request. Falls back to all messages when there is no user text.
func taskText(messages []map[string]interface{}) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if role, _ := messages[i]["role"].(string); role != "user" {
			continue
		}
		if text := messageText(messages[i]); strings.TrimSpace(text) != "" {
			return text
		}
	}
	return extractMessages(messages)
}

// taskPriority breaks ties between equally scored task types; earlier wins.
var taskPriority = []TaskType{
	TaskCoding, TaskTranslation, TaskSummarize, TaskExtraction, TaskReasoning, TaskWriting,
}

// messageCount returns the number of messages in the conversation.
func messageCount(messages []map[string]interface{}) int {
	return len(messages)
}

// totalCharCount returns the total character count across all message content.
func totalCharCount(messages []map[string]interface{}) int {
	total := 0
	for _, m := range messages {
		if content, ok := m["content"].(string); ok {
			total += len(content)
		}
	}
	return total
}

// countCodeBlocks counts the number of fenced code blocks in text.
func countCodeBlocks(text string) int {
	count := 0
	inBlock := false
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !inBlock {
				count++
				inBlock = true
			} else {
				inBlock = false
			}
		}
	}
	return count
}

// hasFileExtensions checks for file path patterns like /path/file.ext or backtick-quoted filenames.
func hasFileExtensions(text string) bool {
	extensions := []string{
		".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".java", ".rs", ".rb",
		".c", ".cpp", ".h", ".cs", ".php", ".swift", ".kt", ".scala",
		".html", ".css", ".scss", ".json", ".yaml", ".yml", ".toml", ".xml",
		".sql", ".sh", ".bash", ".zsh", ".fish",
	}
	// Check for path-style references: /path/file.ext, ./file.ext, ../file.ext
	// or backtick-quoted filenames: `file.ext`
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, ext := range extensions {
			// Match /path/file.ext pattern
			if strings.Contains(lower, "/") && strings.Contains(lower, ext) {
				return true
			}
			// Match `filename.ext` pattern
			tick := "`" + ext
			if strings.Contains(lower, tick) {
				return true
			}
		}
	}
	return false
}

// hasStackTrace checks for common stack trace patterns.
func hasStackTrace(text string) bool {
	lower := strings.ToLower(text)
	patterns := []string{
		"traceback", "stack trace", "panic:", "fatal error",
		"exception:", "error:", "traceback (most recent",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	// "at " needs word-boundary check — "format" contains "at " but is not a stack trace
	if idx := strings.Index(lower, "at "); idx >= 0 {
		before := rune(0)
		if idx > 0 {
			before = rune(lower[idx-1])
		}
		afterIdx := idx + 3
		after := rune(0)
		if afterIdx < len(lower) {
			after = rune(lower[afterIdx])
		}
		if (!unicode.IsLetter(before) && !unicode.IsDigit(before)) &&
			(!unicode.IsLetter(after) && !unicode.IsDigit(after)) {
			return true
		}
	}
	// Check for file:line patterns like "main.go:42" — require word.ext:NUMBER
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for i := 0; i < len(trimmed)-2; i++ {
			if trimmed[i] == '.' && unicode.IsLetter(rune(trimmed[i+1])) {
				extEnd := i + 1
				for extEnd < len(trimmed) && unicode.IsLetter(rune(trimmed[extEnd])) {
					extEnd++
				}
				if extEnd < len(trimmed) && trimmed[extEnd] == ':' {
					numStart := extEnd + 1
					if numStart < len(trimmed) && unicode.IsDigit(rune(trimmed[numStart])) {
						return true
					}
				}
			}
		}
	}
	return false
}

// hasGitDiff checks for git diff markers.
func hasGitDiff(text string) bool {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			// Exclude code lines that happen to start with +/- in markdown
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "+++") || strings.HasPrefix(trimmed, "---") {
				return true
			}
			if strings.HasPrefix(trimmed, "@@") {
				return true
			}
		}
	}
	return false
}

// hasTranslateKeywords checks for translation-related keywords.
func hasTranslateKeywords(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"translate", "translate to", "translate this", "translate into",
		"translate the", "translation", "diterjemahkan", "dalam bahasa",
		"terjemahkan", "ubahkan ke bahasa", "change to language",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasSummarizeKeywords checks for summarization-related keywords.
func hasSummarizeKeywords(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"summarize", "summarise", "summary", "tldr", "tl;dr",
		"ringkas", "ringkasan", "brief summary", "brief overview",
		"give me a summary", "key points",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasExtractionKeywords checks for data extraction keywords.
func hasExtractionKeywords(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"extract", "parse", "scrape", "pull data", "get data from",
		"ekstrak", "ambil data", "ambilkan data",
		"structured data", "json format", "csv format",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasReasoningKeywords checks for reasoning/explanation keywords.
func hasReasoningKeywords(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"explain", "why", "how does", "how do", "how to",
		"what is", "what are", "describe", "analyze", "analyse",
		"compare", "contrast", "evaluate", "reason", "reasoning",
		"kenapa", "bagaimana", "mengapa", "jelaskan", "analisis",
		"perbandingan",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasWritingKeywords checks for writing/composition keywords.
func hasWritingKeywords(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"write", "write a", "compose", "draft", "create a",
		"buat", "buatkan", "tulis", "tuliskan",
		"article", "blog post", "essay", "letter", "email",
		"rewrite", "rewrite in", "paraphrase",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasCodingKeywords checks for coding-related indicators beyond code blocks.
func hasCodingKeywords(text string) bool {
	lower := strings.ToLower(text)

	// Exact phrase keywords (no boundary check needed)
	phraseKeywords := []string{
		"fix this", "refactor", "debug", "implement a",
		"write a function", "write a class", "write a method",
		"api endpoint", "docker-compose", "pull request",
	}
	for _, kw := range phraseKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	// Words that need word-boundary checking to avoid false positives
	// e.g. "error" in "error report", but NOT "json format"
	boundaryKeywords := []string{
		"function", "class", "interface", "struct", "enum",
		"import ", "export ", "require(", "const ", "let ", "var ",
		"def ", "async ", "await ", "return ",
		"implement", "build", "code", "bug", "error", "crash",
		"react", "component", "hook", "useeffect", "usestate",
		"middleware", "router",
		"git", "commit", "branch", "merge",
		"dockerfile", "kubernetes",
		"database", "migration", "schema", "query",
	}
	words := strings.Fields(lower)
	wordSet := make(map[string]bool, len(words))
	for _, w := range words {
		// strip trailing punctuation
		w = strings.TrimRight(w, ".,;:!?")
		wordSet[w] = true
	}

	for _, kw := range boundaryKeywords {
		kw = strings.TrimRight(kw, " ")
		if wordSet[kw] {
			return true
		}
		// For multi-char keywords without space, check with surrounding boundary
		if idx := strings.Index(lower, kw); idx >= 0 {
			before := rune(0)
			if idx > 0 {
				before = rune(lower[idx-1])
			}
			afterIdx := idx + len(kw)
			after := rune(0)
			if afterIdx < len(lower) {
				after = rune(lower[afterIdx])
			}
			if (!unicode.IsLetter(before) && !unicode.IsDigit(before)) &&
				(!unicode.IsLetter(after) && !unicode.IsDigit(after)) {
				return true
			}
		}
	}
	return false
}

// estimateTokens estimates token count from character count (rough: ~4 chars per token).
func estimateTokens(charCount int) int {
	tokens := charCount / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// ClassifyRequest analyzes a request and returns its characteristics.
func ClassifyRequest(messages []map[string]interface{}) RequestCharacteristics {
	text := taskText(messages)
	charCount := totalCharCount(messages)

	// Complexity looks at the conversation only: a client's fixed system
	// prompt says nothing about how hard the current request is.
	var conversation []map[string]interface{}
	for _, m := range messages {
		if role, _ := m["role"].(string); role != "system" && role != "developer" {
			conversation = append(conversation, m)
		}
	}
	convoChars := totalCharCount(conversation)
	msgCount := messageCount(conversation)

	hasCode := countCodeBlocks(text) > 0
	hasFiles := hasFileExtensions(text)
	hasTrace := hasStackTrace(text)
	hasDiff := hasGitDiff(text)

	// Determine task type with scoring
	scores := make(map[TaskType]float64)

	if hasCode || hasFiles || hasTrace || hasDiff {
		scores[TaskCoding] += 3.0
	}
	if hasCodingKeywords(text) {
		scores[TaskCoding] += 1.0
	}

	if hasTranslateKeywords(text) {
		scores[TaskTranslation] += 3.0
	}
	if hasSummarizeKeywords(text) {
		scores[TaskSummarize] += 3.0
	}
	if hasExtractionKeywords(text) {
		scores[TaskExtraction] += 3.0
	}
	if hasReasoningKeywords(text) {
		scores[TaskReasoning] += 2.0
	}
	if hasWritingKeywords(text) {
		scores[TaskWriting] += 2.0
	}

	// Pick highest scoring task; iterate in priority order so ties are deterministic
	task := TaskGeneral
	bestScore := 0.0
	for _, t := range taskPriority {
		if s := scores[t]; s > bestScore {
			bestScore = s
			task = t
		}
	}

	// Determine complexity
	complexityScore := 0
	if convoChars > 4000 {
		complexityScore++
	}
	if convoChars > 16000 {
		complexityScore += 2
	}
	if msgCount > 10 {
		complexityScore++
	}
	if msgCount > 20 {
		complexityScore++
	}
	if hasCode && convoChars > 4000 {
		complexityScore++ // code-heavy = more complex
	}

	var complexity Complexity
	switch {
	case complexityScore >= 4:
		complexity = ComplexityHigh
	case complexityScore >= 2:
		complexity = ComplexityMedium
	default:
		complexity = ComplexityLow
	}

	tokens := estimateTokens(charCount)
	outputRatio := OutputRatioByTask(task)
	estimatedTokens := int(math.Round(float64(tokens) * (1 + outputRatio)))

	return RequestCharacteristics{
		Task:            task,
		Complexity:      complexity,
		ContextTokens:   tokens,
		HasCodeBlocks:   hasCode,
		EstimatedTokens: estimatedTokens,
	}
}
