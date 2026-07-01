package models

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/agent"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/k8s"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/security"
	"github.com/lingshu/lingshu/pkg/tools"
	"github.com/lingshu/lingshu/pkg/tools/l0"
	"github.com/lingshu/lingshu/pkg/tools/l1"
	"github.com/lingshu/lingshu/pkg/tools/l2"
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
	configPanel    *components.ConfigPanel

	width       int
	height      int
	showHelp    bool

	cluster     string
	namespace   string
	environment string

	aiThinking bool
	streaming  bool

	msgChan chan tea.Msg
	program *tea.Program

	// Agent Loop integration
	agentLoop *agent.DefaultAgentLoop
	k8sClient *k8s.ClientManager
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
		configPanel:    components.NewConfigPanel(s),
		cluster:        "kind-lingshu-dev",
		namespace:      "default",
		environment:    "development",
		msgChan:        make(chan tea.Msg, 100),
	}

	m.addWelcomeMessage()
	m.initAgentLoop()

	return m
}

func (m *TUIModel) addWelcomeMessage() {
	welcome := "欢迎使用 LingShu AI-native SRE Agent！\n\n" +
		"我可以帮你：\n" +
		"  • 排查 Kubernetes 集群问题\n" +
		"  • 查看 Pod 状态、日志、事件\n" +
		"  • 执行安全的运维操作（扩容、重启等）\n\n" +
		"快捷键提示：\n" +
		"  按 F1 或 ? 查看完整快捷键列表\n" +
		"  按 c 打开配置面板（配置 LLM Provider）\n" +
		"  按 i 或 Enter 开始输入问题\n\n" +
		"试试输入：排查 nginx Pod 重启原因"

	m.chatView.AddMessage(components.ChatMessage{
		Role:      components.RoleSystem,
		Content:   welcome,
		Timestamp: time.Now(),
	})
}

// initAgentLoop initializes the Agent Loop with K8s tools and LLM
func (m *TUIModel) initAgentLoop() {
	// Try to load configuration
	cfg := config.Get()

	// Initialize K8s client from kubeconfig
	var k8sClient *k8s.ClientManager
	kubeconfigPath := ""
	if cfg != nil && cfg.Server.Host != "" {
		// Try to use configured kubeconfig if available
	}
	// Try to get kubeconfig from environment or default location
	if os.Getenv("KUBECONFIG") != "" || kubeconfigPath != "" {
		var err error
		k8sClient, err = k8s.NewClientManager(kubeconfigPath)
		if err == nil {
			m.k8sClient = k8sClient
		}
	}

	// Create LLM router with default or configured provider
	llmRouter := m.createLLMRouter(cfg)

	if llmRouter == nil {
		return
	}

	// Initialize tool registry with K8s tools
	toolRegistry := agent.NewDefaultToolRegistry()
	if m.k8sClient != nil {
		// Get the default context clientset
		clientset, err := m.k8sClient.GetClientSet(context.Background(), "")
		if err != nil {
			// silently ignore k8s errors in TUI mode
		} else {
			// Register L0 tools (read-only)
			_ = toolRegistry.RegisterTool(l0.NewGetTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewDescribeTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewLogsTool(clientset))
			_ = toolRegistry.RegisterTool(l0.NewEventsTool(clientset))
			// Register L1 tools (safe write)
			_ = toolRegistry.RegisterTool(l1.NewTopTool(clientset, nil))
			_ = toolRegistry.RegisterTool(l1.NewStatusTool(clientset))
			// Register L2 tools (moderate risk)
			_ = toolRegistry.RegisterTool(l2.NewScaleTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRestartTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewRolloutTool(clientset))
			_ = toolRegistry.RegisterTool(l2.NewPatchTool(clientset))
		}
	}

	// Initialize security gateway
	securityGateway := m.createSecurityGateway()

	// Initialize Agent Loop
	agentLoopConfig := agent.DefaultLoopConfig()
	m.agentLoop = agent.NewDefaultAgentLoop(
		agentLoopConfig,
		llmRouter,
		toolRegistry,
		securityGateway,
	)
}

// createLLMRouter creates an LLM router from config or environment
func (m *TUIModel) createLLMRouter(cfg *config.Config) *llm.Router {
	_ = config.LoadLLMConfig()

	providerConfig := config.GetCurrentProviderConfig()
	if providerConfig == nil {
		m.statusBar.SetLLMProvider("未配置")
		return nil
	}

	m.statusBar.SetLLMProvider(providerConfig.Name)
	return llm.NewRouter([]llm.ProviderConfig{*providerConfig})
}

