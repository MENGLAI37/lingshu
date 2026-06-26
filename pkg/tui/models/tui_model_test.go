package models

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/lingshu/pkg/tui/components"
	"github.com/lingshu/lingshu/pkg/tui/theme"
)

func TestNewTUIModel(t *testing.T) {
	m := NewTUIModel()

	if m == nil {
		t.Fatal("Expected non-nil model")
	}

	if m.currentPage != PageChat {
		t.Errorf("Expected initial page to be PageChat, got %s", m.currentPage)
	}

	if m.chatView == nil {
		t.Error("Expected chatView to be initialized")
	}

	if m.input == nil {
		t.Error("Expected input to be initialized")
	}

	if m.statusBar == nil {
		t.Error("Expected statusBar to be initialized")
	}

	if m.commandPreview == nil {
		t.Error("Expected commandPreview to be initialized")
	}

	if m.highlighted == nil {
		t.Error("Expected highlighted to be initialized")
	}
}

func TestTUIModelInit(t *testing.T) {
	m := NewTUIModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected non-nil cmd from Init()")
	}
}

func TestTUIModelWindowSize(t *testing.T) {
	m := NewTUIModel()
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	tm := model.(*TUIModel)
	if tm.width != 120 {
		t.Errorf("Expected width 120, got %d", tm.width)
	}
	if tm.height != 40 {
		t.Errorf("Expected height 40, got %d", tm.height)
	}
}

func TestTUIModelView(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	view := m.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}
}

func TestTUIModelSetCluster(t *testing.T) {
	m := NewTUIModel()
	m.SetCluster("test-cluster")
	if m.cluster != "test-cluster" {
		t.Errorf("Expected cluster 'test-cluster', got '%s'", m.cluster)
	}
}

func TestTUIModelSetNamespace(t *testing.T) {
	m := NewTUIModel()
	m.SetNamespace("test-ns")
	if m.namespace != "test-ns" {
		t.Errorf("Expected namespace 'test-ns', got '%s'", m.namespace)
	}
}

func TestTUIModelSetEnvironment(t *testing.T) {
	m := NewTUIModel()
	m.SetEnvironment("staging")
	if m.environment != "staging" {
		t.Errorf("Expected environment 'staging', got '%s'", m.environment)
	}
}

func TestTUIModelSetTheme(t *testing.T) {
	m := NewTUIModel()
	m.SetTheme(theme.ThemeLight)

	lightTheme := theme.GetTheme(theme.ThemeLight)
	if m.theme.Primary != lightTheme.Primary {
		t.Error("Expected theme to be changed to light")
	}

	m.SetTheme(theme.ThemeDark)
	darkTheme := theme.GetTheme(theme.ThemeDark)
	if m.theme.Primary != darkTheme.Primary {
		t.Error("Expected theme to be changed to dark")
	}
}

func TestTUIModelHelpOverlay(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	m.showHelp = true
	view := m.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view with help overlay")
	}
}

func TestTUIModelHandleUserInput(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	initialMsgCount := len(m.chatView.Messages())
	m.handleUserInput("Hello, AI!")

	if len(m.chatView.Messages()) != initialMsgCount+1 {
		t.Errorf("Expected %d messages, got %d", initialMsgCount+1, len(m.chatView.Messages()))
	}

	lastMsg := m.chatView.Messages()[len(m.chatView.Messages())-1]
	if lastMsg.Role != components.RoleUser {
		t.Errorf("Expected user role, got %s", lastMsg.Role)
	}
	if lastMsg.Content != "Hello, AI!" {
		t.Errorf("Expected 'Hello, AI!', got '%s'", lastMsg.Content)
	}

	if !m.aiThinking {
		t.Error("Expected aiThinking to be true")
	}
}

func TestTUIModelHandleAIResponse(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	m.handleAIResponse(AIResponseMsg{
		Content: "Hello!",
		Done:    false,
	})

	if !m.streaming {
		t.Error("Expected streaming to be true")
	}

	m.handleAIResponse(AIResponseMsg{
		Content: " How can I help?",
		Done:    true,
	})

	if m.streaming {
		t.Error("Expected streaming to be false after done")
	}
	if m.aiThinking {
		t.Error("Expected aiThinking to be false after done")
	}
}

func TestTUIModelToolCallRequest(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	msg := ToolCallRequestMsg{
		Tool:      "kubectl",
		Command:   "kubectl get pods",
		RiskLevel: components.RiskL1,
		Desc:      "List pods",
		Impact:    "Read-only",
		Preflight: []components.PreflightCheck{},
	}

	m.handleToolCallRequest(msg)

	if !m.commandPreview.Visible() {
		t.Error("Expected command preview to be visible")
	}
}

func TestTUIModelConfirmApproved(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	m.handleConfirmApproved("kubectl get pods")

	msgs := m.chatView.Messages()
	if len(msgs) < 2 {
		t.Errorf("Expected at least 2 messages, got %d", len(msgs))
	}
}

func TestTUIModelConfirmCancelled(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	m.aiThinking = true
	m.streaming = true

	m.handleConfirmCancelled()

	if m.aiThinking {
		t.Error("Expected aiThinking to be false after cancel")
	}
	if m.streaming {
		t.Error("Expected streaming to be false after cancel")
	}
}

func TestTUIModelShowHighlight(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	_, _ = m.Update(ShowHighlightMsg{
		Content:     `{"key": "value"}`,
		ContentType: components.ContentJSON,
		Title:       "test.json",
	})

	if !m.highlighted.Visible() {
		t.Error("Expected highlighted renderer to be visible")
	}
}

func TestTUIModelInputSubmitted(t *testing.T) {
	m := NewTUIModel()
	m.width = 80
	m.height = 24

	initialCount := len(m.chatView.Messages())
	_, _ = m.Update(components.InputSubmittedMsg{Value: "test input"})

	if len(m.chatView.Messages()) != initialCount+1 {
		t.Errorf("Expected message count to increase by 1")
	}
}
