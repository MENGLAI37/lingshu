package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/lingshu/lingshu/pkg/llm"
)

// ===========================================================================
// Tool Call Parser - Function Call 解析器
// ===========================================================================

// ParsedToolCall represents a parsed tool call from LLM response.
type ParsedToolCall struct {
	Name        string         `json:"name"`
	Arguments   map[string]any `json:"arguments"`
	RawJSON     string         `json:"raw_json,omitempty"`     // Original JSON for debugging
	ToolCallID  string         `json:"tool_call_id,omitempty"` // ID from LLM, required for multi-turn tool calling
}

// ParseError represents an error during parsing.
type ParseError struct {
	Message string
	Raw     string
	Cause   error
}

func (e *ParseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("parse error: %s (raw: %s): %v", e.Message, e.Raw, e.Cause)
	}
	return fmt.Sprintf("parse error: %s (raw: %s)", e.Message, e.Raw)
}

// ToolCallParser parses function calls from LLM responses.
type ToolCallParser struct {
	jsonPattern    *regexp.Regexp
	namePattern    *regexp.Regexp
	fallbackParser *FallbackParser
}

// NewToolCallParser creates a new tool call parser.
func NewToolCallParser() *ToolCallParser {
	return &ToolCallParser{
		jsonPattern:    regexp.MustCompile(`\{[^{}]*"name"\s*:\s*"([^"]+)"[^{}]*\}`),
		namePattern:    regexp.MustCompile(`tool_call:\s*(\w+)\s*\(([^)]*)\)`),
		fallbackParser: NewFallbackParser(),
	}
}

// Parse parses function calls from an LLM FunctionCall.
func (p *ToolCallParser) Parse(fc *llm.FunctionCall) []ParsedToolCall {
	if fc == nil {
		return nil
	}

	// Try structured parsing first
	toolCalls := p.parseStructured(fc)
	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Try parsing from content if FunctionCall arguments contain tool references
	toolCalls = p.parseFromContent(fc.Arguments)
	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Try fallback parsing
	return p.fallbackParser.Parse(fc.Name, fc.Arguments)
}

// ParseToolCalls parses tool calls from []llm.ToolCall (standard OpenAI format with IDs).
func (p *ToolCallParser) ParseToolCalls(tcs []llm.ToolCall) []ParsedToolCall {
	if len(tcs) == 0 {
		return nil
	}

	result := []ParsedToolCall{}
	for _, tc := range tcs {
		args, err := p.parseArguments(tc.Function.Arguments)
		if err != nil {
			// Return partial result with error indication
			result = append(result, ParsedToolCall{
				Name:       tc.Function.Name,
				RawJSON:    tc.Function.Arguments,
				ToolCallID: tc.ID,
			})
			continue
		}
		result = append(result, ParsedToolCall{
			Name:       tc.Function.Name,
			Arguments:  args,
			RawJSON:    tc.Function.Arguments,
			ToolCallID: tc.ID,
		})
	}
	return result
}

// ParseFromContent parses tool calls directly from text content.
// This is used as a fallback when the model doesn't use structured function calling.
func (p *ToolCallParser) ParseFromContent(content string) []ParsedToolCall {
	if content == "" {
		return nil
	}

	toolCalls := []ParsedToolCall{}

	// 1. Try JSON code blocks with tool calls
	jsonBlockPattern := regexp.MustCompile("(?s)```(?:json)?\\s*(\\{[^{}]*\"name\"\\s*:\\s*\"[^\"]+\"[^{}]*\\})\\s*```")
	matches := jsonBlockPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			args, err := p.parseArguments(match[1])
			if err == nil {
				if name, ok := args["name"].(string); ok {
					toolArgs := map[string]any{}
					if params, ok := args["parameters"].(map[string]any); ok {
						toolArgs = params
					}
					toolCalls = append(toolCalls, ParsedToolCall{
						Name:      name,
						Arguments: toolArgs,
						RawJSON:   match[1],
					})
				}
			}
		}
	}

	if len(toolCalls) > 0 {
		return toolCalls
	}

	// 2. Try inline JSON patterns
	matches = p.jsonPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			fullMatch := match[0]
			toolName := match[1]

			args, err := p.parseArguments(fullMatch)
			if err == nil {
				toolCalls = append(toolCalls, ParsedToolCall{
					Name:      toolName,
					Arguments: args,
					RawJSON:   fullMatch,
				})
			}
		}
	}

	if len(toolCalls) > 0 {
		return toolCalls
	}

	// 3. Try fallback parser on the full content
	return p.fallbackParser.ParseFromText(content)
}

