package tui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"termtap.dev/internal/model"
)

// TODO: LOTS OF THIS SUCKS BUT IT WORKS

const (
	detailsTabOverview = iota
	detailsTabRequest
	detailsTabResponse
	detailsTabHeaders
)

var detailsTabNames = []string{"Overview", "Request", "Response", "Headers"}

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
	logState := "logs off"
	if m.showEvents && m.showStd {
		logState = "events+output"
	} else if m.showEvents {
		logState = "events"
	} else if m.showStd {
		logState = "output"
	}
	right := " " + logState + " "

	spaceSize := max(w-(len(left)+len(right)), 0)
	space := strings.Repeat(" ", spaceSize)

	return m.theme.Header.Render(left + space + right)
}

func (m Model) renderBottomStatusBar(w int) string {
	if w <= 0 {
		return ""
	}

	modeLabel := "REQ"
	modeColor := blue
	switch m.focusedPane {
	case focusPaneDetails:
		modeLabel = "DETAIL"
		modeColor = blue
	case focusPaneEvents:
		modeLabel = "EVENT"
		modeColor = green
	case focusPaneStd:
		modeLabel = "OUT"
		modeColor = orange
	}

	modeStyle := lipgloss.NewStyle().Foreground(background).Background(modeColor).Bold(true)
	left := modeStyle.Render(" " + modeLabel + " ")
	if m.restarting {
		left += m.theme.Text.Render(" ") + m.theme.EventWarn.Render(" RESTARTING ")
	}

	right := m.theme.TextMuted.Render(" " + m.bottomStatusRight() + " ")
	if lipgloss.Width(right) >= w {
		return clampRendered(right, w)
	}

	maxLeft := max(0, w-lipgloss.Width(right)-1)
	left = clampRendered(left, maxLeft)
	spaceSize := max(1, w-lipgloss.Width(left)-lipgloss.Width(right))
	space := m.theme.Text.Render(strings.Repeat(" ", spaceSize))

	return clampRendered(left+space+right, w)
}

func (m Model) bottomStatusRight() string {
	selected, total := m.requestSelectionStats()

	switch m.focusedPane {
	case focusPaneRequests:
		return fmt.Sprintf("req %d/%d", selected, total)

	case focusPaneDetails:
		tab := "overview"
		if m.detailsTab >= 0 && m.detailsTab < len(detailsTabNames) {
			tab = strings.ToLower(detailsTabNames[m.detailsTab])
		}
		linesTotal := m.detailsContentLineCount(m.detailsPaneWidth())
		linesPos := 0
		if linesTotal > 0 {
			linesPos = min(linesTotal, m.detailsScroll+1)
		}
		return fmt.Sprintf("req %d/%d | tab %s | %d/%d", selected, total, tab, linesPos, linesTotal)

	case focusPaneEvents:
		count := m.eventCount()
		if m.eventsScroll == 0 {
			return fmt.Sprintf("events %d | LIVE", count)
		}
		return fmt.Sprintf("events %d | PAUSED", count)

	case focusPaneStd:
		count := m.stdLogCount()
		if m.stdScroll == 0 {
			return fmt.Sprintf("lines %d | LIVE", count)
		}
		return fmt.Sprintf("lines %d | PAUSED", count)
	}

	return ""
}

func (m Model) requestSelectionStats() (selected int, total int) {
	total = m.visibleRequestCount()
	if total == 0 {
		return 0, 0
	}

	selected = min(total, max(1, m.requestCursor+1))
	return selected, total
}

func (m Model) renderSearchPane(w, h int) []string {
	if h <= 0 {
		return nil
	}

	left := m.theme.TextMuted.Render(" / SEARCH ")
	hint := m.theme.TextMuted.Render(" host/path method:get status:5xx ")
	if strings.TrimSpace(m.searchQuery) != "" {
		hint = m.theme.Text.Render(" " + m.searchQuery + " ")
	}

	line := left + hint
	line = clampRendered(line, w)
	if lipgloss.Width(line) < w {
		line += m.theme.Text.Render(strings.Repeat(" ", w-lipgloss.Width(line)))
	}

	lines := make([]string, h)
	lines[0] = line
	for y := 1; y < h; y++ {
		lines[y] = m.theme.Text.Render(strings.Repeat(" ", w))
	}
	return lines
}

