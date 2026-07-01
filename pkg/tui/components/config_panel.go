package components

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/config"
	"github.com/lingshu/lingshu/pkg/llm"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type ConfigPanel struct {
	styles        *styles.Styles
	visible       bool
	providers     []llm.ProviderConfig
	currentIndex  int
	selectedIndex int
	mode          configMode
	inputFields   []textinput.Model
	width         int
	height        int
	errMsg        string
}

type configMode string

const (
	modeList   configMode = "list"
	modeEdit   configMode = "edit"
	modeAdd    configMode = "add"
)

type ConfigSavedMsg struct{}
type ConfigCancelledMsg struct{}

func NewConfigPanel(s *styles.Styles) *ConfigPanel {
	return &ConfigPanel{
		styles:      s,
		visible:     false,
		currentIndex: 0,
		mode:        modeList,
		inputFields: make([]textinput.Model, 4),
	}
}

func (c *ConfigPanel) Init() tea.Cmd {
	return nil
}

// IsEditing returns true if the panel is in edit or add mode
func (c *ConfigPanel) IsEditing() bool {
	return c.mode == modeEdit || c.mode == modeAdd
}

func (c *ConfigPanel) Update(msg tea.Msg) (*ConfigPanel, tea.Cmd) {
	if !c.visible {
		return c, nil
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	pasteHandled := false

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch c.mode {
		case modeList:
			return c.handleListMode(msg)
		case modeEdit, modeAdd:
			c, cmd, pasteHandled = c.handleEditMode(msg)
			cmds = append(cmds, cmd)
		}
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
	}

	// Only update inputFields if paste was not handled manually
	// to avoid duplicate paste content
	if c.mode == modeEdit || c.mode == modeAdd {
		for i := range c.inputFields {
			if !pasteHandled {
				c.inputFields[i], cmd = c.inputFields[i].Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return c, tea.Batch(cmds...)
}

func (c *ConfigPanel) handleListMode(msg tea.KeyMsg) (*ConfigPanel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if c.currentIndex > 0 {
			c.currentIndex--
		}
	case tea.KeyDown:
		if c.currentIndex < len(c.providers)-1 {
			c.currentIndex++
		}
	case tea.KeyEnter:
		if len(c.providers) > 0 {
			c.mode = modeEdit
			c.selectedIndex = c.currentIndex
			c.initEditFields()
			return c, c.inputFields[0].Focus()
		}
	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			switch string(msg.Runes[0]) {
			case "a", "A", "n", "N":
				c.mode = modeAdd
				c.selectedIndex = -1
				c.initAddFields()
				return c, c.inputFields[0].Focus()
			case "d", "D", "x", "X":
				if len(c.providers) > 0 {
					return c, c.deleteProvider()
				}
			case "s", "S":
				return c, c.saveConfig()
			case "e", "E":
				if len(c.providers) > 0 {
					c.mode = modeEdit
					c.selectedIndex = c.currentIndex
					c.initEditFields()
					return c, c.inputFields[0].Focus()
				}
			case "r", "R":
				c.loadProviders()
				return c, nil
			}
		}
	case tea.KeyEsc:
		c.visible = false
		c.mode = modeList
		return c, func() tea.Msg {
			return ConfigCancelledMsg{}
		}
	}
	return c, nil
}

func (c *ConfigPanel) handleEditMode(msg tea.KeyMsg) (*ConfigPanel, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyTab:
		c.switchInputField(1)
	case tea.KeyShiftTab:
		c.switchInputField(-1)
	case tea.KeyEnter:
		if msg.Alt {
			return c, c.saveConfig(), false
		}
	case tea.KeyCtrlS:
		return c, c.saveConfig(), false
	case tea.KeyCtrlV:
		if content, err := clipboard.ReadAll(); err == nil && content != "" {
			c.insertIntoFocusedField(content)
		}
		return c, nil, true // Mark paste as handled
	case tea.KeyCtrlA:
		c.selectAllCurrentField()
	case tea.KeyEsc:
		c.mode = modeList
		c.errMsg = ""
		for i := range c.inputFields {
			c.inputFields[i].Blur()
		}
	}
	return c, nil, false
}

