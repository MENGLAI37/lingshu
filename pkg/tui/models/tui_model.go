package models

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/components"
	"github.com/lingshu/lingshu/pkg/tui/styles"
	"github.com/lingshu/lingshu/pkg/tui/theme"
)

type Page string

const (
	PageChat    Page = "chat"
	PageHelp    Page = "help"
	PageHistory Page = "history"
)

type TUIModel struct {
	theme        *theme.Theme
	styles       *styles.Styles
	currentPage  Page

	chatView       *components.ChatView
	input          *components.MultiLineInput
	statusBar      *components.StatusBar
	commandPreview *components.CommandPreview
	highlighted    *components.HighlightedRenderer

	width       int
	height      int
	showHelp    bool

	cluster     string
	namespace   string
	environment string
	sessionID   string //nolint:unused

	aiThinking bool
	streaming  bool

	msgChan chan tea.Msg
	program *tea.Program
}

type AIResponseMsg struct {
	Content string
	Done    bool
	Error   error
}

type ToolCallRequestMsg struct {
	Tool      string
	Command   string
	RiskLevel components.RiskLevel
	Desc      string
	Impact    string
	Preflight []components.PreflightCheck
}

type ToolResultMsg struct {
	Content string
	Error   error
}

type ShowHighlightMsg struct {
	Content     string
	ContentType components.ContentType
	Title       string
}

func NewTUIModel() *TUIModel {
	t := theme.GetTheme(theme.ThemeDark)
	s := styles.NewStyles(t)

	m := &TUIModel{
		theme:          t,
		styles:         s,
		currentPage:    PageChat,
		chatView:       components.NewChatView(s),
		input:          components.NewMultiLineInput(s),
		statusBar:      components.NewStatusBar(s),
		commandPreview: components.NewCommandPreview(s),
		highlighted:    components.NewHighlightedRenderer(s),
		cluster:        "kind-lingshu-dev",
		namespace:      "default",
		environment:    "development",
		msgChan:        make(chan tea.Msg, 100),
	}

	return m
}

func (m *TUIModel) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *TUIModel) SendMessage(msg tea.Msg) {
	if m.program != nil {
		m.program.Send(msg)
	}
}

func (m *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.input.Focus(),
		m.statusBar.Init(),
		tea.EnterAltScreen,
		tea.SetWindowTitle("lingshu - AI-native SRE Agent"),
	)
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			return m, tea.Quit
		case msg.String() == "q":
			if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp {
				return m, tea.Quit
			}
		case msg.String() == "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			if m.highlighted.Visible() {
				m.highlighted.Hide()
				return m, nil
			}
		case msg.String() == "?":
			if !m.commandPreview.Visible() && !m.highlighted.Visible() {
				m.showHelp = !m.showHelp
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 1
		inputHeight := 5
		bodyHeight := m.height - headerHeight - footerHeight - inputHeight

		m.chatView.SetWidth(m.width - 4)
		m.chatView.SetHeight(bodyHeight - 2)

		m.input.SetWidth(m.width)
		m.statusBar.SetWidth(m.width)

		m.highlighted.SetWidth(m.width - 10)
		m.highlighted.SetHeight(m.height - 10)

		m.chatView, cmd = m.chatView.Update(msg)
		cmds = append(cmds, cmd)

		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		m.statusBar, cmd = m.statusBar.Update(msg)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)

	case components.InputSubmittedMsg:
		m.handleUserInput(msg.Value)
		return m, nil

	case components.ConfirmApprovedMsg:
		m.handleConfirmApproved(msg.Command)
		return m, nil

	case components.ConfirmCancelledMsg:
		m.handleConfirmCancelled()
		return m, nil

	case AIResponseMsg:
		m.handleAIResponse(msg)
		return m, nil

	case ToolCallRequestMsg:
		m.handleToolCallRequest(msg)
		return m, nil

	case ToolResultMsg:
		m.handleToolResult(msg)
		return m, nil

	case ShowHighlightMsg:
		m.highlighted.SetContent(msg.Content, msg.ContentType, msg.Title)
		m.highlighted.Show()
		return m, nil
	}

	if m.commandPreview.Visible() {
		m.commandPreview, cmd = m.commandPreview.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	if m.highlighted.Visible() {
		m.highlighted, cmd = m.highlighted.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	m.chatView, cmd = m.chatView.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.statusBar, cmd = m.statusBar.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *TUIModel) View() string {
	header := m.renderHeader()
	body := m.renderBody()
	footer := m.statusBar.View()

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		footer,
	)

	if m.showHelp {
		return m.overlayHelp(mainContent)
	}

	if m.commandPreview.Visible() {
		return m.overlayCenter(mainContent, m.commandPreview.View())
	}

	if m.highlighted.Visible() {
		return m.overlayCenter(mainContent, m.highlighted.View())
	}

	return mainContent
}

