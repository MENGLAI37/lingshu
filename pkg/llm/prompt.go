package llm

import (
	"fmt"
	"strings"
	"sync"
)

// ===========================================================================
// Prompt Template Engine
// ===========================================================================

// TemplateVersion holds version info for a template.
type TemplateVersion struct {
	Version string
	Hash    string
}

// PromptTemplate represents a compiled prompt template.
type PromptTemplate struct {
	Name     string
	Template string
	Version  string
	Vars     []string // extracted variable names
}

// DefaultPromptEngine is the default template engine implementation.
type DefaultPromptEngine struct {
	templates map[string]*PromptTemplate
	versions  map[string]string
	mu        sync.RWMutex
}

// NewPromptEngine creates a new prompt template engine.
func NewPromptEngine() *DefaultPromptEngine {
	return &DefaultPromptEngine{
		templates: make(map[string]*PromptTemplate),
		versions:  make(map[string]string),
	}
}

// Register registers a new template.
func (e *DefaultPromptEngine) Register(name, template string) error {
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	vars := extractVars(template)

	e.mu.Lock()
	defer e.mu.Unlock()

	// Version bump: if template changed, increment version
	oldTpl, exists := e.templates[name]
	version := "v1.0.0"
	if exists && oldTpl.Template != template {
		version = bumpVersion(oldTpl.Version)
	} else if exists {
		version = oldTpl.Version
	}

	e.templates[name] = &PromptTemplate{
		Name:     name,
		Template: template,
		Version:  version,
		Vars:     vars,
	}
	e.versions[name] = version

	return nil
}

// Render renders a template with variables.
func (e *DefaultPromptEngine) Render(name string, vars map[string]string) (string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tpl, ok := e.templates[name]
	if !ok {
		return "", fmt.Errorf("template %q not found", name)
	}

	result := tpl.Template
	for key, val := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, val)
	}

	// Check for unreplaced variables
	unreplaced := extractVars(result)
	if len(unreplaced) > 0 {
		// Allow partial renders but could warn
	}

	return result, nil
}

// GetVersion returns the version of a template.
func (e *DefaultPromptEngine) GetVersion(name string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.versions[name]
}

// GetTemplate returns a template by name.
func (e *DefaultPromptEngine) GetTemplate(name string) (*PromptTemplate, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	tpl, ok := e.templates[name]
	if !ok {
		return nil, fmt.Errorf("template %q not found", name)
	}
	return tpl, nil
}

// ListTemplates returns all registered template names.
func (e *DefaultPromptEngine) ListTemplates() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.templates))
	for name := range e.templates {
		names = append(names, name)
	}
	return names
}

// ===========================================================================
// Built-in Templates
// ===========================================================================

// RegisterBuiltinTemplates registers common prompt templates.
func (e *DefaultPromptEngine) RegisterBuiltinTemplates() {
	_ = e.Register("diagnose", `You are a Kubernetes SRE assistant. Analyze the following issue and suggest root causes.

Cluster: {{cluster}}
Namespace: {{namespace}}
Issue: {{issue}}

Provide your analysis in structured format with:
1. Root cause hypothesis (ranked by likelihood)
2. Supporting evidence
3. Recommended next steps`)

	_ = e.Register("runbook_search", `You are a helpful assistant. Based on the following runbook context, answer the user's question.

Context:
{{context}}

Question: {{question}}`)

	_ = e.Register("tool_call", `You have access to tools. Choose the most appropriate tool to solve the user's request.

Available tools:
{{tools}}

User request: {{request}}

Respond with a tool call if needed, or with a direct answer.`)

	_ = e.Register("safety_check", `Evaluate the risk level of the following Kubernetes operation.

Operation: {{operation}}
Resource: {{resource}}
Namespace: {{namespace}}

Classify the risk as: L0 (read-only), L1 (safe write), L2 (moderate risk), L3 (high risk), L4 (destructive).
Provide a brief justification.`)
}

// ===========================================================================
// Helpers
// ===========================================================================

func extractVars(template string) []string {
	var vars []string
	start := 0
	for {
		idx := strings.Index(template[start:], "{{")
		if idx == -1 {
			break
		}
		idx += start
		end := strings.Index(template[idx:], "}}")
		if end == -1 {
			break
		}
		end += idx
		varName := strings.TrimSpace(template[idx+2 : end])
		if varName != "" {
			vars = append(vars, varName)
		}
		start = end + 2
	}
	return vars
}

func bumpVersion(v string) string {
	// Simple semver bump for patch version
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "v1.0.0"
	}
	// Increment patch
	var patch int
	_, _ = fmt.Sscanf(parts[2], "%d", &patch)
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}
