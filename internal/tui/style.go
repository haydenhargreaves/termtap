package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type Theme struct {
	Background lipgloss.Style

	Header      lipgloss.Style
	EventHeader lipgloss.Style
	StdHeader   lipgloss.Style

	Text           lipgloss.Style
	TextMuted      lipgloss.Style
	TextError      lipgloss.Style
	TextMutedError lipgloss.Style

	EventGreen  lipgloss.Style
	EventRed    lipgloss.Style
	EventBlue   lipgloss.Style
	EventOrange lipgloss.Style
}

const background = lipgloss.Color("#010e1f")
const backgroundError = lipgloss.Color("#1f1118")
const text = lipgloss.Color("#dfe5ed")
const textMuted = lipgloss.Color("#7c7e80")

const blue = lipgloss.Color("#2280f2")
const orange = lipgloss.Color("#f2a813")
const red = lipgloss.Color("#e6130b")
const green = lipgloss.Color("#10e31e")

func newTheme() Theme {
	return Theme{
		Background: lipgloss.NewStyle().
			Background(background),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(background).
			Background(blue),
		EventHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(background).
			Background(blue),
		StdHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(background).
			Background(orange),

		Text: lipgloss.NewStyle().
			Foreground(text).
			Background(background),
		TextMuted: lipgloss.NewStyle().
			Foreground(textMuted).
			Background(background),
		TextError: lipgloss.NewStyle().
			Foreground(red).
			Background(backgroundError),
		TextMutedError: lipgloss.NewStyle().
			Foreground(textMuted).
			Background(backgroundError),

		EventGreen: lipgloss.NewStyle().
			Foreground(green).
			Background(background),
		EventBlue: lipgloss.NewStyle().
			Foreground(blue).
			Background(background),
		EventRed: lipgloss.NewStyle().
			Foreground(red).
			Background(backgroundError),
		EventOrange: lipgloss.NewStyle().
			Foreground(orange).
			Background(background),
	}
}

func clampRendered(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxCols {
		return s
	}
	return ansi.Truncate(s, maxCols, "...")
}
