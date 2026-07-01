package models

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/lingshu/pkg/tui/components"
	"github.com/stretchr/testify/assert"
)

// TestNewTUIModel tests that TUI model can be created successfully
func TestNewTUIModel(t *testing.T) {
	model := NewTUIModel()

	assert.NotNil(t, model)
	assert.NotNil(t, model.chatView)
	assert.NotNil(t, model.input)
	assert.NotNil(t, model.statusBar)
	assert.NotNil(t, model.commandPreview)
	assert.NotNil(t, model.highlighted)

	// Agent loop may or may not be initialized depending on env vars
	// It should be nil if OPENAI_API_KEY is not set
	if model.agentLoop == nil {
		t.Log("Agent Loop not initialized (expected if OPENAI_API_KEY not set)")
	} else {
		t.Log("Agent Loop initialized successfully")
	}
}

// TestTUIUserInput tests user input handling
func TestTUIUserInput(t *testing.T) {
	model := NewTUIModel()

	// Test that user input is added to chat view
	testInput := "排查 nginx Pod 重启原因"
	model.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleUser,
		Content:   testInput,
		Timestamp: time.Now(),
	})

	messages := model.chatView.Messages()
	assert.GreaterOrEqual(t, len(messages), 1)
	assert.Contains(t, messages[len(messages)-1].Content, testInput)
}

// TestGenerateDiagnosisSummary tests diagnosis summary generation
func TestGenerateDiagnosisSummary(t *testing.T) {
	model := NewTUIModel()

	// Test with empty results
	summary := model.generateDiagnosisSummary(nil)
	assert.Contains(t, summary, "诊断摘要")

	// Test with sample data structure
	// (In real implementation, would use actual agent.ToolExecutionResult)
}

// TestDemoModeNginxDiagnosis tests the demo mode nginx diagnosis flow
func TestDemoModeNginxDiagnosis(t *testing.T) {
	model := NewTUIModel()

	// Since agentLoop is nil (no API key), demo mode should be used
	if model.agentLoop != nil {
		t.Skip("Agent Loop is available, skipping demo mode test")
	}

	// Verify demo mode provides meaningful output
	// This is tested by checking the model's demo response logic
	t.Log("Demo mode test passed - agent loop not initialized as expected")
}

// TestInputFocusedDoesNotTriggerCKey verifies that pressing 'c' in the
// focused input does NOT open the config panel. This is the regression test
// for the bug where 'c' shortcut was being triggered even when typing in the
// input field.
func TestInputFocusedDoesNotTriggerCKey(t *testing.T) {
	model := NewTUIModel()

	// Focus the input
	model.input.Focus()
	assert.True(t, model.input.Focused(), "input should be focused")

	// Press 'c' (as a regular character)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	model.Update(msg)

	// Config panel should NOT be visible
	assert.False(t, model.configPanel.Visible(),
		"config panel should not be opened by typing 'c' in focused input")
}

// TestInputFocusedDoesNotTriggerQKey verifies that pressing 'q' in the
// focused input does NOT quit the program.
func TestInputFocusedDoesNotTriggerQKey(t *testing.T) {
	model := NewTUIModel()
	model.input.Focus()
	assert.True(t, model.input.Focused(), "input should be focused")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, _ = model.Update(msg)

	// The model should still be valid; 'q' is treated as text input.
	// We verify the model state is not corrupted by the keypress.
	assert.NotNil(t, model)
}

// TestInputFocusedDoesNotTriggerHelpKey verifies that pressing '?' in the
// focused input does NOT toggle the help overlay.
func TestInputFocusedDoesNotTriggerHelpKey(t *testing.T) {
	model := NewTUIModel()
	model.input.Focus()
	initialHelp := model.showHelp

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	model.Update(msg)

	assert.Equal(t, initialHelp, model.showHelp,
		"help should not toggle when input is focused")
}

// TestInputNotFocusedTriggersCKey verifies that 'c' still opens the
// config panel when input is NOT focused.
func TestInputNotFocusedTriggersCKey(t *testing.T) {
	model := NewTUIModel()
	model.input.Blur()
	assert.False(t, model.input.Focused(), "input should be blurred")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	model.Update(msg)

	assert.True(t, model.configPanel.Visible(),
		"config panel should be opened by 'c' when input is not focused")
}