func (m *TUIModel) renderHeader() string {
	titleStyle := m.styles.Header
	subtitleStyle := lipgloss.NewStyle().Foreground(m.theme.Muted)

	title := titleStyle.Render("⚡ lingshu")
	subtitle := subtitleStyle.Render("AI-native SRE Agent")

	rightInfo := fmt.Sprintf("%s @ %s [%s]",
		m.namespace,
		m.cluster,
		m.environment,
	)
	rightStyle := lipgloss.NewStyle().
		Foreground(m.theme.Secondary).
		Align(lipgloss.Right)

	rightWidth := lipgloss.Width(rightInfo)
	leftWidth := lipgloss.Width(title + " " + subtitle)
	paddingWidth := m.width - leftWidth - rightWidth - 4
	if paddingWidth < 0 {
		paddingWidth = 0
	}
	padding := strings.Repeat(" ", paddingWidth)

	headerContent := lipgloss.JoinHorizontal(lipgloss.Top,
		title+" ",
		subtitle,
		padding,
		rightStyle.Width(rightWidth).Render(rightInfo),
	)

	return lipgloss.NewStyle().
		Background(m.theme.Selection).
		Padding(0, 2).
		Width(m.width).
		Render(headerContent)
}

func (m *TUIModel) renderBody() string {
	chatArea := m.chatView.View()
	inputArea := m.input.View()

	separator := lipgloss.NewStyle().
		Foreground(m.theme.Border).
		Width(m.width).
		Render(strings.Repeat("─", m.width))

	bodyHeight := m.height - 8
	if bodyHeight < 5 {
		bodyHeight = 5
	}

	body := lipgloss.NewStyle().
		Padding(1, 2).
		Height(bodyHeight).
		Render(chatArea)

	inputSection := lipgloss.NewStyle().
		Padding(1, 2).
		Render(inputArea)

	return lipgloss.JoinVertical(lipgloss.Left,
		body,
		separator,
		inputSection,
	)
}

func (m *TUIModel) overlayHelp(content string) string {
	helpContent := `
╔══════════════════════════════════════════════════════════════╗
║                       快捷键帮助                             ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  基础操作:                                                   ║
║    Enter        - 发送消息                                   ║
║    Shift+Enter  - 换行                                       ║
║    ↑/↓          - 浏览历史记录                               ║
║                                                              ║
║  导航:                                                       ║
║    ?            - 显示/隐藏帮助                              ║
║    Esc          - 关闭弹窗/取消操作                          ║
║    Ctrl+C / q   - 退出程序                                   ║
║                                                              ║
║  命令预览:                                                   ║
║    Y            - 确认执行                                   ║
║    N / Esc      - 取消执行                                   ║
║    Tab / ←/→    - 切换按钮                                   ║
║                                                              ║
║  代码查看器:                                                 ║
║    ↑/↓          - 上下移动                                   ║
║    Space/Enter  - 折叠/展开                                  ║
║    PgUp/PgDn    - 翻页                                       ║
║    Home/End     - 跳转到开头/结尾                            ║
║    Esc          - 关闭                                       ║
║                                                              ║
║  聊天区域:                                                   ║
║    PgUp/PgDn    - 滚动聊天                                   ║
║    Home/End     - 跳转到开头/结尾                            ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`
	return m.overlayCenter(content, m.styles.BorderActive.Render(helpContent))
}

func (m *TUIModel) overlayCenter(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	baseHeight := len(baseLines)
	overlayHeight := len(overlayLines)
	overlayWidth := lipgloss.Width(overlayLines[0])

	startRow := (baseHeight - overlayHeight) / 2
	startCol := (m.width - overlayWidth) / 2

	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, line := range overlayLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}

		baseLine := baseLines[row]

		var result string
		if startCol > 0 {
			runes := []rune(baseLine)
			if len(runes) >= startCol {
				result = string(runes[:startCol])
			} else {
				result = baseLine + strings.Repeat(" ", startCol-len(runes))
			}
		}

		result += line

		endCol := startCol + overlayWidth
		runes := []rune(baseLine)
		if len(runes) > endCol {
			result += string(runes[endCol:])
		}

		baseLines[row] = result
	}

	return strings.Join(baseLines, "\n")
}

func (m *TUIModel) handleUserInput(input string) {
	m.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleUser,
		Content:   input,
		Timestamp: time.Now(),
	})

	m.aiThinking = true
	m.statusBar.AddTokens(len(input) / 4)

	go m.simulateAIResponse(input)
}

