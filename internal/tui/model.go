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

const (
	focusPaneRequests = iota
	focusPaneDetails
	focusPaneEvents
	focusPaneStd
)

type Model struct {
	channel  <-chan model.Event
	controls Controls

	events        []model.Event
	requests      []model.Request
	requestCursor int
	requestScroll int
	detailsTab    int
	detailsScroll int
	eventsScroll  int
	stdScroll     int
	focusedPane   int

	width  int
	height int

	theme       Theme
	showEvents  bool
	showStd     bool
	showSearch  bool
	searchQuery string
	restarting  bool

	now time.Time
}

type Controls struct {
	Restart func() error
}

func NewModel(ch <-chan model.Event, controls Controls) Model {
	return Model{
		channel:       ch,
		controls:      controls,
		events:        make([]model.Event, 0, maxEvents),
		requests:      make([]model.Request, 0, maxRequests),
		requestCursor: 0,
		requestScroll: 0,
		detailsTab:    detailsTabOverview,
		detailsScroll: 0,
		eventsScroll:  0,
		stdScroll:     0,
		focusedPane:   focusPaneRequests,
		width:         0,
		height:        0,
		showEvents:    false,
		showStd:       false,
		showSearch:    false,
		searchQuery:   "",
		restarting:    false,
		theme:         newTheme(),
	}
}

func Run(ch <-chan model.Event, controls Controls) error {
	p := tea.NewProgram(NewModel(ch, controls), tea.WithAltScreen())
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