// parseStructured parses a properly formatted function call.
func (p *ToolCallParser) parseStructured(fc *llm.FunctionCall) []ParsedToolCall {
	// Direct function call with JSON arguments
	if fc.Name != "" && fc.Arguments != "" {
		args, err := p.parseArguments(fc.Arguments)
		if err != nil {
			// Return partial result with error indication
			return []ParsedToolCall{
				{
					Name:    fc.Name,
					RawJSON: fc.Arguments,
				},
			}
		}
		return []ParsedToolCall{
			{
				Name:      fc.Name,
				Arguments: args,
				RawJSON:   fc.Arguments,
			},
		}
	}

	return nil
}

// parseFromContent attempts to parse tool calls embedded in content.
func (p *ToolCallParser) parseFromContent(content string) []ParsedToolCall {
	toolCalls := []ParsedToolCall{}

	// Look for JSON-like tool call structures
	matches := p.jsonPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			fullMatch := match[0]
			toolName := match[1]

			args, err := p.parseArguments(fullMatch)
			if err == nil {
				toolCalls = append(toolCalls, ParsedToolCall{
					Name:      toolName,
					Arguments: args,
					RawJSON:   fullMatch,
				})
			}
		}
	}

	// Look for simple tool_call patterns
	matches2 := p.namePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches2 {
		if len(match) >= 3 {
			toolName := match[1]
			argsStr := match[2]

			args := p.parseSimpleArgs(argsStr)
			toolCalls = append(toolCalls, ParsedToolCall{
				Name:      toolName,
				Arguments: args,
				RawJSON:   match[0],
			})
		}
	}

	return toolCalls
}

// parseArguments parses JSON arguments string.
func (p *ToolCallParser) parseArguments(argsStr string) (map[string]any, error) {
	argsStr = strings.TrimSpace(argsStr)
	if argsStr == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		return nil, &ParseError{
			Message: "failed to parse arguments JSON",
			Raw:     argsStr,
			Cause:   err,
		}
	}

	return args, nil
}

// parseSimpleArgs parses simple argument format like "namespace=default,name=pod1".
func (p *ToolCallParser) parseSimpleArgs(argsStr string) map[string]any {
	args := map[string]any{}
	argsStr = strings.TrimSpace(argsStr)

	if argsStr == "" {
		return args
	}

	// Split by comma
	parts := strings.Split(argsStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			// Remove quotes if present
			val = strings.Trim(val, "\"'")
			args[key] = val
		}
	}

	return args
}

// ===========================================================================
// Fallback Parser - 用于处理非标准格式的工具调用
// ===========================================================================

// FallbackParser handles non-standard tool call formats.
type FallbackParser struct {
	toolPatterns map[string]*regexp.Regexp
}