func (c *ConfigPanel) switchInputField(direction int) {
	for i := range c.inputFields {
		if c.inputFields[i].Focused() {
			c.inputFields[i].Blur()
			nextIdx := (i + direction + len(c.inputFields)) % len(c.inputFields)
			c.inputFields[nextIdx].Focus()
			return
		}
	}
	c.inputFields[0].Focus()
}

func (c *ConfigPanel) insertIntoFocusedField(text string) {
	for i := range c.inputFields {
		if c.inputFields[i].Focused() {
			currentValue := c.inputFields[i].Value()
			c.inputFields[i].SetValue(currentValue + text)
			return
		}
	}
}

func (c *ConfigPanel) selectAllCurrentField() {
	for i := range c.inputFields {
		if c.inputFields[i].Focused() {
			c.inputFields[i].SetValue(c.inputFields[i].Value())
			return
		}
	}
}

func (c *ConfigPanel) initEditFields() {
	if c.selectedIndex < 0 || c.selectedIndex >= len(c.providers) {
		return
	}
	provider := c.providers[c.selectedIndex]

	c.inputFields[0] = textinput.New()
	c.inputFields[0].SetValue(provider.Name)
	c.inputFields[0].Placeholder = "Provider Name"
	c.inputFields[0].Width = 30

	c.inputFields[1] = textinput.New()
	c.inputFields[1].SetValue(provider.Model)
	c.inputFields[1].Placeholder = "Model Name"
	c.inputFields[1].Width = 30

	c.inputFields[2] = textinput.New()
	c.inputFields[2].SetValue(provider.APIKey)
	c.inputFields[2].Placeholder = "API Key"
	c.inputFields[2].Width = 40

	c.inputFields[3] = textinput.New()
	c.inputFields[3].SetValue(provider.BaseURL)
	c.inputFields[3].Placeholder = "Base URL"
	c.inputFields[3].Width = 40
}

func (c *ConfigPanel) initAddFields() {
	c.inputFields[0] = textinput.New()
	c.inputFields[0].SetValue("")
	c.inputFields[0].Placeholder = "Provider Name (e.g., deepseek)"
	c.inputFields[0].Width = 30

	c.inputFields[1] = textinput.New()
	c.inputFields[1].SetValue("")
	c.inputFields[1].Placeholder = "Model Name (e.g., deepseek-chat)"
	c.inputFields[1].Width = 30

	c.inputFields[2] = textinput.New()
	c.inputFields[2].SetValue("")
	c.inputFields[2].Placeholder = "API Key"
	c.inputFields[2].Width = 40

	c.inputFields[3] = textinput.New()
	c.inputFields[3].SetValue("")
	c.inputFields[3].Placeholder = "Base URL (e.g., https://api.deepseek.com/v1)"
	c.inputFields[3].Width = 40
}

