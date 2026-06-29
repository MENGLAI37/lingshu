package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type OutputFormat string

const (
	OutputFormatJSON    OutputFormat = "json"
	OutputFormatYAML    OutputFormat = "yaml"
	OutputFormatTable   OutputFormat = "table"
	OutputFormatLLM     OutputFormat = "llm"
)

type Formatter struct {
	format OutputFormat
}

func NewFormatter(format OutputFormat) *Formatter {
	if format == "" {
		format = OutputFormatLLM
	}
	return &Formatter{
		format: format,
	}
}

func (f *Formatter) Format(result *ToolResult) (string, error) {
	switch f.format {
	case OutputFormatJSON:
		return f.formatJSON(result)
	case OutputFormatYAML:
		return f.formatYAML(result)
	case OutputFormatTable:
		return f.formatTable(result)
	case OutputFormatLLM:
		return f.formatLLM(result)
	default:
		return f.formatLLM(result)
	}
}

func (f *Formatter) formatJSON(result *ToolResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}
	return string(data), nil
}

func (f *Formatter) formatYAML(result *ToolResult) (string, error) {
	data, err := yaml.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to format YAML: %w", err)
	}
	return string(data), nil
}

func (f *Formatter) formatTable(result *ToolResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool: %s\n", result.ToolName))
	sb.WriteString(fmt.Sprintf("Risk Level: %s\n", result.RiskLevel))
	sb.WriteString(fmt.Sprintf("Status: %s\n", map[bool]string{true: "Success", false: "Failed"}[result.Success]))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", result.Duration))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", result.Timestamp.Format("2006-01-02 15:04:05")))

	if result.Message != "" {
		sb.WriteString(fmt.Sprintf("\nMessage: %s\n", result.Message))
	}

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("\nError: %s\n", result.Error))
	}

	if result.Data != nil {
		sb.WriteString("\nData:\n")
		sb.WriteString(formatDataAsTable(result.Data, 0))
	}

	return sb.String(), nil
}

func (f *Formatter) formatLLM(result *ToolResult) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("--- Tool Result: %s ---\n", result.ToolName))
	sb.WriteString(fmt.Sprintf("Risk Level: %s\n", result.RiskLevel))
	sb.WriteString(fmt.Sprintf("Success: %t\n", result.Success))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", result.Duration))

	if result.Message != "" {
		sb.WriteString(fmt.Sprintf("Message: %s\n", result.Message))
	}

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
	}

	if result.Data != nil {
		sb.WriteString("\nData:\n")
		sb.WriteString(formatDataForLLM(result.Data, 0))
	}

	sb.WriteString("--- End Tool Result ---\n")

	return sb.String(), nil
}

func formatDataAsTable(data any, indent int) string {
	var sb strings.Builder
	prefix := strings.Repeat("  ", indent)

	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			sb.WriteString(fmt.Sprintf("%s%s: %v\n", prefix, key, formatValue(val)))
		}
	case []any:
		for i, item := range v {
			sb.WriteString(fmt.Sprintf("%s[%d]:\n%s", prefix, i, formatDataAsTable(item, indent+1)))
		}
	default:
		sb.WriteString(fmt.Sprintf("%s%v\n", prefix, v))
	}

	return sb.String()
}

func formatDataForLLM(data any, indent int) string {
	var sb strings.Builder
	prefix := strings.Repeat("  ", indent)

	switch v := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		for _, key := range keys {
			val := v[key]
			sb.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, key, formatValueForLLM(val, indent+1)))
		}
	case []any:
		for i, item := range v {
			sb.WriteString(fmt.Sprintf("%s  [%d] %s\n", prefix, i, formatValueForLLM(item, indent+1)))
		}
	default:
		sb.WriteString(fmt.Sprintf("%v", v))
	}

	return sb.String()
}

func formatValue(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatValueForLLM(val any, indent int) string {
	prefix := strings.Repeat("  ", indent)

	switch v := val.(type) {
	case map[string]any:
		result := "\n"
		for key, innerVal := range v {
			result += fmt.Sprintf("%s%s: %v\n", prefix, key, innerVal)
		}
		return result
	case []any:
		if len(v) == 0 {
			return "[]"
		}
		result := fmt.Sprintf(" (%d items)\n", len(v))
		for i, item := range v {
			if i >= 10 {
				result += fmt.Sprintf("%s  ... and %d more\n", prefix, len(v)-10)
				break
			}
			result += fmt.Sprintf("%s  [%d] %v\n", prefix, i, item)
		}
		return result
	default:
		return fmt.Sprintf("%v", v)
	}
}

func FormatJSON(data any) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

func FormatYAML(data any) (string, error) {
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(bytes), nil
}

func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	var sb strings.Builder

	for i, h := range headers {
		sb.WriteString(padRight(h, colWidths[i]))
		if i < len(headers)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	for i := range headers {
		sb.WriteString(strings.Repeat("-", colWidths[i]))
		if i < len(headers)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n")

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				sb.WriteString(padRight(cell, colWidths[i]))
				if i < len(headers)-1 {
					sb.WriteString("  ")
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func SummarizeResult(result *ToolResult) string {
	if result.Success {
		return fmt.Sprintf("[%s] %s - %s", result.RiskLevel, result.ToolName, result.Message)
	}
	return fmt.Sprintf("[%s] %s FAILED: %s", result.RiskLevel, result.ToolName, result.Error)
}