func (m Model) renderRequestPane(w, h int) []string {
	var lines []string

	titleStyle := m.theme.TextMuted
	if m.focusedPane == focusPaneRequests {
		titleStyle = m.theme.EventHeader
	}
	title := titleStyle.Render(padToWidth("[1] REQUESTS", w))
	lines = append(lines, title)

	// Render header
	headerLeft := fmt.Sprintf(" %-7s %-24s %s", "METHOD", "HOST", "PATH")
	headerRight := fmt.Sprintf("%4s %8s ", "CODE", "TIME")
	headerSpace := strings.Repeat(" ", max(0, w-len(headerLeft+headerRight)))
	header := m.theme.TextMuted.Render(headerLeft + headerSpace + headerRight)
	lines = append(lines, header)

	visible := m.filteredRequestIndices()
	bodyLines := make([]string, 0, len(visible))
	for i, row := len(visible)-1, 0; i >= 0; i, row = i-1, row+1 {
		req := m.requests[visible[i]]
		duration := req.Duration
		if req.Pending && !req.StartTime.IsZero() {
			duration = time.Since(req.StartTime)
		}

		statusStyle := lipgloss.NewStyle().Foreground(green).Background(background).Bold(true)
		if req.Failed || req.Status >= 500 {
			statusStyle = lipgloss.NewStyle().Foreground(red).Background(background).Bold(true)
		} else if req.Status >= 400 {
			statusStyle = lipgloss.NewStyle().Foreground(orange).Background(background).Bold(true)
		}

		latencyStyle := m.theme.Text
		if duration >= 2*time.Second {
			latencyStyle = lipgloss.NewStyle().Foreground(orange).Background(background).Bold(true)
		}

		methodCol := statusStyle.Render(fmt.Sprintf("%-7s", truncate(strings.ToUpper(req.Method), 7)))
		hostCol := m.theme.Text.Render(fmt.Sprintf("%-24s", truncate(req.Host, 24)))
		pathCol := m.theme.Text.Render(req.URL)

		statusText := ""
		if !req.Pending && req.Status > 0 {
			statusText = fmt.Sprintf("%d", req.Status)
		}
		statusCol := statusStyle.Render(fmt.Sprintf("%4s", statusText))
		timeCol := latencyStyle.Render(fmt.Sprintf("%8s", formatDuration(duration)))

		sep := m.theme.Text.Render(" ")
		left := sep + methodCol + sep + hostCol + sep + pathCol
		right := statusCol + sep + timeCol + sep
		space := strings.Repeat(" ", max(0, w-lipgloss.Width(left+right)))

		line := left + m.theme.Text.Render(space) + right
		line = clampRendered(line, w)
		if row == m.requestCursor {
			line = m.theme.RequestSelected.Render(ansi.Strip(line))
		}
		bodyLines = append(bodyLines, line)
	}

	bodyHeight := max(0, h-len(lines))
	scroll := m.requestScroll
	maxScroll := max(0, len(bodyLines)-bodyHeight)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	if bodyHeight > 0 {
		end := min(len(bodyLines), scroll+bodyHeight)
		if scroll < end {
			lines = append(lines, bodyLines[scroll:end]...)
		}
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
	if h <= 0 {
		return nil
	}

	formatLine := func(content string) string {
		line := truncate(content, w)
		if len(line) < w {
			line += strings.Repeat(" ", w-len(line))
		}
		return m.theme.Text.Render(line)
	}

	formatMutedLine := func(content string) string {
		line := truncate(content, w)
		if len(line) < w {
			line += strings.Repeat(" ", w-len(line))
		}
		return m.theme.TextMuted.Render(line)
	}

	formatMutedItalicLine := func(content string) string {
		line := truncate(content, w)
		if len(line) < w {
			line += strings.Repeat(" ", w-len(line))
		}
		return lipgloss.NewStyle().
			Foreground(textMuted).
			Background(background).
			Italic(true).
			Render(line)
	}

	renderTabRow := func() string {
		if w <= 0 {
			return ""
		}

		activeTabStyle := lipgloss.NewStyle().
			Foreground(background).
			Background(blue).
			Bold(true)

		var parts []string
		for i, name := range detailsTabNames {
			label := " " + name + " "
			if i == m.detailsTab {
				label = " [" + name + "] "
				parts = append(parts, activeTabStyle.Render(label))
				continue
			}
			parts = append(parts, m.theme.TextMuted.Render(label))
		}

		sep := m.theme.Text.Render(" ")
		line := strings.Join(parts, sep)
		if lipgloss.Width(line) < w {
			pad := strings.Repeat(" ", w-lipgloss.Width(line))
			line += m.theme.Text.Render(pad)
		}

		return clampRendered(line, w)
	}

	lines := make([]string, 0, h)
	detailsTitleStyle := m.theme.TextMuted
	if m.focusedPane == focusPaneDetails {
		detailsTitleStyle = m.theme.EventHeader
	}
	lines = append(lines, detailsTitleStyle.Render(padToWidth("[2] DETAIL", w)))
	if len(lines) >= h {
		return lines[:h]
	}
	lines = append(lines, renderTabRow())
	if len(lines) >= h {
		return lines[:h]
	}

	contentLines := m.detailsContentLines(w, formatLine, formatMutedLine, formatMutedItalicLine)
	contentHeight := max(0, h-len(lines))
	scroll := m.detailsScroll
	maxScroll := max(0, len(contentLines)-contentHeight)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	if contentHeight > 0 {
		end := min(len(contentLines), scroll+contentHeight)
		if scroll < end {
			lines = append(lines, contentLines[scroll:end]...)
		}
	}

	for len(lines) < h {
		lines = append(lines, formatLine(""))
	}

	if len(lines) > h {
		return lines[:h]
	}

	return lines
}

func padToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}

	s = truncate(s, w)
	if len(s) < w {
		s += strings.Repeat(" ", w-len(s))
	}

	return s
}

