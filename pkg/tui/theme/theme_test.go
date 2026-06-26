package theme

import (
	"testing"
)

func TestGetThemeDark(t *testing.T) {
	th := GetTheme(ThemeDark)
	if th == nil {
		t.Fatal("Expected non-nil theme")
	}
	if th.Name != ThemeDark {
		t.Errorf("Expected theme name '%s', got '%s'", ThemeDark, th.Name)
	}
}

func TestGetThemeLight(t *testing.T) {
	th := GetTheme(ThemeLight)
	if th == nil {
		t.Fatal("Expected non-nil theme")
	}
	if th.Name != ThemeLight {
		t.Errorf("Expected theme name '%s', got '%s'", ThemeLight, th.Name)
	}
}

func TestGetThemeHighContrast(t *testing.T) {
	th := GetTheme(ThemeHighContrast)
	if th == nil {
		t.Fatal("Expected non-nil theme")
	}
	if th.Name != ThemeHighContrast {
		t.Errorf("Expected theme name '%s', got '%s'", ThemeHighContrast, th.Name)
	}
}

func TestGetThemeDefault(t *testing.T) {
	th := GetTheme("invalid")
	if th == nil {
		t.Fatal("Expected non-nil theme for invalid name")
	}
	if th.Name != ThemeDark {
		t.Errorf("Expected default theme to be dark, got '%s'", th.Name)
	}
}

func TestRiskColors(t *testing.T) {
	th := GetTheme(ThemeDark)

	riskLevels := []string{"L0", "L1", "L2", "L3", "L4"}
	for _, level := range riskLevels {
		color := th.GetRiskColor(level)
		if color == "" {
			t.Errorf("Expected non-empty color for risk level %s", level)
		}
	}

	defaultColor := th.GetRiskColor("INVALID")
	if defaultColor != th.Muted {
		t.Error("Expected default risk color to be Muted")
	}
}

func TestAvailableThemes(t *testing.T) {
	themes := AvailableThemes()
	if len(themes) != 3 {
		t.Errorf("Expected 3 available themes, got %d", len(themes))
	}
}
