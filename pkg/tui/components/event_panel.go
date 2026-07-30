package components

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lingshu/lingshu/pkg/tui/styles"
)

// EventType categorizes autonomous events.
type EventType string

const (
	EventScan    EventType = "scan"    // Periodic cluster scan
	EventAlert   EventType = "alert"   // Issue discovered
	EventFix     EventType = "fix"     // Auto-fix attempted
	EventDone    EventType = "done"    // Diagnosis/fix completed
	EventInfo    EventType = "info"    // Informational
)

// EventEntry is one line in the inspection event panel.
type EventEntry struct {
	Time    time.Time
	Type    EventType
	Message string
	Detail  string // Optional detail line
}

// EventPanel is a scrollable real-time log of autonomous inspection events.
type EventPanel struct {
	styles    *styles.Styles
	entries   []EventEntry
	width     int
	height    int
	scrollPos int
	mu        sync.Mutex
}

// NewEventPanel creates an inspection event panel.
func NewEventPanel(s *styles.Styles) *EventPanel {
	return &EventPanel{
		styles:  s,
		entries: make([]EventEntry, 0, 256),
		width:   80,
		height:  10,
	}
}

// AddEvent appends an event and returns a tea.Cmd (nil for now).
func (ep *EventPanel) AddEvent(e EventEntry) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.entries = append(ep.entries, e)
	// Auto-scroll to latest
	if len(ep.entries) > ep.height {
		ep.scrollPos = len(ep.entries) - ep.height
	}
}

// EventCount returns the total number of events.
func (ep *EventPanel) EventCount() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return len(ep.entries)
}

func (ep *EventPanel) Init() tea.Cmd { return nil }

func (ep *EventPanel) Update(msg tea.Msg) (*EventPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ep.width = msg.Width
		return ep, nil
	}
	return ep, nil
}

// SetSize adjusts dimensions.
func (ep *EventPanel) SetSize(w, h int) {
	ep.width = w
	ep.height = h
}

// Height returns current height.
func (ep *EventPanel) Height() int {
	if ep.height == 0 {
		return 10
	}
	return ep.height
}

// ScrollUp scrolls the panel up.
func (ep *EventPanel) ScrollUp(n int) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.scrollPos -= n
	if ep.scrollPos < 0 {
		ep.scrollPos = 0
	}
}

// ScrollDown scrolls the panel down.
func (ep *EventPanel) ScrollDown(n int) {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	maxScroll := len(ep.entries) - ep.height
	if maxScroll < 0 {
		ep.scrollPos = 0
		return
	}
	ep.scrollPos += n
	if ep.scrollPos > maxScroll {
		ep.scrollPos = maxScroll
	}
}

// View renders the event panel.
func (ep *EventPanel) View() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if len(ep.entries) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(ep.styles.Theme.Muted).
			Italic(true).
			Width(ep.width).
			Align(lipgloss.Center).
			Padding(1, 0)
		return emptyStyle.Render("🔍 巡检等待中... (每30秒自动扫描集群)")
	}

	visibleStart := ep.scrollPos
	visibleEnd := ep.scrollPos + ep.height
	if visibleEnd > len(ep.entries) {
		visibleEnd = len(ep.entries)
	}
	if visibleStart < 0 {
		visibleStart = 0
	}
	if visibleStart >= len(ep.entries) {
		visibleStart = 0
		visibleEnd = minInt(len(ep.entries), ep.height)
	}

	timeStyle := lipgloss.NewStyle().
		Foreground(ep.styles.Theme.Muted).
		Width(10)

	var sb strings.Builder
	for i := visibleStart; i < visibleEnd; i++ {
		e := ep.entries[i]
		timeStr := e.Time.Format("15:04:05")

		icon, color := ep.eventIcon(e.Type)
		iconStyle := lipgloss.NewStyle().Foreground(color).Width(3)

		msgStyle := lipgloss.NewStyle().
			Width(ep.width - 14).
			Foreground(ep.styles.Theme.Foreground)

		line := lipgloss.JoinHorizontal(lipgloss.Top,
			timeStyle.Render(timeStr),
			iconStyle.Render(icon),
			msgStyle.Render(e.Message),
		)
		sb.WriteString(line)
		sb.WriteByte('\n')

		if e.Detail != "" {
			detailStyle := lipgloss.NewStyle().
				Foreground(ep.styles.Theme.Muted).
				PaddingLeft(13).
				Width(ep.width - 13)
			sb.WriteString(detailStyle.Render(fmt.Sprintf("→ %s", e.Detail)))
			sb.WriteByte('\n')
		}
	}

	// Scrollbar indicator
	if len(ep.entries) > ep.height {
		scrollbar := ep.renderScrollbar(visibleStart, len(ep.entries))
		return sb.String() + scrollbar
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (ep *EventPanel) eventIcon(t EventType) (string, lipgloss.Color) {
	switch t {
	case EventScan:
		return "🔍", ep.styles.Theme.Muted
	case EventAlert:
		return "🚨", ep.styles.Theme.Error
	case EventFix:
		return "🔧", lipgloss.Color("#f39c12")
	case EventDone:
		return "✅", lipgloss.Color("#27ae60")
	case EventInfo:
		return "ℹ️", ep.styles.Theme.Secondary
	default:
		return "•", ep.styles.Theme.Muted
	}
}

func (ep *EventPanel) renderScrollbar(visibleStart, total int) string {
	if total == 0 {
		return ""
	}
	thumbTop := float64(visibleStart) / float64(total)
	thumbSize := float64(ep.height) / float64(total)
	if thumbSize < 0.05 {
		thumbSize = 0.05
	}

	barHeight := ep.height
	thumbStart := int(thumbTop * float64(barHeight))
	thumbEnd := thumbStart + int(thumbSize*float64(barHeight))
	if thumbEnd > barHeight {
		thumbEnd = barHeight
	}

	var sb strings.Builder
	for i := 0; i < barHeight; i++ {
		if i >= thumbStart && i < thumbEnd {
			sb.WriteString("█")
		} else {
			sb.WriteString("│")
		}
	}
	return "\n" + sb.String()
}

// uses minInt from utils.go for Go 1.21 compatibility
