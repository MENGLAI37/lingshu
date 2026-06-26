package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type StatusBar struct {
	styles        *styles.Styles
	cluster       string
	namespace     string
	environment   string
	costUSD       float64
	tokensUsed    int
	tokensLimit   int
	sessionID     string
	uptime        time.Duration
	mode          string
	width         int
}

type StatusTickMsg struct{}

func NewStatusBar(s *styles.Styles) *StatusBar {
	return &StatusBar{
		styles:      s,
		cluster:     "default",
		namespace:   "default",
		environment: "production",
		costUSD:     0,
		tokensUsed:  0,
		tokensLimit: 100000,
		mode:        "normal",
	}
}

func (s *StatusBar) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return StatusTickMsg{}
	})
}

func (s *StatusBar) Update(msg tea.Msg) (*StatusBar, tea.Cmd) {
	switch msg := msg.(type) {
	case StatusTickMsg:
		s.uptime += time.Second
		return s, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return StatusTickMsg{}
		})

	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	}

	return s, nil
}

func (s *StatusBar) View() string {
	leftItems := []string{
		s.formatItem("集群", s.cluster, s.styles.Theme.Primary),
		s.formatItem("命名空间", s.namespace, s.styles.Theme.Info),
		s.formatItem("环境", s.environment, s.getEnvColor()),
	}

	rightItems := []string{
		s.formatItem("Token", fmt.Sprintf("%d/%d", s.tokensUsed, s.tokensLimit), s.styles.Theme.Warning),
		s.formatItem("成本", fmt.Sprintf("$%.4f", s.costUSD), s.styles.Theme.Accent),
		s.formatItem("运行时间", formatDuration(s.uptime), s.styles.Theme.Muted),
	}

	leftPart := strings.Join(leftItems, "  ")
	rightPart := strings.Join(rightItems, "  ")

	totalWidth := lipgloss.Width(leftPart) + lipgloss.Width(rightPart)
	padding := ""
	if s.width > totalWidth {
		padding = strings.Repeat(" ", s.width-totalWidth)
	}

	content := leftPart + padding + rightPart

	return s.styles.Footer.Width(s.width).Render(content)
}

func (s *StatusBar) formatItem(label, value string, color lipgloss.Color) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(s.styles.Theme.Muted).
		Bold(false)
	valueStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	return labelStyle.Render(label+": ") + valueStyle.Render(value)
}

func (s *StatusBar) getEnvColor() lipgloss.Color {
	switch s.environment {
	case "production", "prod":
		return s.styles.Theme.Error
	case "staging", "uat":
		return s.styles.Theme.Warning
	case "development", "dev":
		return s.styles.Theme.Success
	default:
		return s.styles.Theme.Info
	}
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (s *StatusBar) SetCluster(cluster string) {
	s.cluster = cluster
}

func (s *StatusBar) SetNamespace(ns string) {
	s.namespace = ns
}

func (s *StatusBar) SetEnvironment(env string) {
	s.environment = env
}

func (s *StatusBar) SetCost(cost float64) {
	s.costUSD = cost
}

func (s *StatusBar) AddCost(cost float64) {
	s.costUSD += cost
}

func (s *StatusBar) SetTokensUsed(tokens int) {
	s.tokensUsed = tokens
}

func (s *StatusBar) AddTokens(tokens int) {
	s.tokensUsed += tokens
}

func (s *StatusBar) SetTokensLimit(limit int) {
	s.tokensLimit = limit
}

func (s *StatusBar) SetSessionID(id string) {
	s.sessionID = id
}

func (s *StatusBar) SetMode(mode string) {
	s.mode = mode
}

func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

func (s *StatusBar) Reset() {
	s.costUSD = 0
	s.tokensUsed = 0
	s.uptime = 0
}
