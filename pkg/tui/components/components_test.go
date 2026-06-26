package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/ops-ai/pkg/tui/styles"
	"github.com/lingshu/ops-ai/pkg/tui/theme"
)

func getTestStyles() *styles.Styles {
	t := theme.GetTheme(theme.ThemeDark)
	return styles.NewStyles(t)
}

func TestChatView(t *testing.T) {
	s := getTestStyles()
	cv := NewChatView(s)

	cv.SetWidth(80)
	cv.SetHeight(20)

	cv.AddMessage(ChatMessage{
		Role:    RoleUser,
		Content: "Hello, world!",
	})

	if len(cv.Messages()) != 1 {
		t.Errorf("Expected 1 message, got %d", len(cv.Messages()))
	}

	cv.UpdateLastMessage("AI response", false)
	if len(cv.Messages()) != 2 {
		t.Errorf("Expected 2 messages (user + AI), got %d", len(cv.Messages()))
	}
	if cv.Messages()[1].Role != RoleAI {
		t.Errorf("Expected second message to be AI role, got %s", cv.Messages()[1].Role)
	}
	if cv.Messages()[1].Content != "AI response" {
		t.Errorf("Expected 'AI response', got '%s'", cv.Messages()[1].Content)
	}

	cv.AppendToLastMessage(" more content")
	if cv.Messages()[1].Content != "AI response more content" {
		t.Errorf("Expected 'AI response more content', got '%s'", cv.Messages()[1].Content)
	}

	cv.FinishStreaming()
	if cv.Messages()[1].Streaming {
		t.Error("Expected streaming to be false")
	}

	view := cv.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}
}

func TestChatViewScroll(t *testing.T) {
	s := getTestStyles()
	cv := NewChatView(s)
	cv.SetWidth(80)
	cv.SetHeight(5)

	for i := 0; i < 50; i++ {
		cv.AddMessage(ChatMessage{
			Role:    RoleUser,
			Content: "Message " + string(rune('0'+i%10)),
		})
	}

	cv.ScrollUp(10)
	cv.ScrollDown(5)

	cv.Clear()
	if len(cv.Messages()) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(cv.Messages()))
	}
}

func TestStreamRenderer(t *testing.T) {
	s := getTestStyles()
	sr := NewStreamRenderer(s)

	sr.SetWidth(80)
	sr.SetHeight(10)

	sr.SetStreaming(true)
	if !sr.Streaming() {
		t.Error("Expected streaming to be true")
	}

	sr.AppendChunk("Hello ")
	sr.AppendChunk("world!")
	if sr.Content() != "Hello world!" {
		t.Errorf("Expected 'Hello world!', got '%s'", sr.Content())
	}

	sr.SetContent("New content")
	if sr.Content() != "New content" {
		t.Errorf("Expected 'New content', got '%s'", sr.Content())
	}

	sr.SetStreaming(false)
	view := sr.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}

	sr.Clear()
	if sr.Content() != "" {
		t.Error("Expected empty content after clear")
	}
}

func TestCommandPreview(t *testing.T) {
	s := getTestStyles()
	cp := NewCommandPreview(s)

	if cp.Visible() {
		t.Error("Expected command preview to be hidden initially")
	}

	cp.Show(
		"kubectl get pods",
		RiskL1,
		"List all pods",
		"Read-only operation",
		[]PreflightCheck{
			{Name: "Cluster connection", Passed: true, Detail: "OK"},
		},
	)

	if !cp.Visible() {
		t.Error("Expected command preview to be visible after Show()")
	}

	view := cp.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}

	cp.Hide()
	if cp.Visible() {
		t.Error("Expected command preview to be hidden after Hide()")
	}
}

func TestStatusBar(t *testing.T) {
	s := getTestStyles()
	sb := NewStatusBar(s)

	sb.SetCluster("test-cluster")
	sb.SetNamespace("test-ns")
	sb.SetEnvironment("production")

	sb.AddTokens(1000)
	sb.AddCost(0.05)

	view := sb.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}
}

func TestMultiLineInput(t *testing.T) {
	s := getTestStyles()
	ml := NewMultiLineInput(s)

	ml.SetWidth(80)
	ml.SetHeight(3)

	ml.SetValue("test input")
	if ml.Value() != "test input" {
		t.Errorf("Expected 'test input', got '%s'", ml.Value())
	}

	ml.SetPlaceholder("Type here...")

	view := ml.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}

	cmd := ml.Focus()
	if cmd == nil {
		t.Error("Expected a cmd from Focus()")
	}

	ml.Blur()
}

func TestHighlightedRenderer(t *testing.T) {
	s := getTestStyles()
	hr := NewHighlightedRenderer(s)

	hr.SetWidth(80)
	hr.SetHeight(20)

	jsonContent := `{
  "name": "test",
  "value": 123,
  "active": true
}`
	hr.SetContent(jsonContent, ContentJSON, "test.json")
	hr.Show()

	if !hr.Visible() {
		t.Error("Expected visible after Show()")
	}

	view := hr.View()
	if len(view) == 0 {
		t.Error("Expected non-empty view")
	}

	yamlContent := `name: test
value: 123
active: true
items:
  - one
  - two`
	hr.SetContent(yamlContent, ContentYAML, "test.yaml")

	tableContent := `| Name | Value |
|------|-------|
| foo  | bar   |
| baz  | qux   |`
	hr.SetContent(tableContent, ContentTable, "test table")

	hr.Hide()
	if hr.Visible() {
		t.Error("Expected hidden after Hide()")
	}
}

func TestWindowSizeHandling(t *testing.T) {
	s := getTestStyles()

	cv := NewChatView(s)
	cv.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	sr := NewStreamRenderer(s)
	sr.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	cp := NewCommandPreview(s)
	cp.Show("test", RiskL0, "", "", nil)
	cp.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	hr := NewHighlightedRenderer(s)
	hr.Show()
	hr.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
}

func TestUtils(t *testing.T) {
	if maxInt(5, 10) != 10 {
		t.Error("maxInt(5, 10) should be 10")
	}
	if maxInt(10, 5) != 10 {
		t.Error("maxInt(10, 5) should be 10")
	}
	if minInt(5, 10) != 5 {
		t.Error("minInt(5, 10) should be 5")
	}
	if minInt(10, 5) != 5 {
		t.Error("minInt(10, 5) should be 5")
	}
	if trimSpace("  hello  ") != "hello" {
		t.Error("trimSpace should trim spaces")
	}
}
