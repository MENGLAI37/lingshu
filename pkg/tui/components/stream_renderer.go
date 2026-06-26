package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

type StreamRenderer struct {
	styles     *styles.Styles
	content    string
	width      int
	height     int
	streaming  bool
	cursorPos  int
	lines      []string
	scrollPos  int
	maxLines   int
}

type StreamChunkMsg struct {
	Chunk string
	Done  bool
}

func NewStreamRenderer(s *styles.Styles) *StreamRenderer {
	return &StreamRenderer{
		styles:   s,
		content:  "",
		width:    80,
		height:   20,
		scrollPos: 0,
		maxLines: 1000,
	}
}

func (s *StreamRenderer) Init() tea.Cmd {
	return nil
}

func (s *StreamRenderer) Update(msg tea.Msg) (*StreamRenderer, tea.Cmd) {
	switch msg := msg.(type) {
	case StreamChunkMsg:
		if msg.Done {
			s.streaming = false
			return s, nil
		}
		s.content += msg.Chunk
		s.lines = strings.Split(s.content, "\n")
		if len(s.lines) > s.maxLines {
			s.lines = s.lines[len(s.lines)-s.maxLines:]
			s.content = strings.Join(s.lines, "\n")
		}
		if s.scrollPos == 0 || s.scrollPos >= len(s.lines)-s.height {
			s.scrollToBottom()
		}
		return s, nil

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyPgUp:
			s.ScrollUp(s.height / 2)
			return s, nil
		case tea.KeyPgDown:
			s.ScrollDown(s.height / 2)
			return s, nil
		case tea.KeyHome:
			s.scrollPos = 0
			return s, nil
		case tea.KeyEnd:
			s.scrollToBottom()
			return s, nil
		case tea.KeyUp:
			if msg.Alt {
				s.ScrollUp(1)
				return s, nil
			}
		case tea.KeyDown:
			if msg.Alt {
				s.ScrollDown(1)
				return s, nil
			}
		}
	}

	return s, nil
}

func (s *StreamRenderer) View() string {
	visibleLines := s.getVisibleLines()
	content := strings.Join(visibleLines, "\n")

	if s.streaming {
		cursorStyle := s.styles.InputCursor
		content += cursorStyle.Render("▋")
	}

	return s.styles.AIMessage.Render(content)
}

func (s *StreamRenderer) getVisibleLines() []string {
	if len(s.lines) == 0 {
		return []string{}
	}

	start := s.scrollPos
	end := start + s.height
	if end > len(s.lines) {
		end = len(s.lines)
	}
	if start > len(s.lines) {
		start = len(s.lines) - 1
		if start < 0 {
			start = 0
		}
	}

	return s.lines[start:end]
}

func (s *StreamRenderer) scrollToBottom() {
	if len(s.lines) > s.height {
		s.scrollPos = len(s.lines) - s.height
	} else {
		s.scrollPos = 0
	}
}

func (s *StreamRenderer) ScrollUp(n int) {
	s.scrollPos -= n
	if s.scrollPos < 0 {
		s.scrollPos = 0
	}
}

func (s *StreamRenderer) ScrollDown(n int) {
	s.scrollPos += n
	maxScroll := len(s.lines) - s.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.scrollPos > maxScroll {
		s.scrollPos = maxScroll
	}
}

func (s *StreamRenderer) AppendChunk(chunk string) {
	s.content += chunk
	s.lines = strings.Split(s.content, "\n")
	if len(s.lines) > s.maxLines {
		s.lines = s.lines[len(s.lines)-s.maxLines:]
		s.content = strings.Join(s.lines, "\n")
	}
	if s.scrollPos == 0 || s.scrollPos >= len(s.lines)-s.height {
		s.scrollToBottom()
	}
}

func (s *StreamRenderer) SetContent(content string) {
	s.content = content
	s.lines = strings.Split(content, "\n")
	s.scrollToBottom()
}

func (s *StreamRenderer) Content() string {
	return s.content
}

func (s *StreamRenderer) SetStreaming(streaming bool) {
	s.streaming = streaming
}

func (s *StreamRenderer) Streaming() bool {
	return s.streaming
}

func (s *StreamRenderer) Clear() {
	s.content = ""
	s.lines = []string{}
	s.scrollPos = 0
	s.streaming = false
}

func (s *StreamRenderer) SetWidth(w int) {
	s.width = w
}

func (s *StreamRenderer) SetHeight(h int) {
	s.height = h
}

func (s *StreamRenderer) ScrollPercent() float64 {
	if len(s.lines) <= s.height {
		return 1.0
	}
	return float64(s.scrollPos) / float64(len(s.lines)-s.height)
}

func (s *StreamRenderer) Scrollbar() string {
	if len(s.lines) <= s.height {
		return strings.Repeat(" ", s.height)
	}

	thumbSize := maxInt(1, s.height*s.height/len(s.lines))
	thumbPos := int(float64(s.height-thumbSize) * s.ScrollPercent())

	var sb strings.Builder
	for i := 0; i < s.height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(lipgloss.NewStyle().Foreground(s.styles.Theme.Primary).Render("█"))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(s.styles.Theme.Muted).Render("░"))
		}
	}
	return sb.String()
}
