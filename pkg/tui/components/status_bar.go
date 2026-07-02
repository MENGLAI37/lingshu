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
	llmProvider   string
	aiStatus      AIStatus
}

// AIStatus represents the current AI agent status.
type AIStatus string

const (
	AIStatusIdle      AIStatus = "idle"
	AIStatusThinking  AIStatus = "thinking"
	AIStatusExecuting AIStatus = "executing"
	AIStatusError     AIStatus = "error"
)

func (s AIStatus) Label() string {
	switch s {
	case AIStatusThinking:
		return "🤔 思考中"
	case AIStatusExecuting:
		return "🔧 执行工具中"
	case AIStatusError:
		return "❌ 错误"
	default:
		return "✅ 就绪"
	}
}

func (s AIStatus) Color() string {
	switch s {
	case AIStatusThinking:
		return "#f0c040"
	case AIStatusExecuting:
		return "#40a0f0"
	case AIStatusError:
		return "#f04040"
	default:
		return "#40f080"
	}
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
	aiStatusLabel := s.aiStatus.Label()
	aiStatusColor := lipgloss.Color(s.aiStatus.Color())
	aiStatusStyle := lipgloss.NewStyle().
		Foreground(aiStatusColor).
		Bold(true)
	aiStatusStr := aiStatusStyle.Render(aiStatusLabel)

	leftItems := []string{
		aiStatusStr,
		s.formatItem("集群", s.cluster, s.styles.Theme.Primary),
		s.formatItem("命名空间", s.namespace, s.styles.Theme.Info),
		s.formatItem("环境", s.environment, s.getEnvColor()),
	}

	if s.llmProvider != "" {
		leftItems = append(leftItems, s.formatItem("LLM", s.llmProvider, s.styles.Theme.Secondary))
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

func (s *StatusBar) SetLLMProvider(provider string) {
	s.llmProvider = provider
}

func (s *StatusBar) SetAIStatus(status AIStatus) {
	s.aiStatus = status
}

func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

func (s *StatusBar) Reset() {
	s.costUSD = 0
	s.tokensUsed = 0
	s.uptime = 0
}
