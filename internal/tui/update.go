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
		m.clampPaneScrolls()
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
		case "j", "down":
			m.moveFocusedVertical(1)
		case "k", "up":
			m.moveFocusedVertical(-1)
		case "tab":
			m.moveDetailsTab(1)
		case "shift+tab", "backtab":
			m.moveDetailsTab(-1)
		case "1":
			m.setFocusedPane(focusPaneRequests)
		case "2":
			m.setFocusedPane(focusPaneDetails)
		case "3":
			m.setFocusedPane(focusPaneEvents)
		case "4":
			m.setFocusedPane(focusPaneStd)
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
			m.ensureFocusedPaneVisible()
			m.clampPaneScrolls()
		case "o":
			m.showStd = !m.showStd
			m.ensureFocusedPaneVisible()
			m.clampPaneScrolls()
		case "/":
			m.showSearch = true
			m.clampPaneScrolls()
		case "esc":
			m.showSearch = false
			m.clampPaneScrolls()
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
		m.clampPaneScrolls()
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

	if len(m.requests) > 0 && m.requestCursor > 0 {
		m.requestCursor++
	}

	m.requests = append(m.requests, req)

	// If we passed the max, delete the first one
	// Maybe we should notify the user?
	if len(m.requests) > maxRequests {
		m.requests = m.requests[1:]
	}

	m.clampRequestCursor()
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

func (m *Model) moveRequestCursor(delta int) {
	if len(m.requests) == 0 {
		m.requestCursor = 0
		m.requestScroll = 0
		return
	}

	m.requestCursor += delta
	m.clampRequestCursor()
	m.ensureRequestCursorVisible()
	m.detailsScroll = 0
}

func (m *Model) clampRequestCursor() {
	if len(m.requests) == 0 {
		m.requestCursor = 0
		m.requestScroll = 0
		return
	}

	if m.requestCursor < 0 {
		m.requestCursor = 0
	}

	if m.requestCursor >= len(m.requests) {
		m.requestCursor = len(m.requests) - 1
	}
}

func (m *Model) ensureRequestCursorVisible() {
	viewHeight := m.requestBodyHeight()
	if viewHeight <= 0 {
		m.requestScroll = 0
		return
	}

	maxScroll := max(0, len(m.requests)-viewHeight)
	if m.requestScroll < 0 {
		m.requestScroll = 0
	}
	if m.requestScroll > maxScroll {
		m.requestScroll = maxScroll
	}

	if m.requestCursor < m.requestScroll {
		m.requestScroll = m.requestCursor
	}
	if m.requestCursor >= m.requestScroll+viewHeight {
		m.requestScroll = m.requestCursor - viewHeight + 1
	}

	if m.requestScroll < 0 {
		m.requestScroll = 0
	}
	if m.requestScroll > maxScroll {
		m.requestScroll = maxScroll
	}
}

func (m *Model) moveDetailsTab(delta int) {
	count := len(detailsTabNames)
	if count == 0 {
		m.detailsTab = 0
		m.detailsScroll = 0
		return
	}

	m.detailsTab = (m.detailsTab + delta) % count
	if m.detailsTab < 0 {
		m.detailsTab += count
	}
	m.detailsScroll = 0
	m.clampPaneScrolls()
}

func (m *Model) moveFocusedVertical(delta int) {
	switch m.focusedPane {
	case focusPaneRequests:
		m.moveRequestCursor(delta)
	case focusPaneDetails:
		m.detailsScroll += delta
	case focusPaneEvents:
		if delta > 0 {
			m.eventsScroll = max(0, m.eventsScroll-delta)
		} else {
			m.eventsScroll += -delta
		}
	case focusPaneStd:
		if delta > 0 {
			m.stdScroll = max(0, m.stdScroll-delta)
		} else {
			m.stdScroll += -delta
		}
	}

	m.clampPaneScrolls()
}

func (m *Model) setFocusedPane(pane int) {
	if !m.canFocusPane(pane) {
		return
	}
	m.focusedPane = pane
}

func (m *Model) canFocusPane(pane int) bool {
	switch pane {
	case focusPaneRequests, focusPaneDetails:
		return true
	case focusPaneEvents:
		return m.showEvents
	case focusPaneStd:
		return m.showStd
	default:
		return false
	}
}

func (m *Model) ensureFocusedPaneVisible() {
	if m.canFocusPane(m.focusedPane) {
		return
	}

	if m.canFocusPane(focusPaneDetails) {
		m.focusedPane = focusPaneDetails
		return
	}

	m.focusedPane = focusPaneRequests
}

func (m *Model) clampPaneScrolls() {
	if m.requestScroll < 0 {
		m.requestScroll = 0
	}
	if m.detailsScroll < 0 {
		m.detailsScroll = 0
	}
	if m.eventsScroll < 0 {
		m.eventsScroll = 0
	}
	if m.stdScroll < 0 {
		m.stdScroll = 0
	}

	m.ensureRequestCursorVisible()

	maxDetails := m.maxDetailsScroll()
	if m.detailsScroll > maxDetails {
		m.detailsScroll = maxDetails
	}

	maxEvents := m.maxEventsScroll()
	if m.eventsScroll > maxEvents {
		m.eventsScroll = maxEvents
	}

	maxStd := m.maxStdScroll()
	if m.stdScroll > maxStd {
		m.stdScroll = maxStd
	}
}

func (m Model) panelHeights() (requestHeight, detailsHeight, eventsHeight, stdHeight int) {
	const constHeightOffset = 1

	requestHeight = max(0, m.height-constHeightOffset)
	detailsHeight = max(0, m.height-constHeightOffset)
	eventsHeight = max(0, int(float64(m.height)*0.2))
	stdHeight = max(0, int(float64(m.height)*0.2))

	if m.showSearch {
		requestHeight = max(0, requestHeight-1)
		detailsHeight = max(0, detailsHeight-1)
	}

	if m.showEvents {
		requestHeight = max(0, requestHeight-eventsHeight)
		detailsHeight = max(0, detailsHeight-eventsHeight)
	}

	if m.showStd {
		requestHeight = max(0, requestHeight-stdHeight)
		detailsHeight = max(0, detailsHeight-stdHeight)
	}

	return requestHeight, detailsHeight, eventsHeight, stdHeight
}

func (m Model) requestBodyHeight() int {
	requestHeight, _, _, _ := m.panelHeights()
	return max(0, requestHeight-2)
}

func (m Model) detailsBodyHeight() int {
	_, detailsHeight, _, _ := m.panelHeights()
	return max(0, detailsHeight-2)
}

func (m Model) detailsPaneWidth() int {
	requestWidth := max(0, int(float64(m.width)*0.5))
	return max(0, m.width-requestWidth)
}

func (m Model) maxDetailsScroll() int {
	bodyHeight := m.detailsBodyHeight()
	if bodyHeight <= 0 {
		return 0
	}

	total := m.detailsContentLineCount(m.detailsPaneWidth())
	return max(0, total-bodyHeight)
}

func (m Model) maxEventsScroll() int {
	if !m.showEvents {
		return 0
	}

	_, _, eventsHeight, _ := m.panelHeights()
	bodyHeight := max(0, eventsHeight-1)
	if bodyHeight <= 0 {
		return 0
	}

	return max(0, m.eventCount()-bodyHeight)
}

func (m Model) maxStdScroll() int {
	if !m.showStd {
		return 0
	}

	_, _, _, stdHeight := m.panelHeights()
	bodyHeight := max(0, stdHeight-1)
	if bodyHeight <= 0 {
		return 0
	}

	return max(0, m.stdLogCount()-bodyHeight)
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