func (m Model) detailsContentLines(
	w int,
	formatLine func(string) string,
	formatMutedLine func(string) string,
	formatMutedItalicLine func(string) string,
) []string {
	selectedReq, ok := m.selectedRequest()
	if !ok {
		return []string{formatLine(" No requests yet. Use j/k once requests arrive.")}
	}

	contentLines := make([]string, 0, 64)

	switch m.detailsTab {
	case detailsTabOverview:
		duration := selectedReq.Duration
		if selectedReq.Pending && !selectedReq.StartTime.IsZero() {
			duration = time.Since(selectedReq.StartTime)
		}

		renderKVLine := func(key, value string, valueStyle lipgloss.Style) string {
			keyPart := m.theme.TextMuted.Render(fmt.Sprintf(" %-8s", key))
			valuePart := valueStyle.Render(value)
			sep := m.theme.Text.Render(" ")
			line := keyPart + sep + valuePart
			if lipgloss.Width(line) < w {
				line += m.theme.Text.Render(strings.Repeat(" ", w-lipgloss.Width(line)))
			}
			return clampRendered(line, w)
		}

		statusText := "-"
		statusStyle := m.theme.TextMuted
		if selectedReq.Status > 0 {
			statusText = fmt.Sprintf("%d", selectedReq.Status)
			statusStyle = m.theme.EventSuccess
			if selectedReq.Status >= 400 {
				statusStyle = m.theme.EventWarn
			}
			if selectedReq.Status >= 500 {
				statusStyle = m.theme.EventError
			}
		}

		timeText := "-"
		if !selectedReq.StartTime.IsZero() {
			timeText = selectedReq.StartTime.Format("3:04:05 PM")
		}

		contentLines = append(contentLines, formatLine(""))
		contentLines = append(contentLines, renderKVLine("Method", strings.ToUpper(selectedReq.Method), m.theme.Text))
		contentLines = append(contentLines, renderKVLine("URL", selectedReq.RawURL, m.theme.TextMuted))
		queryText := selectedReq.QueryString
		if queryText == "" {
			queryText = "-"
		}
		contentLines = append(contentLines, renderKVLine("Query", queryText, m.theme.TextMuted))
		contentLines = append(contentLines, renderKVLine("Status", statusText, statusStyle))
		contentLines = append(contentLines, renderKVLine("Latency", formatDuration(duration), m.theme.Text))
		contentLines = append(contentLines, renderKVLine("Time", timeText, m.theme.Text))

		contentLines = append(contentLines, formatLine(""))
		contentLines = append(contentLines, formatMutedLine(" Timing"))

		barValue := formatDuration(duration)
		barPrefix := " "
		barSuffix := " " + barValue
		maxBarWidth := max(1, w/2)
		barWidth := min(maxBarWidth, max(0, w-len(barPrefix)-len(barSuffix)))
		if barWidth == 0 {
			contentLines = append(contentLines, formatLine(" "+barValue))
			break
		}

		const maxDurationForScale = 2 * time.Second
		ratio := float64(duration) / float64(maxDurationForScale)
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}

		filled := int(ratio * float64(barWidth))
		if duration > 0 && filled == 0 {
			filled = 1
		}
		if filled > barWidth {
			filled = barWidth
		}
		empty := max(0, barWidth-filled)

		filledPart := lipgloss.NewStyle().Foreground(blue).Render(strings.Repeat("█", filled))
		emptyPart := lipgloss.NewStyle().Foreground(cyan).Render(strings.Repeat("░", empty))
		barLine := barPrefix + filledPart + emptyPart + m.theme.TextMuted.Render(barSuffix)
		contentLines = append(contentLines, clampRendered(barLine, w))

	case detailsTabRequest:
		contentLines = append(contentLines, formatLine(""))
		contentLines = append(contentLines, formatMutedLine(" -- Request Body --"))
		contentLines = append(contentLines, formatLine(""))
		if len(selectedReq.RequestData) == 0 {
			contentLines = append(contentLines, formatMutedItalicLine(" empty"))
			break
		}
		for _, line := range formatBodyLines(prettyBody(selectedReq.RequestData, selectedReq.RequestHeaders), w) {
			contentLines = append(contentLines, formatLine(" "+line))
		}

	case detailsTabResponse:
		contentLines = append(contentLines, formatLine(""))
		contentLines = append(contentLines, formatMutedLine(" -- Response Body --"))
		contentLines = append(contentLines, formatLine(""))
		if len(selectedReq.ResponseData) == 0 {
			contentLines = append(contentLines, formatMutedItalicLine(" empty"))
			break
		}
		for _, line := range formatBodyLines(prettyBody(selectedReq.ResponseData, selectedReq.ResponseHeaders), w) {
			contentLines = append(contentLines, formatLine(" "+line))
		}

	case detailsTabHeaders:
		contentLines = append(contentLines, formatLine(""))
		renderHeaderLine := func(key, value string) string {
			left := m.theme.HeaderKey.Render(" " + key + ": ")
			right := m.theme.TextMuted.Render(value)
			line := left + right
			if lipgloss.Width(line) < w {
				line += m.theme.Text.Render(strings.Repeat(" ", w-lipgloss.Width(line)))
			}
			return clampRendered(line, w)
		}

		appendHeaders := func(title string, headers map[string][]string) {
			contentLines = append(contentLines, formatMutedLine(" -- "+title+" --"))
			contentLines = append(contentLines, formatLine(""))

			if len(headers) == 0 {
				contentLines = append(contentLines, formatMutedItalicLine(" empty"))
				contentLines = append(contentLines, formatLine(""))
				return
			}

			keys := make([]string, 0, len(headers))
			for key := range headers {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				contentLines = append(contentLines, renderHeaderLine(key, strings.Join(headers[key], ", ")))
			}

			contentLines = append(contentLines, formatLine(""))
		}

		appendHeaders("Request Headers", selectedReq.RequestHeaders)
		appendHeaders("Response Headers", selectedReq.ResponseHeaders)

	default:
		contentLines = append(contentLines, formatLine(" Unknown details tab"))
	}

	return contentLines
}

