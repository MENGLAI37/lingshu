package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/ops-ai/pkg/tui/theme"
)

type Styles struct {
	Theme *theme.Theme

	App          lipgloss.Style
	Header       lipgloss.Style
	Footer       lipgloss.Style
	Body         lipgloss.Style

	Title        lipgloss.Style
	Subtitle     lipgloss.Style
	Help         lipgloss.Style

	InputPrompt  lipgloss.Style
	InputCursor  lipgloss.Style
	InputText    lipgloss.Style

	UserMessage  lipgloss.Style
	AIMessage    lipgloss.Style
	SystemMessage lipgloss.Style

	Border       lipgloss.Style
	BorderActive lipgloss.Style

	Button       lipgloss.Style
	ButtonActive lipgloss.Style

	StatusOK     lipgloss.Style
	StatusWarn   lipgloss.Style
	StatusError  lipgloss.Style
	StatusInfo   lipgloss.Style

	CodeBlock    lipgloss.Style
	InlineCode   lipgloss.Style
}

func NewStyles(t *theme.Theme) *Styles {
	s := &Styles{Theme: t}

	s.App = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Background(t.Background)

	s.Header = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Padding(0, 1)

	s.Footer = lipgloss.NewStyle().
		Foreground(t.Muted).
		Background(t.Selection).
		Padding(0, 1)

	s.Body = lipgloss.NewStyle().
		Padding(1, 2)

	s.Title = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(t.Secondary).
		MarginBottom(1)

	s.Help = lipgloss.NewStyle().
		Foreground(t.Muted).
		Italic(true)

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	s.InputCursor = lipgloss.NewStyle().
		Foreground(t.Cursor).
		Bold(true)

	s.InputText = lipgloss.NewStyle().
		Foreground(t.Foreground)

	s.UserMessage = lipgloss.NewStyle().
		Foreground(t.Primary).
		PaddingLeft(2)

	s.AIMessage = lipgloss.NewStyle().
		Foreground(t.Foreground).
		PaddingLeft(2)

	s.SystemMessage = lipgloss.NewStyle().
		Foreground(t.Muted).
		Italic(true).
		PaddingLeft(2)

	s.Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2)

	s.BorderActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2)

	s.Button = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Background(t.Selection).
		Padding(0, 2).
		MarginRight(1)

	s.ButtonActive = lipgloss.NewStyle().
		Foreground(t.Background).
		Background(t.Primary).
		Padding(0, 2).
		MarginRight(1).
		Bold(true)

	s.StatusOK = lipgloss.NewStyle().
		Foreground(t.Success).
		Bold(true)

	s.StatusWarn = lipgloss.NewStyle().
		Foreground(t.Warning).
		Bold(true)

	s.StatusError = lipgloss.NewStyle().
		Foreground(t.Error).
		Bold(true)

	s.StatusInfo = lipgloss.NewStyle().
		Foreground(t.Info).
		Bold(true)

	s.CodeBlock = lipgloss.NewStyle().
		Foreground(t.Foreground).
		Background(t.Selection).
		Padding(1, 2).
		Margin(1, 0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)

	s.InlineCode = lipgloss.NewStyle().
		Foreground(t.Accent).
		Background(t.Selection).
		Padding(0, 1)

	return s
}

func (s *Styles) RiskBadge(level string) string {
	color := s.Theme.GetRiskColor(level)
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Padding(0, 1).
		Render("[" + level + "]")
}
