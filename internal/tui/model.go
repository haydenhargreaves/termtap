package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

const (
	maxEvents   = 200
	maxRequests = 200
)

type appMsg struct {
	value model.Message
}

type modelErrMsg struct {
	err error
}

type Model struct {
	msgCh <-chan model.Message

	events       []model.Message
	requestOrder []uuid.UUID
	requests     map[uuid.UUID]model.Request

	width  int
	height int
}

func NewModel(msgCh <-chan model.Message) Model {
	return Model{
		msgCh:        msgCh,
		events:       make([]model.Message, 0, maxEvents),
		requestOrder: make([]uuid.UUID, 0, maxRequests),
		requests:     map[uuid.UUID]model.Request{},
		width:        100,
		height:       28,
	}
}

func Run(msgCh <-chan model.Message) error {
	p := tea.NewProgram(NewModel(msgCh), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return waitForAppMessage(m.msgCh)
}

func waitForAppMessage(msgCh <-chan model.Message) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-msgCh
		if !ok {
			return modelErrMsg{err: fmt.Errorf("event channel closed")}
		}

		return appMsg{value: msg}
	}
}
