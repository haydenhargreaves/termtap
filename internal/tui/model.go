package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termtap.dev/internal/model"
)

// TODO: How big can we actually make this?
const (
	maxEvents   = 256
	maxRequests = 256
)

type Model struct {
	channel <-chan model.Event

	events   []model.Event
	requests []model.Request

	width  int
	height int

	theme      Theme
	showEvents bool
	showStd    bool
	showSearch bool

	now time.Time
}

func NewModel(ch <-chan model.Event) Model {
	return Model{
		channel:    ch,
		events:     make([]model.Event, 0, maxEvents),
		requests:   make([]model.Request, 0, maxRequests),
		width:      0,
		height:     0,
		showEvents: false,
		showStd:    false,
		showSearch: false,
		theme:      newTheme(),
	}
}

func Run(ch <-chan model.Event) error {
	p := tea.NewProgram(NewModel(ch), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForEvent(m.channel), tickCmd())
}

func waitForEvent(ch <-chan model.Event) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return ErrMsg{err: fmt.Errorf("event channel closed")}
		}

		return EventMsg{value: msg}
	}
}