func (m Model) detailsContentLineCount(w int) int {
	plain := func(s string) string { return s }
	return len(m.detailsContentLines(w, plain, plain, plain))
}

func (m Model) selectedRequest() (model.Request, bool) {
	visible := m.filteredRequestIndices()
	if len(visible) == 0 {
		return model.Request{}, false
	}

	cursor := m.requestCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(visible) {
		cursor = len(visible) - 1
	}

	idx := len(visible) - cursor - 1
	if idx < 0 || idx >= len(visible) {
		return model.Request{}, false
	}

	return m.requests[visible[idx]], true
}

func formatBodyLines(body []byte, width int) []string {
	if len(body) == 0 {
		return []string{"(empty)"}
	}

	text := string(body)
	if !utf8.Valid(body) {
		previewSize := min(len(body), 64)
		hexPreview := hex.EncodeToString(body[:previewSize])
		suffix := ""
		if previewSize < len(body) {
			suffix = "..."
		}
		text = fmt.Sprintf("(binary payload, %d bytes, hex preview: %s%s)", len(body), hexPreview, suffix)
	}

	src := strings.ReplaceAll(text, "\t", "    ")
	rawLines := strings.Split(src, "\n")
	if width <= 4 {
		return rawLines
	}

	maxWidth := max(1, width-2)
	out := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if line == "" {
			out = append(out, "")
			continue
		}

		for len(line) > maxWidth {
			out = append(out, line[:maxWidth])
			line = line[maxWidth:]
		}
		out = append(out, line)
	}

	return out
}

