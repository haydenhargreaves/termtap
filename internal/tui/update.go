package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termtap.dev/internal/model"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TickMsg:
		m.now = msg.Now
		if m.hasPendingRequests() {
			return m, tickCmd()
		}
		return m, nil

	// TODO: Abstract the keymaps
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case tea.KeyCtrlR.String():
			if m.restarting {
				return m, nil
			}
			if m.controls.Restart == nil {
				return m, nil
			}
			m.restarting = true
			return m, restartCmd(m.controls.Restart)
		case "e":
			m.showEvents = !m.showEvents
		case "o":
			m.showStd = !m.showStd
		case "/":
			m.showSearch = true
		case "esc":
			m.showSearch = false
		}
		return m, nil

	case ErrMsg:
		m.pushEvent(model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeWarn,
			Body: fmt.Sprintf("tui event stream closed: %v", msg.err),
		})
		return m, nil

	case RestartResultMsg:
		m.restarting = false
		if msg.err != nil {
			m.pushEvent(model.Event{
				Time: time.Now().Local(),
				Type: model.EventTypeWarn,
				Body: fmt.Sprintf("failed to restart process: %v", msg.err),
			})
		}
		return m, nil

	case EventMsg:
		m.pushEvent(msg.value)
		m.applyMessage(msg.value)
		if m.hasPendingRequests() {
			return m, tea.Batch(waitForEvent(m.channel), tickCmd())
		}
		return m, waitForEvent(m.channel)
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
		m.createRequest(msg.Request)
	case model.EventTypeRequestFinished, model.EventTypeRequestFailed:
		m.updateRequest(msg.Request)
	}
}

func (m *Model) createRequest(req model.Request) {
	if req.Method == "CONNECT" {
		return
	}

	m.requests = append(m.requests, req)

	// If we passed the max, delete the first one
	// Maybe we should notify the user?
	if len(m.requests) > maxRequests {
		m.requests = m.requests[1:]
	}
}

func (m *Model) updateRequest(req model.Request) {
	// Traverse backward, since the newest one is at the end, and its likely we will be
	// updated a new request.
	for i := len(m.requests) - 1; i >= 0; i-- {
		if m.requests[i].ID == req.ID {
			m.requests[i] = req
			break
		}
	}
}

func (m Model) hasPendingRequests() bool {
	// Traverse backward to be a bit more efficient, the most recent requests are more
	// like to be pending.
	for i := len(m.requests) - 1; i >= 0; i-- {
		if m.requests[i].Pending {
			return true
		}
	}
	return false
}
