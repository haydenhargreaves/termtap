package tui

import "strings"

func (m Model) renderAppPane() string {
	// Constant height offset
	constHeightOffset := 1

	var (
		searchW int = max(0, m.width)
		searchH int = 1

		reqW int = max(0, int(float64(m.width)*0.55))
		reqH int = max(0, m.height-constHeightOffset)

		detW int = max(0, int(float64(m.width)*0.45))
		detH int = max(0, m.height-constHeightOffset)

		eventW int = max(0, m.width)
		eventH int = max(0, int(float64(m.height)*0.15))

		stdW int = max(0, m.width)
		stdH int = max(0, int(float64(m.height)*0.2))
	)

	if m.showSearch {
		reqH = max(0, reqH-searchH)
		detH = max(0, detH-searchH)
	}

	if m.showEvents {
		reqH = max(0, reqH-eventH)
		detH = max(0, detH-eventH)
	}

	if m.showStd {
		reqH = max(0, reqH-stdH)
		detH = max(0, detH-stdH)
	}

	reqPane := m.renderRequestPane(reqW, reqH)
	detPane := m.renderDetailsPane(detW, detH)

	if len(reqPane) != len(detPane) {
		return "height of request and details did not match"
	}

	var screen []string
	statusBar := m.renderStatusBar(m.width)
	screen = append(screen, statusBar)

	if m.showSearch {
		searchPane := m.renderSearchPane(searchW, searchH)
		screen = append(screen, searchPane...)
	}

	for i := range reqPane {
		screen = append(screen, reqPane[i]+detPane[i])
	}

	if m.showEvents {
		eventPane := m.renderEventsPane(eventW, eventH)
		screen = append(screen, eventPane...)
	}

	if m.showStd {
		stdPane := m.renderStdPane(stdW, stdH)
		screen = append(screen, stdPane...)
	}

	if len(screen) != m.height {
		return "height of screen does not match terminal height"
	}

	return strings.Join(screen, ("\n"))
}
