package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

const defaultThemeName = "nord"

type Theme struct {
	Title        string `toml:"title"`
	Subtitle     string `toml:"subtitle"`
	Hint         string `toml:"hint"`
	Error        string `toml:"error"`
	Success      string `toml:"success"`
	Selected     string `toml:"selected"`
	Accent       string `toml:"accent"`
	PanelBorder  string `toml:"panel_border"`
	SectionLabel string `toml:"section_label"`
}

type userThemesFile struct {
	Themes map[string]Theme `toml:"themes"`
}

func builtInThemes() map[string]Theme {
	return map[string]Theme{
		"nord": {
			Title:        "#88C0D0",
			Subtitle:     "#81A1C1",
			Hint:         "#4C566A",
			Error:        "#BF616A",
			Success:      "#A3BE8C",
			Selected:     "#ECEFF4",
			Accent:       "#5E81AC",
			PanelBorder:  "#434C5E",
			SectionLabel: "#8FBCBB",
		},
		"twilight-5": {
			Title:        "#FBBBAD",
			Subtitle:     "#EE8695",
			Hint:         "#4A7A96",
			Error:        "#EE8695",
			Success:      "#FBBBAD",
			Selected:     "#FBBBAD",
			Accent:       "#EE8695",
			PanelBorder:  "#333F58",
			SectionLabel: "#4A7A96",
		},
		"akc12": {
			Title:        "#F9D4B7",
			Subtitle:     "#D7B2A5",
			Hint:         "#7F6470",
			Error:        "#A3797B",
			Success:      "#D4B8B8",
			Selected:     "#FFEFDC",
			Accent:       "#A78682",
			PanelBorder:  "#4F4255",
			SectionLabel: "#D7B2A5",
		},
	}
}

func userThemesPath() string {
	return filepath.Join(filepath.Dir(configPath()), "themes.toml")
}

func loadAllThemes() (map[string]Theme, error) {
	themes := builtInThemes()

	path := userThemesPath()
	var data userThemesFile
	if _, err := toml.DecodeFile(path, &data); err != nil {
		if os.IsNotExist(err) {
			return themes, nil
		}
		return nil, fmt.Errorf("reading themes file %s: %w", path, err)
	}

	for name, t := range data.Themes {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		themes[name] = mergedTheme(themes[defaultThemeName], t)
	}
	return themes, nil
}

func mergedTheme(base, override Theme) Theme {
	if strings.TrimSpace(override.Title) != "" {
		base.Title = override.Title
	}
	if strings.TrimSpace(override.Subtitle) != "" {
		base.Subtitle = override.Subtitle
	}
	if strings.TrimSpace(override.Hint) != "" {
		base.Hint = override.Hint
	}
	if strings.TrimSpace(override.Error) != "" {
		base.Error = override.Error
	}
	if strings.TrimSpace(override.Success) != "" {
		base.Success = override.Success
	}
	if strings.TrimSpace(override.Selected) != "" {
		base.Selected = override.Selected
	}
	if strings.TrimSpace(override.Accent) != "" {
		base.Accent = override.Accent
	}
	if strings.TrimSpace(override.PanelBorder) != "" {
		base.PanelBorder = override.PanelBorder
	}
	if strings.TrimSpace(override.SectionLabel) != "" {
		base.SectionLabel = override.SectionLabel
	}
	return base
}

func themeNames(themes map[string]Theme) []string {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func applyThemeByName(name string) error {
	themes, err := loadAllThemes()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultThemeName
	}
	t, ok := themes[name]
	if !ok {
		return fmt.Errorf("theme %q not found", name)
	}
	applyTheme(t)
	return nil
}

func applyTheme(t Theme) {
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Title))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Subtitle))
	hintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Hint))
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Bold(true)
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success)).Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Selected))
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(t.PanelBorder)).Padding(0, 1)
	sectionLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.SectionLabel))
}

func ensureUserThemesFile() (string, error) {
	path := userThemesPath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	template := `# Add custom themes here.
# Any omitted key falls back to the default "nord" theme.

[themes.my-dark-theme]
title = "#88C0D0"
subtitle = "#81A1C1"
hint = "#4C566A"
error = "#BF616A"
success = "#A3BE8C"
selected = "#ECEFF4"
accent = "#5E81AC"
panel_border = "#434C5E"
section_label = "#8FBCBB"
`
	if err := os.WriteFile(path, []byte(template), 0600); err != nil {
		return "", err
	}
	return path, nil
}