// createSecurityGateway creates a security gateway for the agent loop
func (m *TUIModel) createSecurityGateway() agent.SecurityGateway {
	return &agentSecurityGatewayAdapter{
		gateway: security.NewDefaultSecurityGateway(security.DefaultGatewayConfig()),
	}
}

// agentSecurityGatewayAdapter wraps security.DefaultSecurityGateway to implement agent.SecurityGateway
type agentSecurityGatewayAdapter struct {
	gateway *security.DefaultSecurityGateway
}

func (a *agentSecurityGatewayAdapter) EvaluateRisk(ctx context.Context, toolName string, args map[string]any) (agent.RiskEvaluation, error) {
	eval, err := a.gateway.EvaluateRisk(ctx, toolName, args)
	if err != nil {
		return agent.RiskEvaluation{}, err
	}

	// Convert security.RiskEvaluation to agent.RiskEvaluation
	return agent.RiskEvaluation{
		RiskLevel:         tools.ToolRiskLevel(eval.RiskLevel),
		Score:             eval.Score,
		Reason:            eval.Reason,
		AffectedResources: eval.AffectedResources,
		EnvironmentWeight: eval.EnvironmentWeight,
	}, nil
}

func (a *agentSecurityGatewayAdapter) IsAllowed(ctx context.Context, evaluation agent.RiskEvaluation) (bool, string) {
	// Convert agent.RiskEvaluation back to security.RiskEvaluation
	secEval := security.RiskEvaluation{
		RiskLevel:         security.RiskLevel(evaluation.RiskLevel),
		Score:             evaluation.Score,
		Reason:            evaluation.Reason,
		AffectedResources: evaluation.AffectedResources,
		EnvironmentWeight: evaluation.EnvironmentWeight,
	}
	return a.gateway.IsAllowed(ctx, secEval)
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
		// ============================================================
		// L0: System-level shortcuts (highest priority)
		// These work regardless of input focus state
		// ============================================================

		// Ctrl+C - Force quit (always available)
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// ============================================================
		// L1: Normal mode (input NOT focused) - Global shortcuts
		// ============================================================
		if !m.input.Focused() {
			// Ctrl+Q - Graceful quit (not available in Insert mode)
			if msg.String() == "ctrl+q" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					return m, tea.Quit
				}
			}

			// F1 / ? - Toggle help overlay
			if msg.String() == "f1" || msg.String() == "?" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.configPanel.Visible() {
					m.showHelp = !m.showHelp
					return m, nil
				}
			}

			// Esc - Close topmost popup layer by layer
			// Help overlay is handled here; popups (commandPreview, highlighted, configPanel)
			// handle Esc on their own via their Update() method. We only intercept Esc here
			// for the help overlay, and let popups receive the Esc key directly.
			if msg.String() == "esc" {
				if m.showHelp {
					m.showHelp = false
					return m, nil
				}
				// If any popup is visible, fall through and let the popup handle it.
				// Do NOT return here - the popup needs to receive the Esc key.
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.configPanel.Visible() {
					// No popups open, Esc does nothing in Normal mode
					return m, nil
				}
			}

			// i / Enter - Focus input, enter Insert mode
			if msg.String() == "i" || msg.String() == "enter" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.input.Focus()
					return m, tea.Batch(cmd)
				}
			}

			// j / ↓ - Scroll chat down
			if msg.String() == "j" || msg.String() == "down" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollDown(1)
					return m, nil
				}
			}

			// k / ↑ - Scroll chat up
			if msg.String() == "k" || msg.String() == "up" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollUp(1)
					return m, nil
				}
			}

			// Ctrl+D / PgDn - Scroll down half page
			if msg.String() == "ctrl+d" || msg.Type == tea.KeyPgDown {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollDown(m.chatView.Height() / 2)
					return m, nil
				}
			}

			// Ctrl+U / PgUp - Scroll up half page
			if msg.String() == "ctrl+u" || msg.Type == tea.KeyPgUp {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollUp(m.chatView.Height() / 2)
					return m, nil
				}
			}

			// g / Home - Go to top
			if msg.String() == "g" || msg.Type == tea.KeyHome {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollToTop()
					return m, nil
				}
			}

			// G / End - Go to bottom
			if msg.String() == "G" || msg.Type == tea.KeyEnd {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.chatView.ScrollToBottom()
					return m, nil
				}
			}

			// / - Open search
			if msg.String() == "/" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					// TODO: Implement search feature
					return m, nil
				}
			}

			// n - Next search result
			if msg.String() == "n" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					// TODO: Implement search navigation
					return m, nil
				}
			}

			// N - Previous search result
			if msg.String() == "N" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					// TODO: Implement search navigation
					return m, nil
				}
			}

			// c - Open config panel
			if msg.String() == "c" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.configPanel.Show()
					return m, nil
				}
			}

			// t - Cycle theme
			if msg.String() == "t" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					currentTheme := m.theme.Name
					switch currentTheme {
					case theme.ThemeDark:
						m.SetTheme(theme.ThemeLight)
					case theme.ThemeLight:
						m.SetTheme(theme.ThemeHighContrast)
					default:
						m.SetTheme(theme.ThemeDark)
					}
					return m, nil
				}
			}

			// r - Reload agent loop
			if msg.String() == "r" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					m.reinitAgentLoop()
					return m, nil
				}
			}

			// m - Toggle input mode (single/multi line)
			if msg.String() == "m" {
				if !m.commandPreview.Visible() && !m.highlighted.Visible() && !m.showHelp && !m.configPanel.Visible() {
					// TODO: Implement input mode toggle
					return m, nil
				}
			}
		}

		// ============================================================
		// L2: Insert mode (input focused) - Global shortcuts only
		// Text editing shortcuts (Ctrl+A/E/K/U/W, etc.) are handled
		// by the textarea component itself
		// ============================================================
		if m.input.Focused() {
			// Esc - Exit Insert mode, return to Normal mode
			if msg.String() == "esc" {
				m.input.Blur()
				return m, nil
			}

			// Ctrl+L - Clear input
			if msg.String() == "ctrl+l" {
				m.input.SetValue("")
				return m, nil
			}

			// Let textarea handle all other keys (↑/↓ for history,
			// Ctrl+A/E/K/U/W for editing, etc.)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// 精确布局高度分配
		const (
			headerHeight    = 1
			footerHeight    = 1
			separatorHeight = 1
			bodyPadding     = 2
		)

		// 获取输入区域实际高度（prompt 1行 + textarea 高度）
		inputAreaStr := m.input.View()
		inputLines := strings.Count(inputAreaStr, "\n") + 1
		if inputLines < 4 {
			inputLines = 4
		}

		chatHeight := m.height - headerHeight - footerHeight - separatorHeight - inputLines - bodyPadding
		if chatHeight < 3 {
			chatHeight = 3
		}

		m.chatView.SetWidth(m.width - 4)
		m.chatView.SetHeight(chatHeight)

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

	case components.ConfigSavedMsg:
		m.configPanel.Hide()
		m.reinitAgentLoop()
		return m, nil

	case components.ConfigCancelledMsg:
		m.configPanel.Hide()
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

	if m.configPanel.Visible() {
		m.configPanel, cmd = m.configPanel.Update(msg)
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

	// 确保恰好 m.height 行，防止屏幕残留
	mainContent = ensureHeight(mainContent, m.height)

	if m.showHelp {
		return m.overlayHelp(mainContent)
	}

	if m.commandPreview.Visible() {
		return m.overlayCenter(mainContent, m.commandPreview.View())
	}

	if m.highlighted.Visible() {
		return m.overlayCenter(mainContent, m.highlighted.View())
	}

	if m.configPanel.Visible() {
		return m.overlayCenter(mainContent, m.configPanel.View())
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

	headerHeight := 2
	footerHeight := 1
	inputLines := lipgloss.Height(inputArea)
	separatorHeight := 1
	chatAreaHeight := m.height - headerHeight - footerHeight - inputLines - separatorHeight - 2
	if chatAreaHeight < 3 {
		chatAreaHeight = 3
	}

	body := lipgloss.NewStyle().
		Padding(0, 2).
		Height(chatAreaHeight).
		Render(chatArea)

	separator := lipgloss.NewStyle().
		Foreground(m.theme.Border).
		Width(m.width).
		Render(strings.Repeat("─", m.width))

	inputSection := lipgloss.NewStyle().
		Padding(0, 2).
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
║                    LingShu TUI 快捷键速查                    ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  全局 (任何模式)                                              ║
║    Ctrl+C       强制退出程序                                 ║
║    Ctrl+Q       优雅退出程序                                 ║
║    F1 / ?       显示/隐藏帮助                                ║
║    Esc          关闭弹窗/返回上层                            ║
║                                                              ║
║  Normal 模式 (输入框未聚焦)                                   ║
║    i / Enter    开始输入 (进入 Insert 模式)                   ║
║    j / ↓        向下滚动一行                                 ║
║    k / ↑        向上滚动一行                                 ║
║    Ctrl+D / PgDn  向下翻半页                                 ║
║    Ctrl+U / PgUp  向上翻半页                                  ║
║    g / Home      跳转到顶部                                   ║
║    G / End       跳转到底部                                   ║
║    /            打开搜索                                     ║
║    n / N        搜索结果上/下                                ║
║    c            打开配置面板                                 ║
║    t            切换主题                                     ║
║    r            重新初始化 Agent                             ║
║                                                              ║
║  Insert 模式 (输入框聚焦)                                     ║
║    Enter        发送消息                                     ║
║    Shift+Enter  换行 (不发送)                                ║
║    Esc          退出输入模式                                 ║
║    ↑/↓         上下翻历史记录                               ║
║    Ctrl+P       上一条历史                                   ║
║    Ctrl+N       下一条历史                                   ║
║    Ctrl+A / E   光标到行首/行尾                             ║
║    Ctrl+K / U   删除到行尾/删除整行                         ║
║    Ctrl+L       清空输入框                                   ║
║                                                              ║
║  命令预览弹窗                                                ║
║    Y / Enter    确认执行                                     ║
║    N / Esc      取消执行                                     ║
║    Tab / ←/→   切换按钮焦点                                 ║
║    j/k / ↑/↓   滚动内容                                     ║
║    v            查看完整影响分析                             ║
║                                                              ║
║  代码查看器                                                  ║
║    j/k / ↑/↓   上下移动                                     ║
║    h/l / ←/→   左右滚动 (长行)                              ║
║    Space / Ctrl+F / PgDn  向下翻页                           ║
║    Ctrl+B / PgUp  向上翻页                                   ║
║    g / Home      跳转到顶部                                   ║
║    G / End       跳转到底部                                   ║
║    Enter / o     折叠/展开当前区域                           ║
║    z            折叠/展开所有区域                            ║
║    y            复制当前行                                   ║
║    q            关闭查看器                                   ║
║                                                              ║
║  配置面板                                                    ║
║    j/k / ↑/↓   选择 Provider                                ║
║    Enter / e    编辑选中项                                   ║
║    a / n        添加新 Provider                              ║
║    d / x        删除选中项                                   ║
║    s / Ctrl+S   保存配置                                     ║
║    r            重新加载配置                                 ║
║    q / Esc      关闭面板                                     ║
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

	// Use real Agent Loop if available
	if m.agentLoop != nil {
		go m.runAgentLoop(input)
	} else {
		// Fallback to demo mode with mock responses
		go m.runDemoMode(input)
	}
}

// runAgentLoop executes the real Agent Loop for user input
func (m *TUIModel) runAgentLoop(userInput string) {
	ctx := context.Background()

	// Create event handler to process agent events
	eventHandler := func(event agent.LoopEvent) {
		switch event.Type {
		case "thinking":
			if thought, ok := event.Data.(string); ok {
				m.SendMessage(AIResponseMsg{Content: thought + "\n\n", Done: false})
			}
		case "state_change":
			switch event.State {
			case agent.StateExecuting:
				m.SendMessage(AIResponseMsg{Content: "正在执行工具...\n\n", Done: false})
			case agent.StateObserving:
				m.SendMessage(AIResponseMsg{Content: "分析结果...\n\n", Done: false})
			}
		case "tool_result":
			if result, ok := event.Data.(agent.ToolExecutionResult); ok {
				formattedResult := m.formatToolExecutionResult(result)
				m.SendMessage(ToolResultMsg{Content: formattedResult})
			}
		case "error":
			if err, ok := event.Data.(error); ok {
				m.SendMessage(AIResponseMsg{Content: fmt.Sprintf("错误: %v\n\n", err), Done: true})
			}
		}
	}

	// Execute the agent loop
	result, err := m.agentLoop.Execute(ctx, userInput, eventHandler)

	if err != nil {
		m.SendMessage(AIResponseMsg{
			Content: fmt.Sprintf("Agent 执行失败: %v\n", err),
			Done:    true,
			Error:   err,
		})
		m.aiThinking = false
		return
	}

	// Send final response
	if result.FinalResponse != "" {
		m.SendMessage(AIResponseMsg{
			Content: "\n" + result.FinalResponse + "\n",
			Done:    true,
		})
	} else if len(result.ToolResults) > 0 {
		// Generate summary from tool results
		summary := m.generateDiagnosisSummary(result.ToolResults)
		m.SendMessage(AIResponseMsg{
			Content: "\n" + summary + "\n",
			Done:    true,
		})
	}

	m.aiThinking = false
}

// formatToolExecutionResult formats a tool execution result for display
func (m *TUIModel) formatToolExecutionResult(result agent.ToolExecutionResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 工具: %s\n", result.ToolName))

	if result.Error != nil {
		sb.WriteString(fmt.Sprintf("❌ 错误: %v\n", result.Error))
		return sb.String()
	}

	sb.WriteString("✅ 执行成功\n")

	if result.Result != nil {
		if result.Result.Data != nil {
			dataJSON, err := json.MarshalIndent(result.Result.Data, "", "  ")
			if err == nil {
				// Truncate long outputs
				dataStr := string(dataJSON)
				if len(dataStr) > 2000 {
					dataStr = dataStr[:2000] + "\n... (输出已截断)"
				}
				sb.WriteString(fmt.Sprintf("结果:\n%s\n", dataStr))
			}
		}
		if result.Result.Message != "" {
			sb.WriteString(fmt.Sprintf("消息: %s\n", result.Result.Message))
		}
	}

	sb.WriteString(fmt.Sprintf("耗时: %v\n", result.Duration))
	return sb.String()
}

// generateDiagnosisSummary generates a diagnosis summary from tool results
func (m *TUIModel) generateDiagnosisSummary(results []agent.ToolExecutionResult) string {
	var sb strings.Builder
	sb.WriteString("📊 诊断摘要\n\n")

	// Analyze results for pod restart diagnosis
	var podEvents []string
	var podLogs []string
	var podStatus []string

	for _, result := range results {
		if result.ToolName == "k8s_events" && result.Result != nil {
			podEvents = append(podEvents, result.Result.Message)
		}
		if result.ToolName == "k8s_logs" && result.Result != nil {
			podLogs = append(podLogs, result.Result.Message)
		}
		if result.ToolName == "k8s_get" && result.Result != nil {
			podStatus = append(podStatus, result.Result.Message)
		}
	}

	if len(podEvents) > 0 || len(podLogs) > 0 || len(podStatus) > 0 {
		sb.WriteString("🔍 检查结果:\n")

		if len(podStatus) > 0 {
			sb.WriteString("\nPod 状态:\n")
			for _, s := range podStatus {
				sb.WriteString(fmt.Sprintf("  %s\n", s))
			}
		}

		if len(podEvents) > 0 {
			sb.WriteString("\n相关事件:\n")
			for _, e := range podEvents {
				sb.WriteString(fmt.Sprintf("  %s\n", e))
			}
		}

		if len(podLogs) > 0 {
			sb.WriteString("\n日志摘要:\n")
			for _, l := range podLogs {
				if len(l) > 500 {
					l = l[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("  %s\n", l))
			}
		}
	}

	sb.WriteString("\n💡 建议:\n")
	sb.WriteString("  1. 检查上述事件和日志来确定重启原因\n")
	sb.WriteString("  2. 常见原因: OOMKilled、CrashLoopBackOff、健康检查失败\n")
	sb.WriteString("  3. 如需进一步诊断，请提供更多详细信息\n")

	return sb.String()
}

// runDemoMode provides demo responses when Agent Loop is not available
func (m *TUIModel) runDemoMode(userInput string) {
	responses := []string{
		"我来帮你分析这个问题。\n\n",
		"正在连接到 Kubernetes 集群...\n\n",
	}

	for _, resp := range responses {
		time.Sleep(300 * time.Millisecond)
		m.SendMessage(AIResponseMsg{Content: resp, Done: false})
	}

	// Check if user is asking about nginx pod restart
	lowerInput := strings.ToLower(userInput)
	if strings.Contains(lowerInput, "nginx") && strings.Contains(lowerInput, "重启") {
		m.SendMessage(AIResponseMsg{
			Content: "🔍 开始诊断 nginx Pod 重启问题...\n\n" +
				"正在执行诊断步骤:\n" +
				"1. 获取 Pod 状态...\n" +
				"2. 检查最近的事件...\n" +
				"3. 查看容器日志...\n\n",
			Done: false,
		})

		// Simulate tool execution
		time.Sleep(500 * time.Millisecond)
		m.SendMessage(ToolResultMsg{
			Content: "📋 工具: k8s_get\n" +
				"✅ 执行成功\n" +
				"结果:\n" +
				"NAME                    READY   STATUS            RESTARTS   AGE\n" +
				"nginx-6799fc88d8-x7s2k  1/1     Running           3          5m\n\n" +
				"耗时: 123ms\n",
		})

		time.Sleep(300 * time.Millisecond)
		m.SendMessage(ToolResultMsg{
			Content: "📋 工具: k8s_events\n" +
				"✅ 执行成功\n" +
				"结果:\n" +
				"LAST SEEN   TYPE      REASON             OBJECT         MESSAGE\n" +
				"2m          Warning   BackOff            Pod            Back-off restarting failed container\n" +
				"5m          Normal    Created            Pod            Created container\n" +
				"5m          Normal    Started            Pod            Started container\n\n" +
				"耗时: 89ms\n",
		})

		time.Sleep(300 * time.Millisecond)
		m.SendMessage(ToolResultMsg{
			Content: "📋 工具: k8s_logs\n" +
				"✅ 执行成功\n" +
				"结果:\n" +
				"[error] panic: connection refused\n" +
				"[error] failed to initialize database\n" +
				"[fatal] unable to start server\n\n" +
				"耗时: 156ms\n",
		})

		// Final diagnosis
		m.SendMessage(AIResponseMsg{
			Content: "\n📊 诊断结论\n\n" +
				"🔴 **Pod 状态异常**\n" +
				"- Pod: nginx-6799fc88d8-x7s2k\n" +
				"- 重启次数: 3 次 (BackOff)\n\n" +
				"🔍 **根因分析**\n" +
				"根据日志分析，Pod 重启原因是:\n" +
				"1. 容器启动时连接数据库失败 (connection refused)\n" +
				"2. 应用无法初始化导致崩溃\n" +
				"3. Kubernetes 尝试重新启动，形成 CrashLoopBackOff\n\n" +
				"💡 **建议操作**\n" +
				"1. 检查数据库连接配置是否正确\n" +
				"2. 确认数据库服务是否正常运行\n" +
				"3. 检查应用的数据库连接池配置\n\n" +
				"是否需要我帮你进一步检查数据库状态?",
			Done: true,
		})
		return
	}

	// Check for scale/restart commands
	if strings.Contains(lowerInput, "scale") ||
		strings.Contains(lowerInput, "扩容") ||
		strings.Contains(lowerInput, "restart") ||
		strings.Contains(lowerInput, "重启") {
		m.SendMessage(ToolCallRequestMsg{
			Tool:      "k8s_scale",
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

	// Default response
	finalResponse := "我已收到你的请求。当前 Agent Loop 正在初始化中。\n\n" +
		"在完整功能下，我会:\n" +
		"1. 分析你的问题\n" +
		"2. 调用 K8s 工具收集信息\n" +
		"3. 根据结果进行诊断\n" +
		"4. 提供修复建议\n\n" +
		"你可以尝试输入:\n" +
		"- 排查 nginx Pod 重启原因\n" +
		"- 查看集群健康状态\n" +
		"- 扩容 deployment"

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

	inputFocused := m.input.Focused()
	inputValue := m.input.Value()

	m.chatView = components.NewChatView(m.styles)
	m.input = components.NewMultiLineInput(m.styles)
	m.statusBar = components.NewStatusBar(m.styles)
	m.commandPreview = components.NewCommandPreview(m.styles)
	m.highlighted = components.NewHighlightedRenderer(m.styles)
	m.configPanel = components.NewConfigPanel(m.styles)

	if m.width > 0 && m.height > 0 {
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
	}

	m.statusBar.SetCluster(m.cluster)
	m.statusBar.SetNamespace(m.namespace)
	m.statusBar.SetEnvironment(m.environment)

	cfg := config.GetCurrentProviderConfig()
	if cfg != nil {
		m.statusBar.SetLLMProvider(cfg.Name)
	}

	if inputFocused {
		m.input.Focus()
	}
	if inputValue != "" {
		m.input.SetValue(inputValue)
	}
}

func (m *TUIModel) reinitAgentLoop() {
	m.agentLoop = nil
	m.initAgentLoop()
}

// ensureHeight 将内容截断或填充到恰好 height 行，防止终端屏幕残留
func ensureHeight(content string, height int) string {
	content = strings.TrimSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
