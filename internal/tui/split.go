package tui

import "strings"

func (m Model) renderAppPane() string {
	var (
		searchW int = m.width
		searchH int = 1

		reqW int = int(float64(m.width) * 0.6)
		reqH int = m.height

		detW int = int(float64(m.width) * 0.4)
		detH int = m.height

		eventW int = m.width
		eventH int = int(float64(m.height) * 0.15)

		stdW int = m.width
		stdH int = int(float64(m.height) * 0.2)
	)

	if m.showSearch {
		reqH -= searchH
		detH -= searchH
	}

	if m.showEvents {
		reqH -= eventH
		detH -= eventH
	}

	if m.showStd {
		reqH -= stdH
		detH -= stdH
	}

	reqPane := m.renderRequestPane(reqW, reqH)
	detPane := m.renderDetailsPane(detW, detH)

	if len(reqPane) != len(detPane) {
		return "height of request and details did not match"
	}

	var screen []string
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

func (m Model) renderSearchPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(" ", w)
	}
	return lines
}

func (m Model) renderRequestPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(".", w)
	}
	return lines
}

func (m Model) renderDetailsPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat("^", w)
	}
	return lines
}

func (m Model) renderEventsPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat("~", w)
	}
	return lines
}

func (m Model) renderStdPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(" ", w)
	}
	return lines
}
