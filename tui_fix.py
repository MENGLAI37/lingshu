import re
with open('pkg/tui/models/tui_model.go', 'r', encoding='utf-8') as f:
    content = f.read()

# 1. Add imports
content = content.replace(
    'metricsv "k8s.io/metrics/pkg/client/clientset/versioned"\n)',
    'corev1 "k8s.io/api/core/v1"\n\tmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"\n\tmetricsv "k8s.io/metrics/pkg/client/clientset/versioned"\n)', 1)
content = content.replace(
    'tea "github.com/charmbracelet/bubbletea"\n\t"github.com/charmbracelet/lipgloss"',
    '"github.com/google/uuid"\n\ttea "github.com/charmbracelet/bubbletea"\n\t"github.com/charmbracelet/lipgloss"', 1)
content = content.replace(
    '"github.com/charmbracelet/lipgloss"\n\t"github.com/lingshu/lingshu/pkg/agent"',
    '"github.com/charmbracelet/lipgloss"\n\t"github.com/lingshu/lingshu/pkg/agent"\n\t"github.com/lingshu/lingshu/pkg/alertd"', 1)

# 2. Add InspectionEvent type
content = content.replace('type TUIModel struct {',
    'type InspectionEvent struct {\n\tType    components.EventType\n\tMessage string\n\tDetail  string\n}\n\ntype TUIModel struct {', 1)

# 3. Add struct fields
content = content.replace('\tconfirmationModal  *components.ConfirmationModal\n\n\twidth',
    '\tconfirmationModal  *components.ConfirmationModal\n\teventPanel         *components.EventPanel\n\n\twidth', 1)
content = content.replace('\tagentLoop *agent.DefaultAgentLoop\n\tk8sClient *k8s.ClientManager',
    '\tagentLoop   *agent.DefaultAgentLoop\n\tautoEngine  *agent.AutonomousEngine\n\tk8sClient   *k8s.ClientManager\n\ttoolRegistry agent.ToolRegistry', 1)
content = content.replace('\tpendingConfirmationChan chan bool\n}',
    '\tpendingConfirmationChan chan bool\n\n\tinspectCount int\n}', 1)

# 4. Add eventPanel to NewTUIModel
content = content.replace('\t\tconfirmationModal:    components.NewConfirmationModal(s),',
    '\t\tconfirmationModal:    components.NewConfirmationModal(s),\n\t\teventPanel:           components.NewEventPanel(s),', 1)

# 5. Fix K8s init
old = '// Initialize K8s client from kubeconfig'
idx = content.find(old)
if idx >= 0:
    end_idx = content.find('\t}\n\n\t// Create LLM router', idx)
    if end_idx >= 0:
        new_k8s = '// Initialize K8s client (auto-detects HOME/USERPROFILE)\n\tk8sClient, err := k8s.NewClientManager("")\n\tif err != nil {\n\t\tfmt.Println("Warning: Failed to initialize K8s client:", err)\n\t} else {\n\t\tm.k8sClient = k8sClient\n\t}'
        content = content[:idx] + new_k8s + content[end_idx:]
        print('K8s init: FIXED')
    else:
        print('K8s init end: NOT FOUND')

# 6. Add autonomous engine setup
idx = content.find('m.agentLoop.SetSessionManager(&tuiSessionAdapter{mgr: sessMgr})')
if idx >= 0:
    # Find the closing } before createLLMRouter
    rest = content[idx:]
    m = re.search(r'\n(\t}\n\n// createLLMRouter)', rest)
    if m:
        pos = idx + m.start()
        insert = '\n\n\t\t\t// Wire autonomous engine for background cluster inspection\n\t\t\tm.toolRegistry = toolRegistry\n\t\t\tm.autoEngine = agent.NewAutonomousEngine(m.agentLoop, securityGateway, nil)\n\t\t\tm.autoEngine.SetApprovalPolicy(agent.AutoApproveSafe)\n\t\t\tm.agentLoop.SetConfirmationHandler(m.autoEngine.Confirmer())'
        content = content[:pos] + insert + content[pos:]
        print('AutoEngine: ADDED')

# 7. Add InspectionEvent handler in Update()
old_up = '\tswitch msg := msg.(type) {\n\tcase tea.KeyMsg:'
new_up = '\tswitch msg := msg.(type) {\n\tcase InspectionEvent:\n\t\tm.eventPanel.AddEvent(components.EventEntry{\n\t\t\tTime:    time.Now(),\n\t\t\tType:    msg.Type,\n\t\t\tMessage: msg.Message,\n\t\t\tDetail:  msg.Detail,\n\t\t})\n\t\tm.inspectCount++\n\t\tm.statusBar.SetInspectInfo(fmt.Sprintf("#%d", m.inspectCount))\n\t\treturn m, nil\n\tcase tea.KeyMsg:'
content = content.replace(old_up, new_up, 1)
print('InspectionEvent handler: ADDED')

