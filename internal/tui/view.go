package tui

import (
	"fmt"
	"strings"

	"termtap.dev/internal/model"
)

// TODO: This is all temporary
func (m Model) View() string {
	return m.renderAppPane()

	// eventLines := m.renderEvents(8)
	// requestLines := m.renderRequests(12)
	//
	// return strings.Join([]string{
	// 	"termtap - live session",
	// 	fmt.Sprintf("events=%d requests=%d", len(m.events), len(m.requests)),
	// 	fmt.Sprintf("%dx%d", m.height, m.width),
	// 	"keys: q/esc/ctrl+c quit",
	// 	"",
	// 	"Recent events:",
	// 	eventLines,
	// 	"",
	// 	"Recent requests:",
	// 	requestLines,
	// }, "\n")
}

func (m Model) renderEvents(limit int) string {
	if len(m.events) == 0 {
		return "  (none yet)"
	}

	start := max(len(m.events)-limit, 0)

	rows := make([]string, 0, len(m.events)-start)
	for i := start; i < len(m.events); i++ {
		e := m.events[i]
		rows = append(rows, fmt.Sprintf("  [%s] %s", e.Type, truncate(e.Body, 100)))
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderRequests(limit int) string {
	if len(m.requests) == 0 {
		return "  (none yet)"
	}

	start := max(0, len(m.requests)-limit)

	// Traverse backwards since we don't have a stack
	rows := make([]string, 0, len(m.requests)-start)
	for i := len(m.requests) - 1; i >= start; i-- {
		req := m.requests[i]

		state := "done"
		if req.Pending {
			state = "pending"
		} else if req.Failed {
			state = "failed"
		}

		rows = append(rows, fmt.Sprintf(
			"  %s %s status=%d duration=%s state=%s",
			req.Method,
			requestPath(req),
			req.Status,
			req.Duration,
			state,
		))
	}

	if len(rows) == 0 {
		return "  (none yet)"
	}

	return strings.Join(rows, "\n")
}

func requestPath(req model.Request) string {
	if req.URL != "" {
		return truncate(req.URL, 80)
	}
	if req.RawURL != "" {
		return truncate(req.RawURL, 80)
	}
	if req.Host != "" {
		return truncate(req.Host, 80)
	}
	return "<unknown>"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	if max <= 3 {
		return s[:max]
	}

	return s[:max-3] + "..."
}
