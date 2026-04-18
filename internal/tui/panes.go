package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"termtap.dev/internal/model"
)

// TODO: LOTS OF THIS SUCKS BUT IT WORKS

func (m Model) renderStatusBar(w int) string {
	var errCount int
	var msSum int64
	for _, req := range m.requests {
		if req.Failed || (req.Status >= 400 && req.Status < 600) {
			errCount++
		}
		msSum += req.Duration.Milliseconds()
	}

	avg := int(msSum) / max(1, len(m.requests))
	left := fmt.Sprintf(" tap %3d reqs  |  %d err  | avg %dms", len(m.requests), errCount, avg)
	right := "j/k nav  / search  tab panel  e events  o output  r replay  ctrl+r restart  q quit "

	spaceSize := max(w-(len(left)+len(right)), 0)
	space := strings.Repeat(" ", spaceSize)

	return m.theme.Header.Render(left + space + right)
}

// TODO: Implement
func (m Model) renderSearchPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(" ", w)
	}
	return lines
}

func (m Model) renderRequestPane(w, h int) []string {
	var lines []string

	// Render header
	headerLeft := fmt.Sprintf(" %-7s %-24s %s", "METHOD", "HOST", "PATH")
	headerRight := fmt.Sprintf("%4s %8s ", "CODE", "TIME")
	headerSpace := strings.Repeat(" ", max(0, w-len(headerLeft+headerRight)))
	header := headerLeft + headerSpace + headerRight
	lines = append(lines, header)

	for i := len(m.requests) - 1; i >= 0; i-- {
		req := m.requests[i]

		// Some formatting magic here maybe
		left := fmt.Sprintf(
			" %-7s %-24s %s",
			strings.ToUpper(req.Method),
			req.Host,
			req.URL,
		)
		right := fmt.Sprintf(
			"%4d %8s ",
			req.Status,
			formatDuration(req.Duration),
		)
		if req.Pending && !req.StartTime.IsZero() {
			right = fmt.Sprintf(
				"%4s %8s ",
				"",
				formatDuration(time.Since(req.StartTime)),
			)
		}
		space := strings.Repeat(" ", max(0, w-len(left+right)))

		line := left + space + right
		lines = append(lines, line)
	}

	// Cleanup
	if len(lines) < h {
		for i := len(lines); i < h; i++ {
			lines = append(lines, strings.Repeat(" ", w))
		}
	}

	if len(lines) > h {
		lines = lines[:h]
	}

	return lines
}

// TODO: Implement
func (m Model) renderDetailsPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = m.theme.Text.Render(strings.Repeat(" ", w))
	}
	return lines
}

// TODO: This can be done better
// TODO: Should h be max or defined?
func (m Model) renderEventsPane(w, h int) []string {
	// Remove the stdout or stderr logs
	var events []model.Event
	for _, ev := range m.events {
		if ev.Type != model.EventTypeProcessStderr &&
			ev.Type != model.EventTypeProcessStdout {
			events = append(events, ev)
		}
	}

	displayCount := max(h-1, 0)

	if displayCount < len(events) {
		events = events[len(events)-displayCount:]
	}

	left := fmt.Sprintf("EVENT LOG - %d EVENTS", len(events))
	right := "E: TOGGLE"
	status := m.theme.EventHeader.Render(left + strings.Repeat(" ", w-len(left+right)) + right)
	lines := []string{status}

	for _, event := range events {
		var (
			eTime string = m.theme.TextMuted.Render(event.Time.Format("15:04:05") + " ")
			eType string = getEventColor(m.theme, event.Type).Render(fmt.Sprintf("%-15s ", event.Type))

			avail int    = max(0, w-lipgloss.Width(eTime+eType))
			body  string = clampRendered(m.theme.Text.Render(event.Body), avail)
		)

		if event.Type == model.EventTypeRequestFailed || event.Type == model.EventTypeFatal {
			body = clampRendered(m.theme.TextError.Render(event.Body), avail)
			eTime = m.theme.TextMutedError.Render(event.Time.Format("15:04:05") + " ")
		}

		line := eTime + eType + body
		if event.PID > 0 {
			pid := m.theme.TextMuted.Render(fmt.Sprintf("%d ", event.PID))

			avail = max(0, w-lipgloss.Width(eTime+eType+pid))
			body = clampRendered(m.theme.Text.Render(event.Body), avail)
			line = eTime + eType + pid + body
		}

		if event.Type == model.EventTypeRequestFailed || event.Type == model.EventTypeFatal {
			line += m.theme.TextError.Render(strings.Repeat(" ", w-lipgloss.Width(line)))
		}

		lines = append(lines, line)
	}

	// Cleanup
	if len(lines) < h {
		for i := len(lines); i < h; i++ {
			lines = append(lines, "")
		}
	}

	return lines
}

// TODO: Should h be max or defined?
func (m Model) renderStdPane(w, h int) []string {
	// Only the stdout or stderr logs
	var logs []model.Event
	for _, ev := range m.events {
		if ev.Type == model.EventTypeProcessStderr ||
			ev.Type == model.EventTypeProcessStdout {
			logs = append(logs, ev)
		}
	}

	displayCount := max(h-1, 0)

	if displayCount < len(logs) {
		logs = logs[len(logs)-displayCount:]
	}

	left := fmt.Sprintf("STDOUT/STDERR LOG - %d LINES", len(logs))
	right := "O: TOGGLE"
	status := m.theme.StdHeader.Render(left + strings.Repeat(" ", w-len(left+right)) + right)
	lines := []string{status}

	for _, log := range logs {
		var (
			tag      string
			body     string
			timePart string
		)
		if log.Type == model.EventTypeProcessStderr {
			tag = m.theme.TextError.Render("ERR ")
			timePart = m.theme.TextMutedError.Render(log.Time.Format("15:04:05") + " ")

			prefix := timePart + tag
			avail := max(0, w-lipgloss.Width(prefix))
			body = clampRendered(m.theme.TextError.Render(log.Body), avail)

			pad := max(0, avail-lipgloss.Width(body))
			body += m.theme.TextMutedError.Render(strings.Repeat(" ", pad))
		}
		if log.Type == model.EventTypeProcessStdout {
			tag = m.theme.TextMuted.Render("OUT ")
			timePart = m.theme.TextMuted.Render(log.Time.Format("15:04:05") + " ")

			prefix := timePart + tag
			avail := max(0, w-lipgloss.Width(prefix))
			body = clampRendered(m.theme.Text.Render(log.Body), avail)
		}
		line := clampRendered(timePart+tag+body, w)
		lines = append(lines, line)
	}

	// Cleanup
	if len(lines) < h {
		for i := len(lines); i < h; i++ {
			lines = append(lines, "")
		}
	}

	return lines
}