func prettyBody(body []byte, headers map[string][]string) []byte {
	if len(body) == 0 {
		return body
	}

	if !looksLikeJSON(body, headers) {
		return body
	}

	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return body
	}

	return out.Bytes()
}

func looksLikeJSON(body []byte, headers map[string][]string) bool {
	if json.Valid(body) {
		return true
	}

	for key, values := range headers {
		if !strings.EqualFold(key, "Content-Type") {
			continue
		}

		for _, value := range values {
			contentType := strings.ToLower(value)
			if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json") {
				return true
			}
		}
	}

	return false
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

	left := fmt.Sprintf("[3] EVENT LOG - %d EVENTS", len(events))
	right := "E: TOGGLE"
	headerStyle := m.theme.TextMuted
	if m.focusedPane == focusPaneEvents {
		headerStyle = m.theme.EventPaneHeader
	}
	status := headerStyle.Render(left + strings.Repeat(" ", max(0, w-len(left+right))) + right)
	lines := []string{status}

	bodyHeight := max(0, h-1)
	maxScroll := max(0, len(events)-bodyHeight)
	scroll := m.eventsScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	start := max(0, len(events)-bodyHeight-scroll)
	end := min(len(events), start+bodyHeight)
	visible := events[start:end]

	for _, event := range visible {
		var (
			eTime string = m.theme.TextMuted.Render(event.Time.Format("15:04:05") + " ")
			eType string = getEventColor(m.theme, event.Type).Render(fmt.Sprintf("%-17s ", event.Type))

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

func (m Model) eventCount() int {
	count := 0
	for _, ev := range m.events {
		if ev.Type != model.EventTypeProcessStderr && ev.Type != model.EventTypeProcessStdout {
			count++
		}
	}
	return count
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

	left := fmt.Sprintf("[4] STDOUT/STDERR LOG - %d LINES", len(logs))
	right := "O: TOGGLE"
	headerStyle := m.theme.TextMuted
	if m.focusedPane == focusPaneStd {
		headerStyle = m.theme.StdHeader
	}
	status := headerStyle.Render(left + strings.Repeat(" ", max(0, w-len(left+right))) + right)
	lines := []string{status}

	bodyHeight := max(0, h-1)
	maxScroll := max(0, len(logs)-bodyHeight)
	scroll := m.stdScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}

	start := max(0, len(logs)-bodyHeight-scroll)
	end := min(len(logs), start+bodyHeight)
	visible := logs[start:end]

	for _, log := range visible {
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

func (m Model) stdLogCount() int {
	count := 0
	for _, ev := range m.events {
		if ev.Type == model.EventTypeProcessStderr || ev.Type == model.EventTypeProcessStdout {
			count++
		}
	}
	return count
}
