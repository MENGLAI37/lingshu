package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lingshu/lingshu/pkg/llm"
)

// ===========================================================================
// Context Manager - 上下文窗口管理
// ===========================================================================

// ContextMessage represents a message with metadata for context management.
type ContextMessage struct {
	Role        llm.MessageRole
	Content     string
	TokenCount  int64
	Timestamp   time.Time
	IsKeyContext bool // Whether this message is critical and should be preserved
}

// DefaultContextManager implements context window management.
type DefaultContextManager struct {
	maxTokens     int64
	currentTokens int64
	messages      []ContextMessage
	systemPrompt  string
	mu            sync.Mutex
}

// NewDefaultContextManager creates a new context manager.
func NewDefaultContextManager(maxTokens int64) *DefaultContextManager {
	return &DefaultContextManager{
		maxTokens:     maxTokens,
		currentTokens: 0,
		messages:      []ContextMessage{},
	}
}

// AddMessage adds a message to the context.
func (cm *DefaultContextManager) AddMessage(role llm.MessageRole, content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	tokenCount := estimateTokens(content)
	msg := ContextMessage{
		Role:       role,
		Content:    content,
		TokenCount: tokenCount,
		Timestamp:  time.Now(),
	}

	cm.messages = append(cm.messages, msg)
	cm.currentTokens += tokenCount
}

// AddToolResult adds a tool execution result to the context.
func (cm *DefaultContextManager) AddToolResult(toolName string, result string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	content := fmt.Sprintf("Tool %s result: %s", toolName, result)
	tokenCount := estimateTokens(content)

	msg := ContextMessage{
		Role:       llm.RoleTool,
		Content:    content,
		TokenCount: tokenCount,
		Timestamp:  time.Now(),
	}

	cm.messages = append(cm.messages, msg)
	cm.currentTokens += tokenCount
}

// GetMessages returns all messages in LLM format.
func (cm *DefaultContextManager) GetMessages() []llm.Message {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	result := []llm.Message{}

	// Add system prompt if present
	if cm.systemPrompt != "" {
		result = append(result, llm.Message{
			Role:    llm.RoleSystem,
			Content: cm.systemPrompt,
		})
	}

	// Convert context messages
	for _, ctxMsg := range cm.messages {
		result = append(result, llm.Message{
			Role:    ctxMsg.Role,
			Content: ctxMsg.Content,
		})
	}

	return result
}

// GetTokenCount returns the current token count.
func (cm *DefaultContextManager) GetTokenCount() int64 {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.currentTokens
}

// TrimContext trims the context to fit within maxTokens.
func (cm *DefaultContextManager) TrimContext(maxTokens int64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if we need to trim
	if cm.currentTokens <= maxTokens {
		return nil
	}

	// Calculate target tokens to remove
	targetTokens := cm.currentTokens - maxTokens
	removedTokens := int64(0)

	// Strategy: Remove oldest non-key messages first
	newMessages := []ContextMessage{}
	keyMessages := []ContextMessage{}

	// Separate key and non-key messages
	for _, msg := range cm.messages {
		if msg.IsKeyContext {
			keyMessages = append(keyMessages, msg)
		} else {
			newMessages = append(newMessages, msg)
		}
	}

	// Sort non-key messages by timestamp (oldest first)
	// Already sorted since we append in order

	// Remove oldest messages until we reach target
	trimmedMessages := []ContextMessage{}
	for _, msg := range newMessages {
		if removedTokens < targetTokens {
			removedTokens += msg.TokenCount
			continue
		}
		trimmedMessages = append(trimmedMessages, msg)
	}

	// Combine key messages with trimmed messages
	// Keep key messages at their original positions where possible
	finalMessages := cm.mergeMessages(keyMessages, trimmedMessages)

	cm.messages = finalMessages
	cm.currentTokens -= removedTokens

	// If still over limit, truncate oldest key messages
	if cm.currentTokens > maxTokens {
		cm.truncateKeyMessages(maxTokens)
	}

	return nil
}

// mergeMessages merges key and trimmed messages preserving key message positions.
func (cm *DefaultContextManager) mergeMessages(keyMessages, trimmedMessages []ContextMessage) []ContextMessage {
	// Simple merge: put trimmed messages first, then key messages at the end
	// This ensures key context (like important tool results) is preserved
	result := trimmedMessages
	result = append(result, keyMessages...)
	return result
}