// NewFallbackParser creates a new fallback parser.
func NewFallbackParser() *FallbackParser {
	return &FallbackParser{
		toolPatterns: map[string]*regexp.Regexp{
			"k8s_get":      regexp.MustCompile(`(?i)(?:get|list|show|查看|列出|获取)\s+(pod|pods|deployment|deployments|service|services|event|events|configmap|configmaps|ingress|ingresses|node|nodes|namespace|namespaces)(?:\s+(?:named|called|name=)?([^\s,，。）)]+))?(?:\s+(?:in|from|namespace=?)\s*([^\s,，。）)]+))?`),
			"k8s_describe": regexp.MustCompile(`(?i)(?:describe|details|info about|详细信息|描述)\s+(pod|deployment|service|node)(?:\s+([^\s,，。）)]+))?(?:\s+(?:in|from|namespace=?)\s*([^\s,，。）)]+))?`),
			"k8s_logs":     regexp.MustCompile(`(?i)(?:logs|log|日志)\s+(?:for\s+)?(?:pod\s+)?([^\s,，。）)]+)(?:\s+(?:in|from|namespace=?)\s*([^\s,，。）)]+))?`),
			"k8s_events":   regexp.MustCompile(`(?i)(?:events?|事件)(?:\s+(?:for|about)\s+([^\s,，。）)]+))?(?:\s+(?:in|from|namespace=?)\s*([^\s,，。）)]+))?`),
			"k8s_scale":    regexp.MustCompile(`(?i)(?:scale|扩容|缩容|调整副本)\s+(deployment|statefulset|deploy)\s+([^\s,，。）)]+)\s+(?:to|为|到)\s+(\d+)`),
			"k8s_restart":  regexp.MustCompile(`(?i)(?:restart|重启)\s+(deployment|statefulset|pod|deploy)\s+([^\s,，。）)]+)`),
			"k8s_rollout":  regexp.MustCompile(`(?i)(?:rollout|发布|回滚)\s+(undo|restart|status|回滚|重启|状态)\s+(deployment|deploy)\s+([^\s,，。）)]+)`),
			"k8s_status":   regexp.MustCompile(`(?i)(?:status|状态)\s+(?:of\s+)?([^\s,，。）)]+)`),
			"k8s_top":      regexp.MustCompile(`(?i)(?:top|资源|使用率|监控)\s+(pods?|nodes?|节点|pod)(?:\s+(?:in|from|namespace=?)\s*([^\s,，。）)]+))?`),
		},
	}
}

// Parse attempts to parse tool calls from name and arguments.
func (fp *FallbackParser) Parse(name, args string) []ParsedToolCall {
	// If name looks like a tool name, try to parse args
	if strings.HasPrefix(name, "k8s_") || strings.HasPrefix(name, "tool_") {
		argsMap, err := fp.parseArgsByType(name, args)
		if err == nil {
			return []ParsedToolCall{
				{
					Name:      name,
					Arguments: argsMap,
					RawJSON:   args,
				},
			}
		}
	}

	// Try to match patterns in the content
	for toolName, pattern := range fp.toolPatterns {
		matches := pattern.FindAllStringSubmatch(args, -1)
		for _, match := range matches {
			argsMap := fp.extractArgsFromMatch(toolName, match)
			if len(argsMap) > 0 {
				return []ParsedToolCall{
					{
						Name:      toolName,
						Arguments: argsMap,
						RawJSON:   match[0],
					},
				}
			}
		}
	}

	return nil
}

// ParseFromText attempts to parse tool calls from free-form text content.
func (fp *FallbackParser) ParseFromText(text string) []ParsedToolCall {
	if text == "" {
		return nil
	}

	toolCalls := []ParsedToolCall{}

	// Try to match each tool pattern against the full text
	for toolName, pattern := range fp.toolPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			argsMap := fp.extractArgsFromMatch(toolName, match)
			if len(argsMap) > 0 {
				toolCalls = append(toolCalls, ParsedToolCall{
					Name:      toolName,
					Arguments: argsMap,
					RawJSON:   match[0],
				})
			}
		}
	}

	return toolCalls
}

// parseArgsByType parses arguments based on tool type.
func (fp *FallbackParser) parseArgsByType(toolName, args string) (map[string]any, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return map[string]any{}, nil
	}

	// Try JSON first
	var argsMap map[string]any
	if err := json.Unmarshal([]byte(args), &argsMap); err == nil {
		return argsMap, nil
	}

	// Try simple format
	return fp.parseSimpleFormat(args), nil
}

// parseSimpleFormat parses simple argument formats.
func (fp *FallbackParser) parseSimpleFormat(args string) map[string]any {
	result := map[string]any{}

	// Split by comma, semicolon, or space
	separators := []string{",", ";", " "}
	for _, sep := range separators {
		if strings.Contains(args, sep) {
			parts := strings.Split(args, sep)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.Contains(part, "=") {
					kv := strings.SplitN(part, "=", 2)
					if len(kv) == 2 {
						result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				} else if strings.Contains(part, ":") {
					kv := strings.SplitN(part, ":", 2)
					if len(kv) == 2 {
						result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
					}
				}
			}
			return result
		}
	}

	// Single value - treat as name
	result["name"] = args
	return result
}

