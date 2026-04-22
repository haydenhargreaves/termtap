package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"termtap.dev/internal/model"
)

type Theme struct {
	Background lipgloss.Style

	Header          lipgloss.Style
	EventHeader     lipgloss.Style
	EventPaneHeader lipgloss.Style
	StdHeader       lipgloss.Style

	Text            lipgloss.Style
	TextMuted       lipgloss.Style
	TextError       lipgloss.Style
	TextMutedError  lipgloss.Style
	RequestSelected lipgloss.Style
	HeaderKey       lipgloss.Style

	EventDefault         lipgloss.Style
	EventSession         lipgloss.Style
	EventProcess         lipgloss.Style
	EventProxy           lipgloss.Style
	EventRestart         lipgloss.Style
	EventRequestInFlight lipgloss.Style
	EventSuccess         lipgloss.Style
	EventWarn            lipgloss.Style
	EventError           lipgloss.Style
	EventFatal           lipgloss.Style
}

const background = lipgloss.Color("#010e1f")
const backgroundError = lipgloss.Color("#1f1118")
const text = lipgloss.Color("#dfe5ed")
const textMuted = lipgloss.Color("#7c7e80")

const blue = lipgloss.Color("#2280f2")
const cyan = lipgloss.Color("#22b8f2")
const violetBlue = lipgloss.Color("#6f7dff")
const orange = lipgloss.Color("#f2a813")
const red = lipgloss.Color("#e6130b")
const fatalRed = lipgloss.Color("#ff4d4d")
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
		EventPaneHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(background).
			Background(green),
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
		RequestSelected: lipgloss.NewStyle().
			Foreground(background).
			Background(blue).
			Bold(true),
		HeaderKey: lipgloss.NewStyle().
			Foreground(cyan).
			Background(background),

		EventDefault: lipgloss.NewStyle().
			Foreground(text).
			Background(background).
			Bold(true),
		EventSession: lipgloss.NewStyle().
			Foreground(textMuted).
			Background(background).
			Bold(true),
		EventProcess: lipgloss.NewStyle().
			Foreground(blue).
			Background(background).
			Bold(true),
		EventProxy: lipgloss.NewStyle().
			Foreground(violetBlue).
			Background(background).
			Bold(true),
		EventRestart: lipgloss.NewStyle().
			Foreground(cyan).
			Background(background).
			Bold(true),
		EventRequestInFlight: lipgloss.NewStyle().
			Foreground(cyan).
			Background(background).
			Bold(true),
		EventSuccess: lipgloss.NewStyle().
			Foreground(green).
			Background(background).
			Bold(true),
		EventWarn: lipgloss.NewStyle().
			Foreground(orange).
			Background(background).
			Bold(true),
		EventError: lipgloss.NewStyle().
			Foreground(red).
			Background(backgroundError).
			Bold(true),
		EventFatal: lipgloss.NewStyle().
			Foreground(fatalRed).
			Background(backgroundError).
			Bold(true),
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

func getEventColor(theme Theme, event model.EventType) lipgloss.Style {
	switch event {
	case model.EventTypeSessionStarted,
		model.EventTypeSessionStopped:
		return theme.EventSession

	case model.EventTypeProxyStarted,
		model.EventTypeProxyStarting,
		model.EventTypeProxyStopped:
		return theme.EventProxy

	case model.EventTypeRequestStarted:
		return theme.EventRequestInFlight

	case model.EventTypeProcessRestarting:
		return theme.EventRestart

	case model.EventTypeRequestFinished:
		return theme.EventSuccess

	case model.EventTypeFatal:
		return theme.EventFatal

	case model.EventTypeRequestFailed:
		return theme.EventError

	case model.EventTypeProcessStarting,
		model.EventTypeProcessStarted,
		model.EventTypeProcessExited,
		model.EventTypeProcessSignaled,
		model.EventTypeProcessStdout,
		model.EventTypeProcessStderr:
		return theme.EventProcess

	case model.EventTypeWarn:
		return theme.EventWarn

	default:
		return theme.EventDefault
	}
}
