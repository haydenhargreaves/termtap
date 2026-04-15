package tui

import (
	"fmt"
	"strings"
)

func (m Model) renderStatusBar(w int) string {
	// TODO: Optimize somehow
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
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(".", w)
	}
	return lines
}

func (m Model) renderDetailsPane(w, h int) []string {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat("^", w)
	}
	return lines
}

func (m Model) renderEventsPane(w, h int) []string {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat("~", w)
	}
	return lines
}

func (m Model) renderStdPane(w, h int) []string {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}

	lines := make([]string, h)
	for y := range lines {
		lines[y] = strings.Repeat(" ", w)
	}
	return lines
}
