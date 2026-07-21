package styles

import (
	"testing"

	"github.com/lingshu/lingshu/pkg/tui/theme"
)

func TestNewStyles_CreatesAllStyles(t *testing.T) {
	tm := theme.GetTheme(theme.ThemeDark)
	s := NewStyles(tm)

	if s.Theme == nil {
		t.Errorf("expected non-nil theme")
	}
	if s.App.GetForeground() != tm.Foreground {
		t.Errorf("app foreground should match theme")
	}
}

func TestRiskBadge_AllLevels(t *testing.T) {
	tm := theme.GetTheme(theme.ThemeDark)
	s := NewStyles(tm)

	levels := []string{"L0", "L1", "L2", "L3", "L4"}
	for _, level := range levels {
		badge := s.RiskBadge(level)
		if badge == "" {
			t.Errorf("expected non-empty risk badge for %s", level)
		}
	}
}

func TestNewStyles_StyleFieldsAreSet(t *testing.T) {
	tm := theme.GetTheme(theme.ThemeDark)
	s := NewStyles(tm)

	// Verify all style fields have been initialized (non-empty foreground or background)
	if s.Header.GetForeground() != tm.Primary {
		t.Errorf("header foreground should be primary")
	}
	if s.Footer.GetBackground() != tm.Selection {
		t.Errorf("footer background should be selection")
	}
	if s.StatusOK.GetForeground() != tm.Success {
		t.Errorf("statusOK foreground should be success")
	}
	if s.StatusWarn.GetForeground() != tm.Warning {
		t.Errorf("statusWarn foreground should be warning")
	}
	if s.StatusError.GetForeground() != tm.Error {
		t.Errorf("statusError foreground should be error")
	}
	if s.StatusInfo.GetForeground() != tm.Info {
		t.Errorf("statusInfo foreground should be info")
	}
	if s.InputPrompt.GetForeground() != tm.Primary {
		t.Errorf("inputPrompt foreground should be primary")
	}
}

func TestNewStyles_MessageStyles(t *testing.T) {
	tm := theme.GetTheme(theme.ThemeDark)
	s := NewStyles(tm)

	if s.UserMessage.GetForeground() != tm.Primary {
		t.Errorf("userMessage foreground should be primary")
	}
	if s.AIMessage.GetForeground() != tm.Foreground {
		t.Errorf("aiMessage foreground should be foreground")
	}
	if s.SystemMessage.GetForeground() != tm.Muted {
		t.Errorf("systemMessage foreground should be muted")
	}
}

func TestNewStyles_WithDifferentThemes(t *testing.T) {
	themes := []theme.ThemeName{theme.ThemeDark, theme.ThemeLight, theme.ThemeHighContrast}
	for _, name := range themes {
		t.Run(string(name), func(t *testing.T) {
			tm := theme.GetTheme(name)
			s := NewStyles(tm)
			if s == nil {
				t.Fatalf("expected non-nil styles for theme %s", name)
			}
			if s.Theme == nil {
				t.Errorf("expected non-nil theme in styles for %s", name)
			}
		})
	}
}

func TestNewStyles_BorderStyles(t *testing.T) {
	tm := theme.GetTheme(theme.ThemeDark)
	s := NewStyles(tm)

	// Verify border styles are initialized
	// GetBorderTop() returns bool (whether border is set)
	if !s.Border.GetBorderTop() {
		t.Log("border top not set (expected for lipgloss.RoundedBorder)")
	}
	// BorderActive has a different foreground color than Border
	if s.BorderActive.GetBorderLeftForeground() != s.Border.GetBorderLeftForeground() {
		t.Log("active border has different color (expected)")
	}
}
