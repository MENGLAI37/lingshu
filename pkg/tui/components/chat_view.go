package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type MessageRole string

const (
	RoleUser    MessageRole = "user"
	RoleAI      MessageRole = "ai"
	RoleSystem  MessageRole = "system"
	RoleTool    MessageRole = "tool"
)

type ChatMessage struct {
	ID        string
	Role      MessageRole
	Content   string
	Timestamp time.Time
	Streaming bool
	Type      string
}

type ChatView struct {
	styles     *styles.Styles
	messages   []ChatMessage
	width      int
	height     int
	scrollPos  int
	showTime   bool
}

func NewChatView(s *styles.Styles) *ChatView {
	return &ChatView{
		styles:    s,
		messages:  []ChatMessage{},
		width:     80,
		height:    20,
		scrollPos: 0,
		showTime:  true,
	}
}

func (c *ChatView) Init() tea.Cmd {
	return nil
}

func (c *ChatView) Update(msg tea.Msg) (*ChatView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
		return c, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyPgUp:
			c.ScrollUp(c.height / 2)
			return c, nil
		case tea.KeyPgDown:
			c.ScrollDown(c.height / 2)
			return c, nil
		case tea.KeyHome:
			c.scrollPos = 0
			return c, nil
		case tea.KeyEnd:
			c.scrollToBottom()
			return c, nil
		}
	}

	return c, nil
}

func (c *ChatView) View() string {
	allLines := c.renderAllLines()

	start := c.scrollPos
	end := start + c.height
	if end > len(allLines) {
		end = len(allLines)
	}
	if start > len(allLines) {
		start = maxInt(0, len(allLines)-c.height)
	}

	visibleLines := allLines[start:end]

	for len(visibleLines) < c.height {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

func (c *ChatView) renderAllLines() []string {
	var allLines []string

	for i, msg := range c.messages {
		msgLines := c.renderMessage(msg, i == len(c.messages)-1)
		allLines = append(allLines, msgLines...)
		allLines = append(allLines, "")
	}

	return allLines
}

func (c *ChatView) renderMessage(msg ChatMessage, isLast bool) []string {
	var prefix string
	var msgStyle lipgloss.Style

	switch msg.Role {
	case RoleUser:
		prefix = "👤 你: "
		msgStyle = lipgloss.NewStyle().Foreground(c.styles.Theme.Primary)
	case RoleAI:
		prefix = "🤖 AI: "
		msgStyle = lipgloss.NewStyle().Foreground(c.styles.Theme.Foreground)
	case RoleSystem:
		prefix = "ℹ️  系统: "
		msgStyle = lipgloss.NewStyle().Foreground(c.styles.Theme.Muted).Italic(true)
	case RoleTool:
		prefix = "🔧 工具: "
		msgStyle = lipgloss.NewStyle().Foreground(c.styles.Theme.Secondary)
	default:
		prefix = "   "
		msgStyle = lipgloss.NewStyle().Foreground(c.styles.Theme.Foreground)
	}

	timeStr := ""
	if c.showTime {
		timeStr = fmt.Sprintf(" [%s]", msg.Timestamp.Format("15:04:05"))
		timeStr = lipgloss.NewStyle().Foreground(c.styles.Theme.Muted).Render(timeStr)
	}

	header := msgStyle.Bold(true).Render(prefix) + timeStr

	wrappedContent := wrapText(msg.Content, c.width-4)
	var contentLines []string
	for _, line := range wrappedContent {
		contentLines = append(contentLines, "   "+line)
	}

	result := []string{header}
	result = append(result, contentLines...)

	if msg.Streaming && isLast {
		cursorLine := "   " + lipgloss.NewStyle().
			Foreground(c.styles.Theme.Cursor).
			Blink(true).
			Render("▋")
		result = append(result, cursorLine)
	}

	return result
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return strings.Split(text, "\n")
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		currentLine := ""
		currentLen := 0

		for _, r := range para {
			charLen := 1
			if r >= 0x4e00 && r <= 0x9fff || r >= 0x3000 && r <= 0x303f || r >= 0xff00 && r <= 0xffef {
				charLen = 2
			}

			if currentLen+charLen > width && currentLen > 0 {
				lines = append(lines, currentLine)
				currentLine = string(r)
				currentLen = charLen
			} else {
				currentLine += string(r)
				currentLen += charLen
			}
		}

		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	if len(lines) == 0 {
		lines = append(lines, "")
	}

	return lines
}

func (c *ChatView) AddMessage(msg ChatMessage) {
	c.messages = append(c.messages, msg)
	c.scrollToBottom()
}

func (c *ChatView) UpdateLastMessage(content string, streaming bool) {
	if len(c.messages) == 0 {
		c.AddMessage(ChatMessage{
			Role:      RoleAI,
			Content:   content,
			Timestamp: time.Now(),
			Streaming: streaming,
		})
		return
	}

	last := &c.messages[len(c.messages)-1]
	if last.Role == RoleAI {
		last.Content = content
		last.Streaming = streaming
	} else {
		c.AddMessage(ChatMessage{
			Role:      RoleAI,
			Content:   content,
			Timestamp: time.Now(),
			Streaming: streaming,
		})
	}
	c.scrollToBottom()
}

func (c *ChatView) AppendToLastMessage(chunk string) {
	if len(c.messages) == 0 {
		c.AddMessage(ChatMessage{
			Role:      RoleAI,
			Content:   chunk,
			Timestamp: time.Now(),
			Streaming: true,
		})
		return
	}

	last := &c.messages[len(c.messages)-1]
	if last.Role == RoleAI {
		last.Content += chunk
		last.Streaming = true
	} else {
		c.AddMessage(ChatMessage{
			Role:      RoleAI,
			Content:   chunk,
			Timestamp: time.Now(),
			Streaming: true,
		})
	}
	c.scrollToBottom()
}

// AppendToLastAIMessage appends content to the last AI message,
// skipping over any intermediate Tool/System messages.
// This ensures the AI response stays in one contiguous message even when
// tool results are interleaved.
func (c *ChatView) AppendToLastAIMessage(chunk string) {
	aiIdx := -1
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == RoleAI {
			aiIdx = i
			break
		}
	}
	if aiIdx >= 0 {
		c.messages[aiIdx].Content += chunk
		c.messages[aiIdx].Streaming = true
		c.scrollToBottom()
		return
	}
	// No AI message found, create a new one
	c.AddMessage(ChatMessage{
		Role:      RoleAI,
		Content:   chunk,
		Timestamp: time.Now(),
		Streaming: true,
	})
}

// FinishLastAIStreaming marks the last AI message (searching backwards) as not streaming.
func (c *ChatView) FinishLastAIStreaming() {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == RoleAI && c.messages[i].Streaming {
			c.messages[i].Streaming = false
			return
		}
	}
}

