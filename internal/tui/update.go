package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}
		return m, nil

	case ErrMsg:
		m.events = append(m.events, model.Event{
			Type: model.EventTypeWarn,
			Body: fmt.Sprintf("tui event stream closed: %v", msg.err),
		})
		return m, nil

	case EventMsg:
		m.pushEvent(msg.value)
		m.applyMessage(msg.value)
		return m, waitForAppMessage(m.msgCh)
	}

	return m, nil
}

func (m *Model) pushEvent(msg model.Event) {
	m.events = append(m.events, msg)
	if len(m.events) > maxEvents {
		m.events = m.events[len(m.events)-maxEvents:]
	}
}

func (m *Model) applyMessage(msg model.Event) {
	switch msg.Type {
	case model.EventTypeRequestStarted:
		m.upsertRequest(msg.Request, true)
	case model.EventTypeRequestFinished, model.EventTypeRequestFailed:
		m.upsertRequest(msg.Request, false)
	}
}

func (m *Model) upsertRequest(req model.Request, addIfMissing bool) {
	if req.ID == uuid.Nil {
		return
	}

	_, exists := m.requests[req.ID]
	if !exists && !addIfMissing {
		return
	}

	if !exists {
		m.requestOrder = append(m.requestOrder, req.ID)
		if len(m.requestOrder) > maxRequests {
			drop := m.requestOrder[0]
			delete(m.requests, drop)
			m.requestOrder = m.requestOrder[1:]
		}
	}

	m.requests[req.ID] = req
}
