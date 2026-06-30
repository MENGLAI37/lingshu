package components

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
	"gopkg.in/yaml.v3"
)

type ContentType string

const (
	ContentText  ContentType = "text"
	ContentJSON  ContentType = "json"
	ContentYAML  ContentType = "yaml"
	ContentTable ContentType = "table"
)

type HighlightedRenderer struct {
	styles       *styles.Styles
	content      string
	contentType  ContentType
	width        int
	height       int
	lines        []string
	scrollPos    int
	collapsed    map[int]bool
	collapsible  []int
	selectedLine int
	visible      bool
	title        string
}

type ToggleFoldMsg struct {
	Line int
}

func NewHighlightedRenderer(s *styles.Styles) *HighlightedRenderer {
	return &HighlightedRenderer{
		styles:      s,
		contentType: ContentText,
		collapsed:   make(map[int]bool),
		collapsible: []int{},
		visible:     false,
	}
}

func (h *HighlightedRenderer) Init() tea.Cmd {
	return nil
}

func (h *HighlightedRenderer) Update(msg tea.Msg) (*HighlightedRenderer, tea.Cmd) {
	if !h.visible {
		return h, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if h.selectedLine > 0 {
				h.selectedLine--
				h.ensureVisible()
			}
			return h, nil
		case tea.KeyDown:
			if h.selectedLine < len(h.lines)-1 {
				h.selectedLine++
				h.ensureVisible()
			}
			return h, nil
		case tea.KeyPgUp:
			h.scrollPos -= h.height / 2
			if h.scrollPos < 0 {
				h.scrollPos = 0
			}
			h.selectedLine = h.scrollPos
			return h, nil
		case tea.KeyPgDown:
			h.scrollPos += h.height / 2
			maxScroll := maxInt(0, len(h.lines)-h.height)
			if h.scrollPos > maxScroll {
				h.scrollPos = maxScroll
			}
			h.selectedLine = h.scrollPos
			return h, nil
		case tea.KeySpace, tea.KeyEnter:
			if h.isCollapsible(h.selectedLine) {
				h.toggleFold(h.selectedLine)
			}
			return h, nil
		case tea.KeyHome:
			h.scrollPos = 0
			h.selectedLine = 0
			return h, nil
		case tea.KeyEnd:
			h.scrollPos = maxInt(0, len(h.lines)-h.height)
			h.selectedLine = len(h.lines) - 1
			return h, nil
		}

	case tea.WindowSizeMsg:
		h.width = msg.Width
		h.height = msg.Height
		return h, nil
	}

	return h, nil
}

func (h *HighlightedRenderer) View() string {
	if !h.visible {
		return ""
	}

	visibleLines := h.getVisibleLines()
	var rendered []string

	for i, line := range visibleLines {
		actualLine := h.scrollPos + i
		lineNum := fmt.Sprintf("%4d ", actualLine+1)
		lineNumStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Muted)

		if actualLine == h.selectedLine {
			lineNumStyle = lineNumStyle.Background(h.styles.Theme.Selection).Foreground(h.styles.Theme.Primary)
		}

		foldIndicator := "  "
		if h.isCollapsible(actualLine) {
			if h.collapsed[actualLine] {
				foldIndicator = "▶ "
			} else {
				foldIndicator = "▼ "
			}
			foldIndicator = lipgloss.NewStyle().Foreground(h.styles.Theme.Primary).Render(foldIndicator)
		}

		highlighted := h.highlightLine(line, actualLine)

		if actualLine == h.selectedLine {
			highlighted = lipgloss.NewStyle().Background(h.styles.Theme.Selection).Render(highlighted)
		}

		rendered = append(rendered, lineNumStyle.Render(lineNum)+foldIndicator+highlighted)
	}

	content := strings.Join(rendered, "\n")

	header := h.styles.Title.Render(fmt.Sprintf("📄 %s [%s]", h.title, strings.ToUpper(string(h.contentType))))
	help := h.styles.Help.Render("↑/↓: 移动 | Space/Enter: 折叠/展开 | Home/End: 跳转 | PgUp/PgDn: 翻页")

	body := lipgloss.NewStyle().
		Width(h.width - 4).
		Height(h.height - 4).
		Render(content)

	return h.styles.Border.Render(header + "\n" + body + "\n" + help)
}

