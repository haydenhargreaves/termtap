package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "pending zero", in: 0, want: "PENDING"},
		{name: "microseconds", in: 750 * time.Microsecond, want: "750us"},
		{name: "milliseconds", in: 20 * time.Millisecond, want: "20ms"},
		{name: "seconds", in: 11 * time.Second, want: "11.00s"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatDuration(tt.in); got != tt.want {
				t.Fatalf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{name: "short unchanged", s: "abc", max: 3, want: "abc"},
		{name: "max small no ellipsis", s: "abcdef", max: 3, want: "abc"},
		{name: "ellipsis", s: "abcdef", max: 5, want: "ab..."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := truncate(tt.s, tt.max); got != tt.want {
				t.Fatalf("truncate(%q,%d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestClampRendered(t *testing.T) {
	t.Parallel()

	if got := clampRendered("abcdef", 0); got != "" {
		t.Fatalf("clampRendered max=0 = %q, want empty", got)
	}
	if got := clampRendered("abc", 10); got != "abc" {
		t.Fatalf("clampRendered no truncation = %q, want %q", got, "abc")
	}
	if got := clampRendered("abcdef", 4); !strings.Contains(got, "...") {
		t.Fatalf("clampRendered truncation should include ellipsis, got %q", got)
	}
}

func TestGetEventColor(t *testing.T) {
	t.Parallel()

	theme := newTheme()
	tests := []struct {
		name string
		typ  model.EventType
		want string
	}{
		{name: "session", typ: model.EventTypeSessionStarted, want: theme.EventSession.Render("x")},
		{name: "proxy", typ: model.EventTypeProxyStarted, want: theme.EventProxy.Render("x")},
		{name: "request in flight", typ: model.EventTypeRequestStarted, want: theme.EventRequestInFlight.Render("x")},
		{name: "request success", typ: model.EventTypeRequestFinished, want: theme.EventSuccess.Render("x")},
		{name: "warn", typ: model.EventTypeWarn, want: theme.EventWarn.Render("x")},
		{name: "error", typ: model.EventTypeRequestFailed, want: theme.EventError.Render("x")},
		{name: "fatal", typ: model.EventTypeFatal, want: theme.EventFatal.Render("x")},
		{name: "default", typ: model.EventType("UnknownType"), want: theme.EventDefault.Render("x")},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getEventColor(theme, tt.typ).Render("x")
			if got != tt.want {
				t.Fatalf("unexpected style for %s", tt.typ)
			}
		})
	}
}

func TestViewAndPaneStructure(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})

	t.Run("view returns raw pane when unset size", func(t *testing.T) {
		t.Parallel()
		got := m.View()
		if got != m.renderAppPane() {
			t.Fatal("View should return raw pane when width/height are unset")
		}
	})

	t.Run("renderAppPane line count matches height", func(t *testing.T) {
		t.Parallel()
		m2 := m
		m2.width = 80
		m2.height = 12
		got := m2.renderAppPane()
		if got == "height of request and details did not match" || got == "height of screen does not match terminal height" {
			t.Fatalf("unexpected renderAppPane invariant error: %q", got)
		}
		if lines := strings.Count(got, "\n") + 1; lines != m2.height {
			t.Fatalf("line count = %d, want %d", lines, m2.height)
		}
	})

	t.Run("renderAppPane supports toggles", func(t *testing.T) {
		t.Parallel()
		m2 := m
		m2.width = 90
		m2.height = 14
		m2.showEvents = true
		m2.showStd = true
		m2.showSearch = true
		got := m2.renderAppPane()
		if got == "height of request and details did not match" || got == "height of screen does not match terminal height" {
			t.Fatalf("unexpected renderAppPane invariant error with toggles: %q", got)
		}
	})

	t.Run("view applies configured terminal height", func(t *testing.T) {
		t.Parallel()
		m2 := m
		m2.width = 70
		m2.height = 10

		got := m2.View()
		if lines := strings.Count(got, "\n") + 1; lines < m2.height {
			t.Fatalf("View line count = %d, want at least %d", lines, m2.height)
		}
	})
}

func TestPaneRenderersAndStatusBar(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.width = 100
	m.height = 12
	m.requests = []model.Request{
		{ID: uuid.New(), Method: "GET", Host: "a", URL: "/a", Status: 200, Duration: 5 * time.Millisecond},
		{ID: uuid.New(), Method: "POST", Host: "b", URL: "/b", Status: 500, Duration: 10 * time.Millisecond, Failed: true},
	}
	m.events = []model.Event{
		{Type: model.EventTypeWarn, Body: "warn"},
		{Type: model.EventTypeProcessStdout, Body: "out"},
		{Type: model.EventTypeProcessStderr, Body: "err"},
	}

	status := m.renderStatusBar(100)
	if !strings.Contains(status, "2 reqs") || !strings.Contains(status, "1 err") {
		t.Fatalf("status bar missing expected counters: %q", status)
	}

	search := m.renderSearchPane(20, 3)
	if len(search) != 3 {
		t.Fatalf("search pane len = %d, want 3", len(search))
	}
	for i, line := range search {
		if len(line) != 20 {
			t.Fatalf("search pane line %d len = %d, want %d", i, len(line), 20)
		}
	}

	requests := m.renderRequestPane(50, 4)
	if len(requests) != 4 {
		t.Fatalf("request pane len = %d, want 4", len(requests))
	}

	details := m.renderDetailsPane(30, 4)
	if len(details) != 4 {
		t.Fatalf("details pane len = %d, want 4", len(details))
	}

	events := m.renderEventsPane(60, 4)
	if len(events) != 4 {
		t.Fatalf("events pane len = %d, want 4", len(events))
	}
	if strings.Contains(strings.Join(events, "\n"), "out") || strings.Contains(strings.Join(events, "\n"), "err") {
		t.Fatal("events pane should filter stdout/stderr events")
	}

	std := m.renderStdPane(60, 4)
	if len(std) != 4 {
		t.Fatalf("std pane len = %d, want 4", len(std))
	}
	joined := strings.Join(std, "\n")
	if !strings.Contains(joined, "out") || !strings.Contains(joined, "err") {
		t.Fatal("std pane should include stdout/stderr logs")
	}
}

func TestRenderEventsPane_ErrorAndPIDBranches(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.events = []model.Event{
		{Type: model.EventTypeWarn, Body: "old"},
		{Type: model.EventTypeRequestFailed, Body: "failed body", PID: 123, Time: time.Now()},
		{Type: model.EventTypeFatal, Body: "fatal body", Time: time.Now()},
	}

	lines := m.renderEventsPane(60, 3)
	if len(lines) != 3 {
		t.Fatalf("events pane len = %d, want 3", len(lines))
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "123") {
		t.Fatalf("expected PID to appear in events pane, got: %q", joined)
	}
	if !strings.Contains(joined, "failed body") {
		t.Fatalf("expected failed body to appear in events pane, got: %q", joined)
	}
	if !strings.Contains(joined, "fatal body") {
		t.Fatalf("expected fatal body to appear in events pane, got: %q", joined)
	}
}