// truncateKeyMessages truncates key messages if still over limit.
func (cm *DefaultContextManager) truncateKeyMessages(maxTokens int64) {
	// Keep the most recent messages
	newMessages := []ContextMessage{}
	totalTokens := int64(0)

	// Iterate from newest to oldest
	for i := len(cm.messages) - 1; i >= 0; i-- {
		msg := cm.messages[i]
		if totalTokens + msg.TokenCount <= maxTokens {
			newMessages = append([]ContextMessage{msg}, newMessages...)
			totalTokens += msg.TokenCount
		} else {
			break
		}
	}

	cm.messages = newMessages
	cm.currentTokens = totalTokens
}

// Reset clears all context.
func (cm *DefaultContextManager) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = []ContextMessage{}
	cm.currentTokens = 0
	cm.systemPrompt = ""
}

// SetSystemPrompt sets the system prompt.
func (cm *DefaultContextManager) SetSystemPrompt(prompt string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.systemPrompt = prompt
}

// MarkKeyContext marks specific messages as key context that should be preserved.
func (cm *DefaultContextManager) MarkKeyContext(indices []int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, idx := range indices {
		if idx >= 0 && idx < len(cm.messages) {
			cm.messages[idx].IsKeyContext = true
		}
	}
}

// GetContextSummary returns a summary of current context state.
func (cm *DefaultContextManager) GetContextSummary() ContextSummary {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return ContextSummary{
		TotalMessages:  len(cm.messages),
		TotalTokens:    cm.currentTokens,
		MaxTokens:      cm.maxTokens,
		UsagePercent:   float64(cm.currentTokens) / float64(cm.maxTokens) * 100,
		KeyMessageCount: countKeyMessages(cm.messages),
	}
}

// ===========================================================================
// Context Summary
// ===========================================================================

// ContextSummary provides summary information about context state.
type ContextSummary struct {
	TotalMessages   int
	TotalTokens     int64
	MaxTokens       int64
	UsagePercent    float64
	KeyMessageCount int
}

// ===========================================================================
// Token Estimation
// ===========================================================================

// estimateTokens estimates token count for a string.
// This is a simple estimation: ~4 characters per token for English, ~2 for Chinese.
func estimateTokens(content string) int64 {
	if content == "" {
		return 0
	}

	// Count Chinese characters
	chineseCount := countChineseChars(content)
	otherChars := len(content) - chineseCount

	// Estimate tokens
	// Chinese: ~2 chars per token, Other: ~4 chars per token
	chineseTokens := chineseCount / 2
	otherTokens := otherChars / 4

	return int64(chineseTokens + otherTokens + 1) // +1 for rounding
}

// countChineseChars counts Chinese characters in a string.
func countChineseChars(s string) int {
	count := 0
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			count++
		}
	}
	return count
}

func countKeyMessages(messages []ContextMessage) int {
	count := 0
	for _, msg := range messages {
		if msg.IsKeyContext {
			count++
		}
	}
	return count
}

// ===========================================================================
// Advanced Context Strategies
// ===========================================================================

// ContextStrategy defines how context should be managed.
type ContextStrategy string

const (
	StrategyPreserveRecent ContextStrategy = "preserve_recent" // Keep most recent messages
	StrategyPreserveKey    ContextStrategy = "preserve_key"    // Keep key messages
	StrategySummary        ContextStrategy = "summary"         // Summarize old messages
	StrategySlidingWindow  ContextStrategy = "sliding_window"  // Fixed-size sliding window
)

// AdvancedContextManager provides more sophisticated context management.
type AdvancedContextManager struct {
	base           *DefaultContextManager
	strategy       ContextStrategy
	windowSize     int // For sliding window strategy
	summaryService SummaryService
}

// NewAdvancedContextManager creates an advanced context manager.
func NewAdvancedContextManager(maxTokens int64, strategy ContextStrategy) *AdvancedContextManager {
	return &AdvancedContextManager{
		base:       NewDefaultContextManager(maxTokens),
		strategy:   strategy,
		windowSize: 10, // Default window size
	}
}