func (m *TUIModel) simulateAIResponse(userInput string) {
	responses := []string{
		"我来帮你分析这个问题。\n\n",
		"首先，让我检查一下相关的资源状态...\n\n",
	}

	for _, resp := range responses {
		time.Sleep(300 * time.Millisecond)
		m.SendMessage(AIResponseMsg{Content: resp, Done: false})
	}

	if strings.Contains(strings.ToLower(userInput), "scale") ||
		strings.Contains(strings.ToLower(userInput), "扩容") ||
		strings.Contains(strings.ToLower(userInput), "重启") ||
		strings.Contains(strings.ToLower(userInput), "delete") {
		m.SendMessage(ToolCallRequestMsg{
			Tool:      "kubectl",
			Command:   "kubectl scale deployment/nginx --replicas=5 -n default",
			RiskLevel: components.RiskL2,
			Desc:      "将 nginx deployment 的副本数调整为 5",
			Impact:    "直接受影响: Deployment/nginx, Service/nginx\n间接受影响: 下游服务 2 个 (依赖 nginx:80)",
			Preflight: []components.PreflightCheck{
				{Name: "集群连接", Passed: true, Detail: "正常"},
				{Name: "RBAC 权限", Passed: true, Detail: "充足"},
				{Name: "ResourceQuota", Passed: true, Detail: "CPU 剩余 45%, 内存剩余 62%"},
			},
		})
		return
	}

	finalResponse := "根据分析，这是一个示例响应。在实际使用中，我会调用 K8s 工具进行诊断。\n\n你可以尝试输入：\n- 排查 nginx Pod 重启原因\n- 查看集群健康状态\n- 扩容 deployment"

	m.SendMessage(AIResponseMsg{Content: finalResponse, Done: true})
}

func (m *TUIModel) handleAIResponse(msg AIResponseMsg) {
	if msg.Error != nil {
		m.chatView.AddMessage(components.ChatMessage{
			Role:      components.RoleSystem,
			Content:   fmt.Sprintf("错误: %v", msg.Error),
			Timestamp: time.Now(),
		})
		m.aiThinking = false
		m.streaming = false
		return
	}

	if msg.Done {
		m.chatView.FinishStreaming()
		m.aiThinking = false
		m.streaming = false
		m.statusBar.AddTokens(len(msg.Content) / 4)
	} else {
		m.chatView.AppendToLastMessage(msg.Content)
		m.streaming = true
	}
}

func (m *TUIModel) handleToolCallRequest(msg ToolCallRequestMsg) {
	m.commandPreview.Show(
		msg.Command,
		msg.RiskLevel,
		msg.Desc,
		msg.Impact,
		msg.Preflight,
	)
}

func (m *TUIModel) handleConfirmApproved(command string) {
	m.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleSystem,
		Content:   fmt.Sprintf("✓ 已确认执行: %s", command),
		Timestamp: time.Now(),
	})

	m.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleTool,
		Content:   "命令执行中...",
		Timestamp: time.Now(),
	})

	go func() {
		time.Sleep(1 * time.Second)
		m.SendMessage(ToolResultMsg{
			Content: "命令执行成功！Deployment/nginx 已扩容到 5 个副本。",
		})
	}()
}

func (m *TUIModel) handleToolResult(msg ToolResultMsg) {
	m.chatView.UpdateLastToolMessage(msg.Content)
	m.aiThinking = false
	m.streaming = false
}

func (m *TUIModel) handleConfirmCancelled() {
	m.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleSystem,
		Content:   "操作已取消。",
		Timestamp: time.Now(),
	})
	m.aiThinking = false
	m.streaming = false
}

func (m *TUIModel) SetCluster(cluster string) {
	m.cluster = cluster
	m.statusBar.SetCluster(cluster)
}

func (m *TUIModel) SetNamespace(ns string) {
	m.namespace = ns
	m.statusBar.SetNamespace(ns)
}

func (m *TUIModel) SetEnvironment(env string) {
	m.environment = env
	m.statusBar.SetEnvironment(env)
}

func (m *TUIModel) SetTheme(themeName theme.ThemeName) {
	m.theme = theme.GetTheme(themeName)
	m.styles = styles.NewStyles(m.theme)
	m.chatView = components.NewChatView(m.styles)
	m.input = components.NewMultiLineInput(m.styles)
	m.statusBar = components.NewStatusBar(m.styles)
	m.commandPreview = components.NewCommandPreview(m.styles)
	m.highlighted = components.NewHighlightedRenderer(m.styles)
}