func (h *HighlightedRenderer) highlightLine(line string, lineNum int) string {
	switch h.contentType {
	case ContentJSON:
		return h.highlightJSON(line)
	case ContentYAML:
		return h.highlightYAML(line)
	case ContentTable:
		return h.highlightTable(line)
	default:
		return line
	}
}

func (h *HighlightedRenderer) highlightJSON(line string) string {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") ||
		strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "]") {
		return lipgloss.NewStyle().Foreground(h.styles.Theme.Primary).Bold(true).Render(line)
	}

	if idx := strings.Index(trimmed, ":"); idx > 0 {
		key := trimmed[:idx]
		value := trimmed[idx:]

		key = strings.Trim(key, `", `)
		keyStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Accent)

		if strings.Contains(value, `"`) {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Success)
			return keyStyle.Render(`"`+key+`"`) + valueStyle.Render(value)
		}
		if strings.Contains(value, "true") || strings.Contains(value, "false") {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Secondary)
			return keyStyle.Render(`"`+key+`"`) + valueStyle.Render(value)
		}
		if strings.Contains(value, "null") {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Muted)
			return keyStyle.Render(`"`+key+`"`) + valueStyle.Render(value)
		}

		valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Warning)
		return keyStyle.Render(`"`+key+`"`) + valueStyle.Render(value)
	}

	return line
}

func (h *HighlightedRenderer) highlightYAML(line string) string {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
		return lipgloss.NewStyle().Foreground(h.styles.Theme.Muted).Italic(true).Render(line)
	}

	if idx := strings.Index(trimmed, ":"); idx > 0 {
		key := trimmed[:idx]
		value := trimmed[idx+1:]

		keyStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Primary).Bold(true)
		indent := line[:strings.Index(line, trimmed)]

		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(value, "|") || strings.HasPrefix(value, ">") {
			return indent + keyStyle.Render(key+":") + value
		}

		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Success)
			return indent + keyStyle.Render(key+": ") + valueStyle.Render(value)
		}

		if strings.HasPrefix(value, "true") || strings.HasPrefix(value, "false") ||
			strings.HasPrefix(value, "yes") || strings.HasPrefix(value, "no") {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Secondary)
			return indent + keyStyle.Render(key+": ") + valueStyle.Render(value)
		}

		if strings.HasPrefix(value, "null") || strings.HasPrefix(value, "~") || value == "" {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Muted)
			return indent + keyStyle.Render(key+": ") + valueStyle.Render(value)
		}

		if isNumeric(value) {
			valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Warning)
			return indent + keyStyle.Render(key+": ") + valueStyle.Render(value)
		}

		valueStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Accent)
		return indent + keyStyle.Render(key+": ") + valueStyle.Render(value)
	}

	if strings.HasPrefix(trimmed, "- ") {
		bulletStyle := lipgloss.NewStyle().Foreground(h.styles.Theme.Primary).Bold(true)
		indent := line[:strings.Index(line, "-")]
		return indent + bulletStyle.Render("-") + " " + strings.TrimPrefix(trimmed, "- ")
	}

	return line
}

func (h *HighlightedRenderer) highlightTable(line string) string {
	if strings.Contains(line, "---") || strings.Contains(line, "===") {
		return lipgloss.NewStyle().Foreground(h.styles.Theme.Muted).Render(line)
	}

	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "|") && strings.Contains(line, "|") {
		cells := strings.Split(line, "|")
		var styled []string
		for i, cell := range cells {
			if i == 0 {
				styled = append(styled, lipgloss.NewStyle().Foreground(h.styles.Theme.Primary).Bold(true).Render(cell))
			} else {
				styled = append(styled, cell)
			}
		}
		return strings.Join(styled, "|")
	}

	return line
}

