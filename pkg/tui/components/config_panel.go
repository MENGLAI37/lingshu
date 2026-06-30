package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

func (c *ConfigPanel) Update(msg tea.Msg) (*ConfigPanel, tea.Cmd) {
	if !c.visible {
		return c, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch c.mode {
		case modeList:
			return c.handleListMode(msg)
		case modeEdit, modeAdd:
			return c.handleEditMode(msg)
		}
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
	}

	if c.mode == modeEdit || c.mode == modeAdd {
		var cmd tea.Cmd
		var cmds []tea.Cmd
		for i := range c.inputFields {
			c.inputFields[i], cmd = c.inputFields[i].Update(msg)
			cmds = append(cmds, cmd)
		}
		return c, tea.Batch(cmds...)
	}

	return c, nil
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
			case "a", "A":
				c.mode = modeAdd
				c.selectedIndex = -1
				c.initAddFields()
				return c, c.inputFields[0].Focus()
			case "d", "D":
				if len(c.providers) > 0 {
					return c, c.deleteProvider()
				}
			case "s", "S":
				return c, c.saveConfig()
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

func (c *ConfigPanel) handleEditMode(msg tea.KeyMsg) (*ConfigPanel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		c.switchInputField(1)
	case tea.KeyShiftTab:
		c.switchInputField(-1)
	case tea.KeyEnter:
		if msg.Alt {
			return c, c.saveConfig()
		}
	case tea.KeyEsc:
		c.mode = modeList
		c.errMsg = ""
		for i := range c.inputFields {
			c.inputFields[i].Blur()
		}
	}
	return c, nil
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
	c.inputFields[2].EchoMode = textinput.EchoPassword
	c.inputFields[2].EchoCharacter = '*'

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
	c.inputFields[2].EchoMode = textinput.EchoPassword
	c.inputFields[2].EchoCharacter = '*'

	c.inputFields[3] = textinput.New()
	c.inputFields[3].SetValue("")
	c.inputFields[3].Placeholder = "Base URL (e.g., https://api.deepseek.com/v1)"
	c.inputFields[3].Width = 40
}

func (c *ConfigPanel) saveConfig() tea.Cmd {
	if c.mode == modeEdit {
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
	} else if c.mode == modeAdd {
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

	var content string

	switch c.mode {
	case modeList:
		content = c.renderListMode()
	case modeEdit:
		content = c.renderEditMode("编辑 Provider")
	case modeAdd:
		content = c.renderEditMode("添加 Provider")
	}

	return c.styles.BorderActive.Render(content)
}

func (c *ConfigPanel) renderListMode() string {
	header := c.styles.Title.Render("⚙️  LLM 配置管理") + "\n"

	var listContent string
	if len(c.providers) == 0 {
		listContent = c.styles.Help.Render("暂无配置的 Provider\n按 A 添加新 Provider")
	} else {
		currentProvider := config.GetLLMConfig().CurrentProvider
		for i, p := range c.providers {
			selected := " "
			if i == c.currentIndex {
				selected = "▶"
			}
			active := ""
			if p.Name == currentProvider {
				active = c.styles.StatusOK.Render(" [当前]")
			}
			line := fmt.Sprintf("%s %s%s\n",
				c.styles.Header.Render(selected),
				p.Name,
				active,
			)
			line += fmt.Sprintf("   Model: %s\n", p.Model)
			line += fmt.Sprintf("   BaseURL: %s\n", p.BaseURL)
			if i < len(c.providers)-1 {
				line += "\n"
			}
			listContent += line
		}
	}

	footer := "\n" + c.styles.Help.Render("快捷键: ↑↓ 选择 | Enter 编辑 | A 添加 | D 删除 | S 保存 | Esc 退出")

	if c.errMsg != "" {
		footer = "\n" + c.styles.StatusError.Render("错误: "+c.errMsg) + footer
	}

	return header + "\n" + listContent + footer
}

func (c *ConfigPanel) renderEditMode(title string) string {
	header := c.styles.Title.Render(title) + "\n\n"

	fields := []struct {
		label string
		input textinput.Model
	}{
		{"名称:", c.inputFields[0]},
		{"模型:", c.inputFields[1]},
		{"API Key:", c.inputFields[2]},
		{"Base URL:", c.inputFields[3]},
	}

	formContent := ""
	for _, f := range fields {
		formContent += f.label + " " + f.input.View() + "\n\n"
	}

	footer := c.styles.Help.Render("快捷键: Tab 切换字段 | Alt+Enter 保存 | Esc 返回")

	if c.errMsg != "" {
		footer = c.styles.StatusError.Render("错误: "+c.errMsg) + "\n" + footer
	}

	return header + formContent + footer
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