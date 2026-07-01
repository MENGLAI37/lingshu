package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type MultiLineInput struct {
	textarea    textarea.Model
	styles      *styles.Styles
	history     []string
	historyIdx  int
	placeholder string //nolint:unused
	prompt      string
	width       int
	height      int
	focused     bool
}

type InputSubmittedMsg struct {
	Value string
}

func NewMultiLineInput(s *styles.Styles) *MultiLineInput {
	ta := textarea.New()
	ta.Placeholder = "输入你的问题... (Enter 发送, Shift+Enter 换行, Ctrl+P/N 历史记录)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(80)
	ta.SetHeight(3)

	return &MultiLineInput{
		textarea:   ta,
		styles:     s,
		history:    []string{},
		historyIdx: -1,
		prompt:     "❯ ",
	}
}

func (m *MultiLineInput) Init() tea.Cmd {
	return textarea.Blink
}

func (m *MultiLineInput) Update(msg tea.Msg) (*MultiLineInput, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.focused {
				if msg.Alt {
					m.textarea, cmd = m.textarea.Update(msg)
					return m, cmd
				}
				value := strings.TrimSpace(m.textarea.Value())
				if value != "" {
					m.addHistory(value)
					m.textarea.SetValue("")
					m.historyIdx = -1
					return m, func() tea.Msg {
						return InputSubmittedMsg{Value: value}
					}
				}
			}
		case tea.KeyUp, tea.KeyCtrlP:
			if m.focused && len(m.history) > 0 {
				if m.historyIdx < len(m.history)-1 {
					m.historyIdx++
					m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
				}
				return m, nil
			}
		case tea.KeyDown, tea.KeyCtrlN:
			if m.focused && len(m.history) > 0 {
				if m.historyIdx > 0 {
					m.historyIdx--
					m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
				} else if m.historyIdx == 0 {
					m.historyIdx = -1
					m.textarea.SetValue("")
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.textarea.SetWidth(msg.Width - 4)
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m *MultiLineInput) View() string {
	promptStyle := m.styles.InputPrompt
	if !m.focused {
		promptStyle = promptStyle.Foreground(m.styles.Theme.Muted)
	}

	prompt := promptStyle.Render(m.prompt)
	inputView := m.textarea.View()

	return lipgloss.JoinVertical(lipgloss.Left,
		prompt,
		inputView,
	)
}

func (m *MultiLineInput) Focus() tea.Cmd {
	m.focused = true
	return m.textarea.Focus()
}

func (m *MultiLineInput) Blur() {
	m.focused = false
	m.textarea.Blur()
}

// Focused returns whether the input is currently focused
func (m *MultiLineInput) Focused() bool {
	return m.focused
}

func (m *MultiLineInput) SetValue(v string) {
	m.textarea.SetValue(v)
}

func (m *MultiLineInput) Value() string {
	return m.textarea.Value()
}

func (m *MultiLineInput) SetPlaceholder(p string) {
	m.textarea.Placeholder = p
}

func (m *MultiLineInput) addHistory(cmd string) {
	if cmd == "" {
		return
	}
	if len(m.history) > 0 && m.history[len(m.history)-1] == cmd {
		return
	}
	m.history = append(m.history, cmd)
	if len(m.history) > 100 {
		m.history = m.history[1:]
	}
}

func (m *MultiLineInput) SetWidth(w int) {
	m.width = w
	m.textarea.SetWidth(w - 4)
}

func (m *MultiLineInput) SetHeight(h int) {
	m.height = h
	m.textarea.SetHeight(h)
}

func (m *MultiLineInput) History() []string {
	return m.history
}

func (m *MultiLineInput) ClearHistory() {
	m.history = []string{}
	m.historyIdx = -1
}
