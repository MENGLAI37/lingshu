package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type RiskLevel string

const (
	RiskL0 RiskLevel = "L0"
	RiskL1 RiskLevel = "L1"
	RiskL2 RiskLevel = "L2"
	RiskL3 RiskLevel = "L3"
	RiskL4 RiskLevel = "L4"
)

type CommandPreview struct {
	styles        *styles.Styles
	command       string
	description   string
	riskLevel     RiskLevel
	impactAnalysis string
	preflight     []PreflightCheck
	visible       bool
	confirmed     bool
	cancelled     bool
	selectedIdx   int
	buttons       []string
	width         int
	scrollPos     int
	showFullImpact bool
}

type PreflightCheck struct {
	Name    string
	Passed  bool
	Detail  string
}

type ConfirmApprovedMsg struct {
	Command string
}

type ConfirmCancelledMsg struct{}

func NewCommandPreview(s *styles.Styles) *CommandPreview {
	return &CommandPreview{
		styles:    s,
		visible:   false,
		confirmed: false,
		cancelled: false,
		buttons:   []string{"[Y] 确认", "[N] 取消"},
		selectedIdx: 0,
	}
}

func (c *CommandPreview) Init() tea.Cmd {
	return nil
}

func (c *CommandPreview) Update(msg tea.Msg) (*CommandPreview, tea.Cmd) {
	if !c.visible {
		return c, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyLeft, tea.KeyShiftTab:
			c.selectedIdx = (c.selectedIdx + 1) % len(c.buttons)
			return c, nil
		case tea.KeyRight, tea.KeyTab:
			c.selectedIdx = (c.selectedIdx - 1 + len(c.buttons)) % len(c.buttons)
			return c, nil
		case tea.KeyEnter:
			if c.selectedIdx == 0 {
				c.confirmed = true
				c.visible = false
				return c, func() tea.Msg {
					return ConfirmApprovedMsg{Command: c.command}
				}
			} else {
				c.cancelled = true
				c.visible = false
				return c, func() tea.Msg {
					return ConfirmCancelledMsg{}
				}
			}
		case tea.KeyRunes:
			if len(msg.Runes) > 0 {
				switch strings.ToLower(string(msg.Runes[0])) {
				case "y":
					c.confirmed = true
					c.visible = false
					return c, func() tea.Msg {
						return ConfirmApprovedMsg{Command: c.command}
					}
				case "n":
					c.cancelled = true
					c.visible = false
					return c, func() tea.Msg {
						return ConfirmCancelledMsg{}
					}
				case "v":
					// Toggle full impact analysis view
					c.showFullImpact = !c.showFullImpact
					return c, nil
				case "j", "down":
					// Scroll down (for content overflow)
					c.scrollPos++
					return c, nil
				case "k", "up":
					// Scroll up (for content overflow)
					if c.scrollPos > 0 {
						c.scrollPos--
					}
					return c, nil
				}
			}
		case tea.KeyEsc:
			c.cancelled = true
			c.visible = false
			return c, func() tea.Msg {
				return ConfirmCancelledMsg{}
			}
		}
	}

	return c, nil
}

func (c *CommandPreview) View() string {
	if !c.visible {
		return ""
	}

	panelWidth := 70
	if c.width > 0 {
		panelWidth = c.width - 10
		if panelWidth < 60 {
			panelWidth = 60
		}
		if panelWidth > 100 {
			panelWidth = 100
		}
	}

	riskColor := c.styles.Theme.GetRiskColor(string(c.riskLevel))

	riskBadge := lipgloss.NewStyle().
		Foreground(riskColor).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf("[%s]", c.riskLevel))

	title := c.styles.Title.Render("⚠️  命令预览")
	separator := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Border).
		Render(strings.Repeat("─", panelWidth-4))

	cmdBlock := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Foreground).
		Background(c.styles.Theme.Selection).
		Padding(1, 2).
		Margin(1, 0).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.styles.Theme.Border).
		Width(panelWidth - 4).
		Render(c.command)

	var sections []string

	if c.description != "" {
		descTitle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Secondary).
			Bold(true).
			Render("📝 操作说明")
		sections = append(sections, descTitle+"\n  "+c.description)
	}

	if c.impactAnalysis != "" {
		impactTitle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Warning).
			Bold(true).
			Render("🔍 影响面分析")
		impactContent := c.impactAnalysis
		if !c.showFullImpact {
			lines := strings.Split(impactContent, "\n")
			if len(lines) > 3 {
				impactContent = strings.Join(lines[:3], "\n") + "\n  ... (按 v 查看完整内容)"
			}
		}
		sections = append(sections, impactTitle+"\n  "+impactContent)
	}

	if len(c.preflight) > 0 {
		checkTitle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Info).
			Bold(true).
			Render("✅ Pre-flight 检查")
		var checks []string
		for _, check := range c.preflight {
			status := "✓"
			statusStyle := c.styles.StatusOK
			if !check.Passed {
				status = "✗"
				statusStyle = c.styles.StatusError
			}
			checks = append(checks, fmt.Sprintf("  %s %s: %s",
				statusStyle.Render(status),
				check.Name,
				check.Detail,
			))
		}
		sections = append(sections, checkTitle+"\n"+strings.Join(checks, "\n"))
	}

	sectionsContent := strings.Join(sections, "\n\n")

	buttons := make([]string, len(c.buttons))
	for i, btn := range c.buttons {
		if i == c.selectedIdx {
			buttons[i] = c.styles.ButtonActive.Render(btn)
		} else {
			buttons[i] = c.styles.Button.Render(btn)
		}
	}
	buttonsRow := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(panelWidth - 4).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, buttons...))

	helpText := "Y/Enter 确认 | N/Esc 取消 | Tab/←→ 切换 | v 查看完整影响 | j/k 滚动"
	footer := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Muted).
		Italic(true).
		Align(lipgloss.Center).
		Width(panelWidth - 4).
		Render(helpText)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", riskBadge)

	content := headerRow + "\n" + separator + "\n\n" +
		cmdBlock + "\n" +
		sectionsContent + "\n\n" +
		footer + "\n\n" +
		buttonsRow

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.styles.Theme.Primary).
		Padding(1, 2).
		Width(panelWidth)

	return borderStyle.Render(content)
}

func (c *CommandPreview) Show(cmd string, risk RiskLevel, desc string, impact string, preflight []PreflightCheck) {
	c.command = cmd
	c.riskLevel = risk
	c.description = desc
	c.impactAnalysis = impact
	c.preflight = preflight
	c.visible = true
	c.confirmed = false
	c.cancelled = false
	c.selectedIdx = 0
	c.scrollPos = 0
	c.showFullImpact = false
}

func (c *CommandPreview) Hide() {
	c.visible = false
}

func (c *CommandPreview) Visible() bool {
	return c.visible
}

func (c *CommandPreview) Confirmed() bool {
	return c.confirmed
}

func (c *CommandPreview) Cancelled() bool {
	return c.cancelled
}

func (c *CommandPreview) SetWidth(w int) {
	c.width = w
}

func (c *CommandPreview) Command() string {
	return c.command
}

func (c *CommandPreview) RiskLevel() RiskLevel {
	return c.riskLevel
}