func (h *HighlightedRenderer) getVisibleLines() []string {
	if len(h.lines) == 0 {
		return []string{}
	}

	var result []string
	skipUntil := -1

	for i, line := range h.lines {
		if i < h.scrollPos {
			if h.collapsed[i] && h.isCollapsible(i) {
				skipUntil = h.findBlockEnd(i)
			}
			if skipUntil > 0 && i < skipUntil {
				continue
			}
			continue
		}

		if len(result) >= h.height {
			break
		}

		if skipUntil > 0 && i < skipUntil {
			continue
		}

		result = append(result, line)

		if h.collapsed[i] && h.isCollapsible(i) {
			skipUntil = h.findBlockEnd(i)
			if skipUntil > i {
				collapsedCount := skipUntil - i - 1
				indent := strings.Repeat(" ", getIndent(line))
				placeholder := fmt.Sprintf("%s  ... (%d lines collapsed)", indent, collapsedCount)
				result[len(result)-1] = placeholder
			}
		}
	}

	return result
}

func (h *HighlightedRenderer) isCollapsible(lineNum int) bool {
	for _, l := range h.collapsible {
		if l == lineNum {
			return true
		}
	}
	return false
}

func (h *HighlightedRenderer) findBlockEnd(start int) int {
	if start >= len(h.lines)-1 {
		return start
	}

	baseIndent := getIndent(h.lines[start])

	for i := start + 1; i < len(h.lines); i++ {
		if strings.TrimSpace(h.lines[i]) == "" {
			continue
		}
		indent := getIndent(h.lines[i])
		if indent <= baseIndent {
			return i
		}
	}

	return len(h.lines)
}

func (h *HighlightedRenderer) toggleFold(line int) {
	h.collapsed[line] = !h.collapsed[line]
}

func (h *HighlightedRenderer) ensureVisible() {
	if h.selectedLine < h.scrollPos {
		h.scrollPos = h.selectedLine
	}
	if h.selectedLine >= h.scrollPos+h.height {
		h.scrollPos = h.selectedLine - h.height + 1
	}
}

func getIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, ch := range s {
		if (ch < '0' || ch > '9') && ch != '.' && ch != '-' && ch != 'e' && ch != 'E' {
			return false
		}
	}
	return true
}

func (h *HighlightedRenderer) SetContent(content string, contentType ContentType, title string) {
	h.content = content
	h.contentType = contentType
	h.title = title
	h.lines = strings.Split(content, "\n")
	h.scrollPos = 0
	h.selectedLine = 0
	h.collapsed = make(map[int]bool)
	h.identifyCollapsibleRegions()
}

func (h *HighlightedRenderer) identifyCollapsibleRegions() {
	h.collapsible = []int{}

	switch h.contentType {
	case ContentYAML, ContentJSON:
		var stack []int
		for i, line := range h.lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}

			indent := getIndent(line)
			for len(stack) > 0 && indent <= stack[len(stack)-1] {
				stack = stack[:len(stack)-1]
			}

			if h.isBlockStart(line) {
				h.collapsible = append(h.collapsible, i)
				stack = append(stack, indent)
			}
		}
	}
}

func (h *HighlightedRenderer) isBlockStart(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch h.contentType {
	case ContentYAML:
		return strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "|") && !strings.Contains(trimmed, ">")
	case ContentJSON:
		return strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "[")
	}
	return false
}

func (h *HighlightedRenderer) Show() {
	h.visible = true
}

func (h *HighlightedRenderer) Hide() {
	h.visible = false
}

func (h *HighlightedRenderer) Visible() bool {
	return h.visible
}

func (h *HighlightedRenderer) SetWidth(w int) {
	h.width = w
}

func (h *HighlightedRenderer) SetHeight(hgt int) {
	h.height = hgt
}

func FormatJSON(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FormatYAML(data interface{}) (string, error) {
	b, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