# 8. Modify renderBody
old_rb = 'func (m *TUIModel) renderBody() string {\n\tchatArea := m.chatView.View()'
new_rb = 'func (m *TUIModel) renderBody() string {\n\tif m.width <= 0 {\n\t\tm.width = 80\n\t}\n\teventHeight := m.height / 3\n\tif eventHeight < 6 {\n\t\teventHeight = 6\n\t}\n\tif eventHeight > 15 {\n\t\teventHeight = 15\n\t}\n\tm.eventPanel.SetSize(m.width-4, eventHeight)\n\teventArea := lipgloss.NewStyle().Padding(0, 2).Height(eventHeight).Render(m.eventPanel.View())\n\n\tsepLabel := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("─ 巡检事件")\n\tsepPadWidth := m.width - lipgloss.Width(sepLabel) - 2\n\tif sepPadWidth < 0 {\n\t\tsepPadWidth = 0\n\t}\n\tsepPad := strings.Repeat("─", sepPadWidth)\n\teventSep := lipgloss.NewStyle().Foreground(m.theme.Border).Render(sepLabel + sepPad)\n\n\tremainingHeight := m.height - eventHeight - 11\n\tif remainingHeight < 5 {\n\t\tremainingHeight = 5\n\t}\n\tm.chatView.SetHeight(remainingHeight)\n\tchatArea := m.chatView.View()'
content = content.replace(old_rb, new_rb, 1)
print('renderBody: UPDATED')

# Fix return in renderBody
old_ret = 'return lipgloss.JoinVertical(lipgloss.Left,\n\t\tchatSection,\n\t\tseparator,\n\t\tinputSection,\n\t)'
new_ret = 'return lipgloss.JoinVertical(lipgloss.Left,\n\t\teventArea,\n\t\teventSep,\n\t\tchatSection,\n\t\tlipgloss.NewStyle().Foreground(m.theme.Border).Width(m.width).Render(strings.Repeat("─", m.width)),\n\t\tinputSection,\n\t)'
if old_ret in content:
    content = content.replace(old_ret, new_ret, 1)
    print('renderBody return: UPDATED')

# 9. Modify Init()
old_init = 'func (m *TUIModel) Init() tea.Cmd {\n\treturn tea.Batch(\n\t\tm.input.Focus(),\n\t\tm.statusBar.Init(),\n\t\ttea.EnterAltScreen,\n\t\ttea.SetWindowTitle("lingshu - AI-native SRE Agent"),\n\t)\n}'
new_init = 'func (m *TUIModel) Init() tea.Cmd {\n\tm.statusBar.SetInspectInfo("启动中")\n\treturn tea.Batch(\n\t\tm.input.Focus(),\n\t\tm.statusBar.Init(),\n\t\ttea.EnterAltScreen,\n\t\ttea.SetWindowTitle("lingshu - AI-native SRE Agent"),\n\t\tm.startAutonomousInspection(),\n\t)\n}'
content = content.replace(old_init, new_init, 1)
print('Init: UPDATED')

# 10. Fix overlayCenter - max width
old_ov = 'overlayWidth := lipgloss.Width(overlayLines[0])'
new_ov = 'overlayWidth := 0\n\tfor _, l := range overlayLines {\n\t\tw := lipgloss.Width(l)\n\t\tif w > overlayWidth {\n\t\t\toverlayWidth = w\n\t\t}\n\t}'
content = content.replace(old_ov, new_ov, 1)
print('overlayCenter max-width: UPDATED')

# Fix overlayCenter - padding to fill width
old_ov2 = '\t\tresult += line\n\n\t\tendCol := startCol + overlayWidth\n\t\trunes := []rune(baseLine)\n\t\tif len(runes) > endCol {\n\t\t\tresult += string(runes[endCol:])\n\t\t}\n\n\t\tbaseLines[row] = result'
new_ov2 = '\t\tlineVisWidth := lipgloss.Width(line)\n\t\tfillWidth := m.width - startCol - lineVisWidth\n\t\tif fillWidth < 0 {\n\t\t\tfillWidth = 0\n\t\t}\n\t\tpaddedLine := line + strings.Repeat(" ", fillWidth)\n\t\tresult += paddedLine\n\n\t\tbaseLines[row] = result'
content = content.replace(old_ov2, new_ov2, 1)
print('overlayCenter fill-padding: UPDATED')

with open('pkg/tui/models/tui_model.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('ALL STRUCTURAL CHANGES DONE')
