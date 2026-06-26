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
		case tea.KeyLeft, tea.KeyTab:
			c.selectedIdx = (c.selectedIdx + 1) % len(c.buttons)
			return c, nil
		case tea.KeyRight, tea.KeyShiftTab:
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

	riskColor := c.styles.Theme.GetRiskColor(string(c.riskLevel))

	riskBadge := lipgloss.NewStyle().
		Foreground(riskColor).
		Bold(true).
		Padding(0, 1).
		Render(fmt.Sprintf("风险等级: %s", c.riskLevel))

	header := lipgloss.JoinHorizontal(lipgloss.Top,
		c.styles.Title.Render("⚠️  命令预览"),
		"  ",
		riskBadge,
	)

	cmdBlock := c.styles.CodeBlock.Render(c.command)

	descSection := ""
	if c.description != "" {
		descSection = c.styles.Subtitle.Render("操作说明:") + "\n" + c.description + "\n\n"
	}

	impactSection := ""
	if c.impactAnalysis != "" {
		impactSection = c.styles.Subtitle.Render("影响面分析:") + "\n" + c.impactAnalysis + "\n\n"
	}

	preflightSection := ""
	if len(c.preflight) > 0 {
		checks := []string{c.styles.Subtitle.Render("Pre-flight 检查:")}
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
		preflightSection = strings.Join(checks, "\n") + "\n\n"
	}

	buttons := make([]string, len(c.buttons))
	for i, btn := range c.buttons {
		if i == c.selectedIdx {
			buttons[i] = c.styles.ButtonActive.Render(btn)
		} else {
			buttons[i] = c.styles.Button.Render(btn)
		}
	}
	buttonsRow := lipgloss.JoinHorizontal(lipgloss.Left, buttons...)

	content := header + "\n\n" +
		descSection +
		cmdBlock + "\n" +
		impactSection +
		preflightSection +
		c.styles.Help.Render("按 Y 确认执行，N 或 Esc 取消") + "\n\n" +
		buttonsRow

	return c.styles.BorderActive.Render(content)
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
