package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termtap.dev/internal/model"
)

type EventMsg struct {
	value model.Event
}

type ErrMsg struct {
	err error
}

type TickMsg struct {
	Now time.Time
}

type RestartResultMsg struct {
	err error
}

const tick = 20 * time.Millisecond

func tickCmd() tea.Cmd {
	return tea.Tick(tick, func(t time.Time) tea.Msg {
		return TickMsg{Now: t}
	})
}

func restartCmd(restart func() error) tea.Cmd {
	if restart == nil {
		return nil
	}

	return func() tea.Msg {
		return RestartResultMsg{err: restart()}
	}
}