// SetWindowSize sets the window size for sliding window strategy.
func (acm *AdvancedContextManager) SetWindowSize(size int) {
	acm.windowSize = size
}

// SetSummaryService sets a summary service for summary strategy.
func (acm *AdvancedContextManager) SetSummaryService(service SummaryService) {
	acm.summaryService = service
}

// TrimContext trims context using the configured strategy.
func (acm *AdvancedContextManager) TrimContext(maxTokens int64) error {
	switch acm.strategy {
	case StrategySlidingWindow:
		return acm.trimSlidingWindow()
	case StrategySummary:
		return acm.trimWithSummary(maxTokens)
	case StrategyPreserveKey:
		return acm.base.TrimContext(maxTokens)
	case StrategyPreserveRecent:
		return acm.trimPreserveRecent(maxTokens)
	default:
		return acm.base.TrimContext(maxTokens)
	}
}

// trimSlidingWindow implements sliding window strategy.
func (acm *AdvancedContextManager) trimSlidingWindow() error {
	acm.base.mu.Lock()
	defer acm.base.mu.Unlock()

	if len(acm.base.messages) <= acm.windowSize {
		return nil
	}

	// Keep only the last windowSize messages
	start := len(acm.base.messages) - acm.windowSize
	removedTokens := int64(0)

	for i := 0; i < start; i++ {
		removedTokens += acm.base.messages[i].TokenCount
	}

	acm.base.messages = acm.base.messages[start:]
	acm.base.currentTokens -= removedTokens

	return nil
}

// trimWithSummary summarizes old messages and keeps recent ones.
func (acm *AdvancedContextManager) trimWithSummary(maxTokens int64) error {
	if acm.summaryService == nil {
		return acm.base.TrimContext(maxTokens)
	}

	acm.base.mu.Lock()
	defer acm.base.mu.Unlock()

	// Calculate how many tokens need to be removed
	if acm.base.currentTokens <= maxTokens {
		return nil
	}

	// Find messages to summarize (older half)
	messagesToSummarize := acm.base.messages[:len(acm.base.messages)/2]
	messagesToKeep := acm.base.messages[len(acm.base.messages)/2:]

	// Generate summary
	summaryContent, err := acm.summaryService.Summarize(messagesToSummarize)
	if err != nil {
		return err
	}

	// Replace old messages with summary
	summaryMsg := ContextMessage{
		Role:        llm.RoleSystem,
		Content:     summaryContent,
		TokenCount:  estimateTokens(summaryContent),
		Timestamp:   time.Now(),
		IsKeyContext: true,
	}

	// Calculate removed tokens
	removedTokens := int64(0)
	for _, msg := range messagesToSummarize {
		removedTokens += msg.TokenCount
	}

	acm.base.messages = append([]ContextMessage{summaryMsg}, messagesToKeep...)
	acm.base.currentTokens = acm.base.currentTokens - removedTokens + summaryMsg.TokenCount

	return nil
}

// trimPreserveRecent keeps only the most recent messages.
func (acm *AdvancedContextManager) trimPreserveRecent(maxTokens int64) error {
	acm.base.mu.Lock()
	defer acm.base.mu.Unlock()

	if acm.base.currentTokens <= maxTokens {
		return nil
	}

	// Keep messages from newest until we hit maxTokens
	newMessages := []ContextMessage{}
	totalTokens := int64(0)

	for i := len(acm.base.messages) - 1; i >= 0; i-- {
		msg := acm.base.messages[i]
		if totalTokens + msg.TokenCount <= maxTokens {
			newMessages = append([]ContextMessage{msg}, newMessages...)
			totalTokens += msg.TokenCount
		} else {
			break
		}
	}

	acm.base.messages = newMessages
	acm.base.currentTokens = totalTokens

	return nil
}

// Delegate other methods to base manager
func (acm *AdvancedContextManager) AddMessage(role llm.MessageRole, content string) {
	acm.base.AddMessage(role, content)
}

func (acm *AdvancedContextManager) AddToolResult(toolName string, result string) {
	acm.base.AddToolResult(toolName, result)
}

func (acm *AdvancedContextManager) GetMessages() []llm.Message {
	return acm.base.GetMessages()
}