// extractArgsFromMatch extracts arguments from regex match.
func (fp *FallbackParser) extractArgsFromMatch(toolName string, match []string) map[string]any {
	args := map[string]any{}

	switch toolName {
	case "k8s_get":
		if len(match) >= 3 {
			args["resource_type"] = strings.ToLower(match[1])
			args["name"] = match[2]
			if len(match) >= 4 && match[3] != "" {
				args["namespace"] = match[3]
			}
		}
	case "k8s_describe":
		if len(match) >= 3 {
			args["resource_type"] = strings.ToLower(match[1])
			args["name"] = match[2]
			if len(match) >= 4 && match[3] != "" {
				args["namespace"] = match[3]
			}
		}
	case "k8s_logs":
		if len(match) >= 2 {
			args["pod_name"] = match[1]
			if len(match) >= 3 && match[2] != "" {
				args["namespace"] = match[2]
			}
		}
	case "k8s_events":
		if len(match) >= 3 {
			if match[1] != "" {
				args["name"] = match[1]
			}
			if match[2] != "" {
				args["namespace"] = match[2]
			}
		}
	case "k8s_scale":
		if len(match) >= 4 {
			args["resource_type"] = strings.ToLower(match[1])
			args["name"] = match[2]
			args["replicas"] = match[3]
		}
	case "k8s_restart":
		if len(match) >= 3 {
			args["resource_type"] = strings.ToLower(match[1])
			args["name"] = match[2]
		}
	case "k8s_rollout":
		if len(match) >= 4 {
			args["action"] = strings.ToLower(match[1])
			args["resource_type"] = strings.ToLower(match[2])
			args["name"] = match[3]
		}
	case "k8s_status":
		if len(match) >= 2 {
			args["name"] = match[1]
		}
	case "k8s_top":
		if len(match) >= 2 {
			args["resource_type"] = strings.ToLower(match[1])
			if len(match) >= 3 && match[2] != "" {
				args["namespace"] = match[2]
			}
		}
	}

	return args
}

// ===========================================================================
// Argument Validator
// ===========================================================================

// ArgumentValidator validates tool arguments.
type ArgumentValidator struct {
	requiredFields map[string][]string
}

// NewArgumentValidator creates a new argument validator.
func NewArgumentValidator() *ArgumentValidator {
	return &ArgumentValidator{
		requiredFields: map[string][]string{
			"k8s_get":        []string{"resource_type"},
			"k8s_describe":   []string{"resource_type", "name"},
			"k8s_logs":       []string{"pod_name"},
			"k8s_scale":      []string{"resource_type", "name", "replicas"},
			"k8s_restart":    []string{"resource_type", "name"},
			"k8s_patch":      []string{"resource_type", "name", "patch_data"},
			"k8s_rollout":    []string{"action", "resource_type", "name"},
		},
	}
}

// Validate validates arguments for a given tool.
func (av *ArgumentValidator) Validate(toolName string, args map[string]any) error {
	required, ok := av.requiredFields[toolName]
	if !ok {
		return nil // No validation rules for this tool
	}

	missing := []string{}
	for _, field := range required {
		if _, ok := args[field]; !ok {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required arguments: %s", strings.Join(missing, ", "))
	}

	return nil
}

// ValidateAndSuggest validates and provides suggestions for missing arguments.
func (av *ArgumentValidator) ValidateAndSuggest(toolName string, args map[string]any) (error, []string) {
	err := av.Validate(toolName, args)
	if err == nil {
		return nil, nil
	}

	// Parse missing fields from error
	missingStr := strings.TrimPrefix(err.Error(), "missing required arguments: ")
	missing := strings.Split(missingStr, ", ")

	suggestions := []string{}
	for _, field := range missing {
		suggestion := av.getSuggestion(toolName, field)
		suggestions = append(suggestions, suggestion)
	}

	return err, suggestions
}

// getSuggestion provides a suggestion for a missing field.
func (av *ArgumentValidator) getSuggestion(toolName, field string) string {
	switch field {
	case "resource_type":
		return "Specify resource type: pod, deployment, service, configmap, ingress"
	case "name":
		return "Provide the resource name"
	case "namespace":
		return "Specify the namespace (defaults to 'default')"
	case "replicas":
		return "Specify the target replica count (number)"
	case "pod_name":
		return "Specify the pod name"
	case "patch_data":
		return "Provide JSON or YAML patch data"
	case "action":
		return "Specify action: undo, restart, status"
	default:
		return fmt.Sprintf("Provide value for '%s'", field)
	}
}