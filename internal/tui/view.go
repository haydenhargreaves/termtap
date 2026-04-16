package tui

import (
	"fmt"
	"time"
)

func (m Model) View() string {
	view := m.renderAppPane()
	if m.width <= 0 || m.height <= 0 {
		return view
	}

	return m.theme.Background.
		Width(m.width).
		Height(m.height).
		Render(view)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "PENDING"
	}

	if d >= 10*time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}

	if d >= time.Millisecond {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	return fmt.Sprintf("%dus", d.Microseconds())
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