func (acm *AdvancedContextManager) GetTokenCount() int64 {
	return acm.base.GetTokenCount()
}

func (acm *AdvancedContextManager) Reset() {
	acm.base.Reset()
}

// ===========================================================================
// Summary Service Interface
// ===========================================================================

// SummaryService provides message summarization.
type SummaryService interface {
	Summarize(messages []ContextMessage) (string, error)
}

// DefaultSummaryService implements basic summarization.
type DefaultSummaryService struct{}

// NewDefaultSummaryService creates a default summary service.
func NewDefaultSummaryService() *DefaultSummaryService {
	return &DefaultSummaryService{}
}

// Summarize creates a summary from messages.
func (dss *DefaultSummaryService) Summarize(messages []ContextMessage) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Simple summary: concatenate key points
	keyPoints := []string{}

	for _, msg := range messages {
		// Extract key information
		if msg.Role == llm.RoleTool {
			// Keep tool results as key points
			keyPoints = append(keyPoints, msg.Content)
		}
	}

	// Build summary
	summary := "Previous context summary:\n"
	for _, point := range keyPoints {
		summary += "- " + truncateString(point, 100) + "\n"
	}

	return summary, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ===========================================================================
// JSON Context Serialization
// ===========================================================================

// ContextJSON represents context for JSON serialization.
type ContextJSON struct {
	MaxTokens     int64             `json:"max_tokens"`
	CurrentTokens int64             `json:"current_tokens"`
	Messages      []ContextMessageJSON `json:"messages"`
	SystemPrompt  string            `json:"system_prompt,omitempty"`
}

// ContextMessageJSON represents a message for JSON serialization.
type ContextMessageJSON struct {
	Role        string `json:"role"`
	Content     string `json:"content"`
	TokenCount  int64  `json:"token_count"`
	Timestamp   string `json:"timestamp"`
	IsKeyContext bool  `json:"is_key_context"`
}

// ToJSON serializes context to JSON.
func (cm *DefaultContextManager) ToJSON() (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	msgs := []ContextMessageJSON{}
	for _, msg := range cm.messages {
		msgs = append(msgs, ContextMessageJSON{
			Role:        string(msg.Role),
			Content:     msg.Content,
			TokenCount:  msg.TokenCount,
			Timestamp:   msg.Timestamp.Format(time.RFC3339),
			IsKeyContext: msg.IsKeyContext,
		})
	}

	ctxJSON := ContextJSON{
		MaxTokens:     cm.maxTokens,
		CurrentTokens: cm.currentTokens,
		Messages:      msgs,
		SystemPrompt:  cm.systemPrompt,
	}

	data, err := json.Marshal(ctxJSON)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// FromJSON deserializes context from JSON.
func (cm *DefaultContextManager) FromJSON(jsonStr string) error {
	var ctxJSON ContextJSON
	if err := json.Unmarshal([]byte(jsonStr), &ctxJSON); err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.maxTokens = ctxJSON.MaxTokens
	cm.currentTokens = ctxJSON.CurrentTokens
	cm.systemPrompt = ctxJSON.SystemPrompt

	msgs := []ContextMessage{}
	for _, msgJSON := range ctxJSON.Messages {
		ts, _ := time.Parse(time.RFC3339, msgJSON.Timestamp)
		msgs = append(msgs, ContextMessage{
			Role:        llm.MessageRole(msgJSON.Role),
			Content:     msgJSON.Content,
			TokenCount:  msgJSON.TokenCount,
			Timestamp:   ts,
			IsKeyContext: msgJSON.IsKeyContext,
		})
	}

	cm.messages = msgs

	return nil
}

// FormatForDebug returns a human-readable debug representation.
func (cm *DefaultContextManager) FormatForDebug() string {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Context Summary: %d/%d tokens (%.1f%%)\n",
		cm.currentTokens, cm.maxTokens,
		float64(cm.currentTokens)/float64(cm.maxTokens)*100))
	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(cm.messages)))
	sb.WriteString("---\n")

	for i, msg := range cm.messages {
		sb.WriteString(fmt.Sprintf("[%d] %s (tokens: %d): %s\n",
			i, msg.Role, msg.TokenCount,
			truncateString(msg.Content, 50)))
	}

	return sb.String()
}