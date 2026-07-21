package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type ConfirmationModal struct {
	styles    *styles.Styles
	width     int
	height    int
	message   string
	riskLevel string
	toolName  string
	visible   bool
	cursorPos int
	confirmed bool
	token     string
}

func NewConfirmationModal(s *styles.Styles) *ConfirmationModal {
	return &ConfirmationModal{
		styles:    s,
		visible:   false,
		cursorPos: 0,
	}
}

func (m *ConfirmationModal) Show(message, riskLevel, toolName, token string) {
	m.message = message
	m.riskLevel = riskLevel
	m.toolName = toolName
	m.token = token
	m.visible = true
	m.cursorPos = 0
	m.confirmed = false
}

func (m *ConfirmationModal) Hide() {
	m.visible = false
}

func (m *ConfirmationModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *ConfirmationModal) IsVisible() bool {
	return m.visible
}

func (m *ConfirmationModal) Confirm() bool {
	m.confirmed = true
	return true
}

func (m *ConfirmationModal) Cancel() bool {
	m.confirmed = false
	return false
}

func (m *ConfirmationModal) GetToken() string {
	return m.token
}

func (m *ConfirmationModal) Update(msg tea.Msg) (*ConfirmationModal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.cursorPos == 0 {
				m.Confirm()
				return m, nil
			} else {
				m.Cancel()
				return m, nil
			}
		case tea.KeyUp, tea.KeyDown:
			m.cursorPos = (m.cursorPos + 1) % 2
			return m, nil
		case tea.KeyEsc:
			m.Cancel()
			return m, nil
		}
	}

	return m, nil
}

func (m *ConfirmationModal) View() string {
	if !m.visible {
		return ""
	}

	panelWidth := min(m.width-4, 80)
	panelHeight := min(m.height-4, 24)

	content := strings.Builder{}

	borderColor := m.styles.Theme.Border
	switch m.riskLevel {
	case "L2":
		borderColor = m.styles.Theme.Warning
	case "L3", "L4":
		borderColor = m.styles.Theme.Error
	}

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(panelWidth).
		Padding(1, 2)

	title := fmt.Sprintf("⚠️ 确认操作 - %s (%s)", m.toolName, m.riskLevel)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Theme.Primary).
		Align(lipgloss.Center).
		Width(panelWidth - 4)

	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	lines := strings.Split(m.message, "\n")
	for _, line := range lines {
		content.WriteString(line)
		content.WriteString("\n")
	}

	content.WriteString("\n")

	yesStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Theme.Error).
		Padding(0, 2)

	noStyle := lipgloss.NewStyle().
		Foreground(m.styles.Theme.Secondary).
		Padding(0, 2)

	if m.cursorPos == 0 {
		content.WriteString("[")
		content.WriteString(yesStyle.Render("确认"))
		content.WriteString("]  ")
		content.WriteString(" ")
		content.WriteString(noStyle.Render("取消"))
		content.WriteString(" ")
	} else {
		content.WriteString(" ")
		content.WriteString(yesStyle.Render("确认"))
		content.WriteString("  ")
		content.WriteString("[")
		content.WriteString(noStyle.Render("取消"))
		content.WriteString("]")
	}

	content.WriteString("\n")
	content.WriteString("(使用 ↑↓ 切换，Enter 确认，Esc 取消)")

	paddedContent := lipgloss.NewStyle().
		Width(panelWidth - 4).
		MaxHeight(panelHeight - 6).
		Render(content.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		borderStyle.Render(paddedContent),
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}