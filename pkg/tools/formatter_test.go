package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewFormatter(t *testing.T) {
	f := NewFormatter(OutputFormatJSON)
	assert.NotNil(t, f)
}

func TestNewFormatter_DefaultFormat(t *testing.T) {
	f := NewFormatter("")
	assert.NotNil(t, f)
}

func TestFormatter_FormatJSON(t *testing.T) {
	f := NewFormatter(OutputFormatJSON)

	result := &ToolResult{
		Success:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	output, err := f.Format(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "test_tool")
}

func TestFormatter_FormatYAML(t *testing.T) {
	f := NewFormatter(OutputFormatYAML)

	result := &ToolResult{
		Success:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	output, err := f.Format(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestFormatter_FormatTable(t *testing.T) {
	f := NewFormatter(OutputFormatTable)

	result := &ToolResult{
		Success:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
		Data: []map[string]interface{}{
			{
				"Name":   "pod1",
				"Status": "Running",
				"Age":    "1d",
			},
			{
				"Name":   "pod2",
				"Status": "Pending",
				"Age":    "2d",
			},
		},
	}

	output, err := f.Format(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "test_tool")
}

func TestFormatter_FormatLLM(t *testing.T) {
	f := NewFormatter(OutputFormatLLM)

	result := &ToolResult{
		Success:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
		Data: map[string]interface{}{
			"key": "value",
		},
	}

	output, err := f.Format(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Tool Result: test_tool")
	assert.Contains(t, output, "Success: true")
}

func TestFormatter_FormatWithError(t *testing.T) {
	f := NewFormatter(OutputFormatJSON)

	result := &ToolResult{
		Success:   false,
		Error:     "test error",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
	}

	output, err := f.Format(result)
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "test error")
}

func TestFormatJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	result, err := FormatJSON(data)
	assert.NoError(t, err)
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "value")
}

func TestFormatYAML(t *testing.T) {
	data := map[string]string{"key": "value"}
	result, err := FormatYAML(data)
	assert.NoError(t, err)
	assert.Contains(t, result, "key: value")
}

func TestFormatTable(t *testing.T) {
	headers := []string{"Name", "Status"}
	rows := [][]string{
		{"pod1", "Running"},
		{"pod2", "Pending"},
	}

	result := FormatTable(headers, rows)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Status")
	assert.Contains(t, result, "pod1")
	assert.Contains(t, result, "Running")
}

func TestSummarizeResult(t *testing.T) {
	result := &ToolResult{
		Success:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
	}

	summary := SummarizeResult(result)
	assert.NotEmpty(t, summary)
	assert.Contains(t, summary, "test_tool")
	assert.Contains(t, summary, "test message")
}

func TestSummarizeResult_Error(t *testing.T) {
	result := &ToolResult{
		Success:   false,
		Error:     "test error",
		Timestamp: time.Now(),
		Duration:  "1s",
		ToolName:  "test_tool",
		RiskLevel: RiskLevelL0,
	}

	summary := SummarizeResult(result)
	assert.NotEmpty(t, summary)
	assert.Contains(t, summary, "test_tool")
	assert.Contains(t, summary, "FAILED")
	assert.Contains(t, summary, "test error")
}