func (c *ConfigPanel) saveConfig() tea.Cmd {
	switch c.mode {
	case modeEdit:
		if c.selectedIndex < 0 || c.selectedIndex >= len(c.providers) {
			return nil
		}
		name := strings.TrimSpace(c.inputFields[0].Value())
		if name == "" {
			c.errMsg = "Provider name cannot be empty"
			return nil
		}
		c.providers[c.selectedIndex] = llm.ProviderConfig{
			Name:       name,
			Model:      strings.TrimSpace(c.inputFields[1].Value()),
			APIKey:     strings.TrimSpace(c.inputFields[2].Value()),
			BaseURL:    strings.TrimSpace(c.inputFields[3].Value()),
			Priority:   c.providers[c.selectedIndex].Priority,
			Timeout:    c.providers[c.selectedIndex].Timeout,
			IsLocal:    c.providers[c.selectedIndex].IsLocal,
			MaxRetries: c.providers[c.selectedIndex].MaxRetries,
		}
	case modeAdd:
		name := strings.TrimSpace(c.inputFields[0].Value())
		model := strings.TrimSpace(c.inputFields[1].Value())
		apiKey := strings.TrimSpace(c.inputFields[2].Value())
		baseURL := strings.TrimSpace(c.inputFields[3].Value())

		if name == "" || apiKey == "" {
			c.errMsg = "Provider name and API Key are required"
			return nil
		}
		if model == "" {
			model = "gpt-4o"
		}
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		c.providers = append(c.providers, llm.ProviderConfig{
			Name:       name,
			Model:      model,
			APIKey:     apiKey,
			BaseURL:    baseURL,
			Priority:   len(c.providers) + 1,
			Timeout:    30,
			IsLocal:    false,
			MaxRetries: 3,
		})
		c.currentIndex = len(c.providers) - 1
	}

	currentCfg := config.GetLLMConfig()
	newCfg := &config.LLMConfig{
		CurrentProvider: currentCfg.CurrentProvider,
		Providers:       c.providers,
	}

	if err := config.SaveLLMConfig(newCfg); err != nil {
		c.errMsg = fmt.Sprintf("Failed to save config: %v", err)
		return nil
	}

	c.mode = modeList
	c.errMsg = ""
	for i := range c.inputFields {
		c.inputFields[i].Blur()
	}

	return func() tea.Msg {
		return ConfigSavedMsg{}
	}
}

func (c *ConfigPanel) deleteProvider() tea.Cmd {
	name := c.providers[c.currentIndex].Name
	if err := config.RemoveProvider(name); err != nil {
		c.errMsg = fmt.Sprintf("Failed to delete provider: %v", err)
		return nil
	}

	c.loadProviders()
	if c.currentIndex >= len(c.providers) {
		c.currentIndex = len(c.providers) - 1
	}
	if c.currentIndex < 0 {
		c.currentIndex = 0
	}

	return func() tea.Msg {
		return ConfigSavedMsg{}
	}
}

func (c *ConfigPanel) View() string {
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

	var content string

	switch c.mode {
	case modeList:
		content = c.renderListMode(panelWidth)
	case modeEdit:
		content = c.renderEditMode("编辑 Provider", panelWidth)
	case modeAdd:
		content = c.renderEditMode("添加 Provider", panelWidth)
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c.styles.Theme.Primary).
		Padding(1, 2).
		Width(panelWidth)

	return borderStyle.Render(content)
}