func (c *ChatView) FinishStreaming() {
	if len(c.messages) > 0 {
		c.messages[len(c.messages)-1].Streaming = false
	}
}

func (c *ChatView) UpdateLastToolMessage(content string) {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == RoleTool {
			c.messages[i].Content = content
			c.messages[i].Streaming = false
			c.scrollToBottom()
			return
		}
	}
	c.AddMessage(ChatMessage{
		Role:      RoleTool,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (c *ChatView) scrollToBottom() {
	totalLines := len(c.renderAllLines())
	if totalLines > c.height {
		c.scrollPos = totalLines - c.height
	} else {
		c.scrollPos = 0
	}
}

func (c *ChatView) countTotalLines() int {
	return len(c.renderAllLines())
}

func (c *ChatView) ScrollUp(n int) {
	c.scrollPos -= n
	if c.scrollPos < 0 {
		c.scrollPos = 0
	}
}

func (c *ChatView) ScrollDown(n int) {
	c.scrollPos += n
	totalLines := c.countTotalLines()
	maxScroll := maxInt(0, totalLines-c.height)
	if c.scrollPos > maxScroll {
		c.scrollPos = maxScroll
	}
}

func (c *ChatView) ScrollToTop() {
	c.scrollPos = 0
}

func (c *ChatView) ScrollToBottom() {
	c.scrollToBottom()
}

func (c *ChatView) Clear() {
	c.messages = []ChatMessage{}
	c.scrollPos = 0
}

func (c *ChatView) Messages() []ChatMessage {
	return c.messages
}

func (c *ChatView) SetWidth(w int) {
	c.width = w
}

func (c *ChatView) SetHeight(h int) {
	c.height = h
}

func (c *ChatView) Height() int {
	return c.height
}
