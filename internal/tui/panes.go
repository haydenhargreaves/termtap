package tui

import (
	"fmt"
	"strings"
	"time"

	"termtap.dev/internal/model"
)

func (m Model) renderStatusBar(w int) string {
	var errCount int
	for _, req := range m.requests {
		if req.Failed || (req.Status >= 400 && req.Status < 600) {
			errCount++
		}
	}

	left := fmt.Sprintf(" tap %3d reqs  |  %d err  | avg 500ms", len(m.requests), errCount)
	right := "j/k nav  / search  tab panel  e events  o output  r replay  q quit "

	spaceSize := max(w-(len(left)+len(right)), 0)
	space := strings.Repeat(" ", spaceSize)

	return left + space + right
}

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

func (m Model) renderDetailsPane(w, h int) []string {
	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat("^", w)
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

	lines := []string{
		fmt.Sprintf("EVENT LOG - %d EVENTS", len(events)),
	}

	for _, event := range events {
		line := fmt.Sprintf(
			"%s %-15s %s",
			event.Time.Format("15:04:05"),
			event.Type,
			event.Body,
		)
		if event.PID > 0 {
			line = fmt.Sprintf(
				"%s %-15s %d %s",
				event.Time.Format("15:04:05"),
				event.Type,
				event.PID,
				event.Body,
			)
		}
		lines = append(lines, truncate(line, w))
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

	lines := []string{
		fmt.Sprintf("STDOUT/STDERR LOG - %d LINES", len(logs)),
	}

	for _, log := range logs {
		var t string
		if log.Type == model.EventTypeProcessStderr {
			t = "STDERR"
		}
		if log.Type == model.EventTypeProcessStdout {
			t = "STDOUT"
		}
		line := fmt.Sprintf(
			"%s %6s %s",
			log.Time.Format("15:04:05"),
			t,
			log.Body,
		)
		lines = append(lines, truncate(line, w))
	}

	// Cleanup
	if len(lines) < h {
		for i := len(lines); i < h; i++ {
			lines = append(lines, "")
		}
	}

	return lines
}