func (c *ConfigPanel) renderListMode(panelWidth int) string {
	title := c.styles.Title.Render("⚙️  LLM 配置管理")
	separator := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Border).
		Render(strings.Repeat("─", panelWidth-4))

	var listContent string
	if len(c.providers) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Muted).
			Italic(true).
			Align(lipgloss.Center).
			Width(panelWidth - 4)
		listContent = emptyStyle.Render("暂无配置的 Provider") + "\n\n" +
			emptyStyle.Render("按 A 添加新 Provider")
	} else {
		currentProvider := config.GetLLMConfig().CurrentProvider
		for i, p := range c.providers {
			isSelected := i == c.currentIndex
			isCurrent := p.Name == currentProvider

			var rowStyle lipgloss.Style
			if isSelected {
				rowStyle = lipgloss.NewStyle().
					Background(c.styles.Theme.Selection).
					Foreground(c.styles.Theme.Primary).
					Bold(true)
			} else {
				rowStyle = lipgloss.NewStyle()
			}

			prefix := "  "
			if isSelected {
				prefix = "▶ "
			}

			nameText := p.Name
			if isCurrent {
				nameText += " " + c.styles.StatusOK.Render("[当前]")
			}

			nameLine := rowStyle.Width(panelWidth - 4).Render(prefix + nameText)
			listContent += nameLine + "\n"

			detailStyle := lipgloss.NewStyle().
				Foreground(c.styles.Theme.Muted).
				PaddingLeft(4)
			if isSelected {
				detailStyle = detailStyle.Background(c.styles.Theme.Selection)
			}

			modelLine := fmt.Sprintf("Model:   %s", p.Model)
			urlLine := fmt.Sprintf("BaseURL: %s", truncateString(p.BaseURL, panelWidth-14))
			listContent += detailStyle.Width(panelWidth - 4).Render(modelLine) + "\n"
			listContent += detailStyle.Width(panelWidth - 4).Render(urlLine) + "\n"

			if i < len(c.providers)-1 {
				listContent += "\n"
			}
		}
	}

	helpText := "↑↓ 选择 | Enter/e 编辑 | A/n 添加 | D/x 删除 | S 保存 | r 刷新 | Esc 退出"
	footerStyle := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Muted).
		Italic(true).
		Align(lipgloss.Center).
		Width(panelWidth - 4)
	footer := footerStyle.Render(helpText)

	var errLine string
	if c.errMsg != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Error).
			Bold(true).
			Align(lipgloss.Center).
			Width(panelWidth - 4)
		errLine = errStyle.Render("✖ "+c.errMsg) + "\n\n"
	}

	return title + "\n" + separator + "\n\n" + listContent + "\n\n" + errLine + footer
}

func (c *ConfigPanel) renderEditMode(title string, panelWidth int) string {
	titleText := c.styles.Title.Render(title)
	separator := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Border).
		Render(strings.Repeat("─", panelWidth-4))

	labelWidth := 10
	fieldWidth := panelWidth - 4 - labelWidth - 2

	fields := []struct {
		label string
		input textinput.Model
	}{
		{"名称", c.inputFields[0]},
		{"模型", c.inputFields[1]},
		{"API Key", c.inputFields[2]},
		{"Base URL", c.inputFields[3]},
	}

	c.inputFields[0].Width = fieldWidth
	c.inputFields[1].Width = fieldWidth
	c.inputFields[2].Width = fieldWidth
	c.inputFields[3].Width = fieldWidth

	var formContent string
	for _, f := range fields {
		labelStyle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Secondary).
			Bold(true).
			Width(labelWidth).
			Align(lipgloss.Right)

		label := labelStyle.Render(f.label + ":")
		inputView := f.input.View()

		formContent += lipgloss.JoinHorizontal(lipgloss.Top, label, " ", inputView) + "\n\n"
	}

	helpText := "Tab 切换字段 | Ctrl+S 保存 | Esc 返回"
	footerStyle := lipgloss.NewStyle().
		Foreground(c.styles.Theme.Muted).
		Italic(true).
		Align(lipgloss.Center).
		Width(panelWidth - 4)
	footer := footerStyle.Render(helpText)

	var errLine string
	if c.errMsg != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(c.styles.Theme.Error).
			Bold(true).
			Align(lipgloss.Center).
			Width(panelWidth - 4)
		errLine = errStyle.Render("✖ "+c.errMsg) + "\n\n"
	}

	return titleText + "\n" + separator + "\n\n" + formContent + errLine + footer
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 3 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (c *ConfigPanel) Show() {
	c.visible = true
	c.loadProviders()
	c.mode = modeList
	c.currentIndex = 0
	c.errMsg = ""
}

func (c *ConfigPanel) Hide() {
	c.visible = false
}

func (c *ConfigPanel) Visible() bool {
	return c.visible
}

func (c *ConfigPanel) loadProviders() {
	cfg := config.GetLLMConfig()
	c.providers = make([]llm.ProviderConfig, len(cfg.Providers))
	copy(c.providers, cfg.Providers)
}

func (c *ConfigPanel) SetWidth(w int) {
	c.width = w
}

func (c *ConfigPanel) SetHeight(h int) {
	c.height = h
}