package theme

import "github.com/charmbracelet/lipgloss"

type ThemeName string

const (
	ThemeDark      ThemeName = "dark"
	ThemeLight     ThemeName = "light"
	ThemeHighContrast ThemeName = "high-contrast"
)

type Theme struct {
	Name        ThemeName
	Foreground  lipgloss.Color
	Background  lipgloss.Color
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Accent      lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
	Info        lipgloss.Color
	Muted       lipgloss.Color
	Border      lipgloss.Color
	Selection   lipgloss.Color
	Cursor      lipgloss.Color

	RiskL0      lipgloss.Color
	RiskL1      lipgloss.Color
	RiskL2      lipgloss.Color
	RiskL3      lipgloss.Color
	RiskL4      lipgloss.Color
}

var (
	DarkTheme = Theme{
		Name:       ThemeDark,
		Foreground: lipgloss.Color("#E0E0E0"),
		Background: lipgloss.Color("#1A1B26"),
		Primary:    lipgloss.Color("#7AA2F7"),
		Secondary:  lipgloss.Color("#BB9AF7"),
		Accent:     lipgloss.Color("#7DCFFF"),
		Success:    lipgloss.Color("#9ECE6A"),
		Warning:    lipgloss.Color("#E0AF68"),
		Error:      lipgloss.Color("#F7768E"),
		Info:       lipgloss.Color("#7AA2F7"),
		Muted:      lipgloss.Color("#565F89"),
		Border:     lipgloss.Color("#414868"),
		Selection:  lipgloss.Color("#33467C"),
		Cursor:     lipgloss.Color("#C0CAF5"),

		RiskL0: lipgloss.Color("#9ECE6A"),
		RiskL1: lipgloss.Color("#7AA2F7"),
		RiskL2: lipgloss.Color("#E0AF68"),
		RiskL3: lipgloss.Color("#F7768E"),
		RiskL4: lipgloss.Color("#BB9AF7"),
	}

	LightTheme = Theme{
		Name:       ThemeLight,
		Foreground: lipgloss.Color("#343B58"),
		Background: lipgloss.Color("#FFFFFF"),
		Primary:    lipgloss.Color("#2E7DE0"),
		Secondary:  lipgloss.Color("#9854F6"),
		Accent:     lipgloss.Color("#00B4D8"),
		Success:    lipgloss.Color("#2CB67D"),
		Warning:    lipgloss.Color("#F59E0B"),
		Error:      lipgloss.Color("#EF4444"),
		Info:       lipgloss.Color("#2E7DE0"),
		Muted:      lipgloss.Color("#9CA3AF"),
		Border:     lipgloss.Color("#E5E7EB"),
		Selection:  lipgloss.Color("#DBEAFE"),
		Cursor:     lipgloss.Color("#1F2937"),

		RiskL0: lipgloss.Color("#2CB67D"),
		RiskL1: lipgloss.Color("#2E7DE0"),
		RiskL2: lipgloss.Color("#F59E0B"),
		RiskL3: lipgloss.Color("#EF4444"),
		RiskL4: lipgloss.Color("#9854F6"),
	}

	HighContrastTheme = Theme{
		Name:       ThemeHighContrast,
		Foreground: lipgloss.Color("#FFFFFF"),
		Background: lipgloss.Color("#000000"),
		Primary:    lipgloss.Color("#00FFFF"),
		Secondary:  lipgloss.Color("#FF00FF"),
		Accent:     lipgloss.Color("#FFFF00"),
		Success:    lipgloss.Color("#00FF00"),
		Warning:    lipgloss.Color("#FFFF00"),
		Error:      lipgloss.Color("#FF0000"),
		Info:       lipgloss.Color("#00FFFF"),
		Muted:      lipgloss.Color("#808080"),
		Border:     lipgloss.Color("#FFFFFF"),
		Selection:  lipgloss.Color("#0000FF"),
		Cursor:     lipgloss.Color("#FFFFFF"),

		RiskL0: lipgloss.Color("#00FF00"),
		RiskL1: lipgloss.Color("#00FFFF"),
		RiskL2: lipgloss.Color("#FFFF00"),
		RiskL3: lipgloss.Color("#FF0000"),
		RiskL4: lipgloss.Color("#FF00FF"),
	}
)

func GetTheme(name ThemeName) *Theme {
	switch name {
	case ThemeLight:
		return &LightTheme
	case ThemeHighContrast:
		return &HighContrastTheme
	default:
		return &DarkTheme
	}
}

func (t *Theme) GetRiskColor(level string) lipgloss.Color {
	switch level {
	case "L0":
		return t.RiskL0
	case "L1":
		return t.RiskL1
	case "L2":
		return t.RiskL2
	case "L3":
		return t.RiskL3
	case "L4":
		return t.RiskL4
	default:
		return t.Muted
	}
}

func AvailableThemes() []ThemeName {
	return []ThemeName{ThemeDark, ThemeLight, ThemeHighContrast}
}
