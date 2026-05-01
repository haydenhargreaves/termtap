package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

func TestParseStatusToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want int
		ok   bool
	}{
		{name: "exact code", in: "404", want: 404, ok: true},
		{name: "class code", in: "5xx", want: 5000, ok: true},
		{name: "invalid class", in: "9xx", want: 0, ok: false},
		{name: "non-number", in: "abc", want: 0, ok: false},
		{name: "too low", in: "99", want: 0, ok: false},
		{name: "too high", in: "600", want: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseStatusToken(tt.in)
			if ok != tt.ok {
				t.Fatalf("parseStatusToken(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("parseStatusToken(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseRequestSearchQuery(t *testing.T) {
	t.Parallel()

	q := parseRequestSearchQuery("api method:post status:5xx status:404 foo:bar")

	if len(q.methods) != 1 || q.methods[0] != "post" {
		t.Fatalf("methods = %#v, want [post]", q.methods)
	}
	if _, ok := q.statusHuns[5]; !ok {
		t.Fatal("expected status class 5xx to be parsed")
	}
	if _, ok := q.statuses[404]; !ok {
		t.Fatal("expected status 404 to be parsed")
	}
	if len(q.terms) != 2 || q.terms[0] != "api" || q.terms[1] != "foo:bar" {
		t.Fatalf("terms = %#v, want [api foo:bar]", q.terms)
	}
}

func TestParseRequestSearchQuery_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantTerms    []string
		wantMethods  []string
		wantStatuses []int
		wantClasses  []int
	}{
		{
			name:         "normalizes case and spaces",
			input:        "  API.Example.Com   METHOD:GET   STATUS:2xx  ",
			wantTerms:    []string{"api.example.com"},
			wantMethods:  []string{"get"},
			wantStatuses: nil,
			wantClasses:  []int{2},
		},
		{
			name:         "invalid status token falls back to free text",
			input:        "status:wat service",
			wantTerms:    []string{"status:wat", "service"},
			wantMethods:  nil,
			wantStatuses: nil,
			wantClasses:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := parseRequestSearchQuery(tt.input)

			if len(q.terms) != len(tt.wantTerms) {
				t.Fatalf("terms len = %d, want %d", len(q.terms), len(tt.wantTerms))
			}
			for i := range tt.wantTerms {
				if q.terms[i] != tt.wantTerms[i] {
					t.Fatalf("terms[%d] = %q, want %q", i, q.terms[i], tt.wantTerms[i])
				}
			}

			if len(q.methods) != len(tt.wantMethods) {
				t.Fatalf("methods len = %d, want %d", len(q.methods), len(tt.wantMethods))
			}
			for i := range tt.wantMethods {
				if q.methods[i] != tt.wantMethods[i] {
					t.Fatalf("methods[%d] = %q, want %q", i, q.methods[i], tt.wantMethods[i])
				}
			}

			for _, status := range tt.wantStatuses {
				if _, ok := q.statuses[status]; !ok {
					t.Fatalf("expected status %d to exist", status)
				}
			}
			for _, class := range tt.wantClasses {
				if _, ok := q.statusHuns[class]; !ok {
					t.Fatalf("expected status class %dxx to exist", class)
				}
			}
		})
	}
}

func TestRequestMatchesQuery(t *testing.T) {
	t.Parallel()

	req := model.Request{
		Method: "POST",
		Host:   "api.example.com",
		URL:    "/v1/login",
		RawURL: "https://api.example.com/v1/login",
		Status: 502,
	}

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "free text host", query: "api.example.com", want: true},
		{name: "free text path", query: "/v1/login", want: true},
		{name: "method match", query: "method:post", want: true},
		{name: "method mismatch", query: "method:get", want: false},
		{name: "status class match", query: "status:5xx", want: true},
		{name: "status exact mismatch", query: "status:200", want: false},
		{name: "combined and match", query: "api method:post status:5xx", want: true},
		{name: "combined and mismatch", query: "api method:post status:2xx", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := parseRequestSearchQuery(tt.query)
			got := requestMatchesQuery(req, q)
			if got != tt.want {
				t.Fatalf("requestMatchesQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSelectedRequestUsesFilteredResults(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.requests = []model.Request{
		{ID: uuid.New(), Method: "GET", Host: "a.test", URL: "/a", Status: 200},
		{ID: uuid.New(), Method: "POST", Host: "b.test", URL: "/b", Status: 500},
		{ID: uuid.New(), Method: "GET", Host: "c.test", URL: "/c", Status: 201},
	}
	m.searchQuery = "status:5xx"

	req, ok := m.selectedRequest()
	if !ok {
		t.Fatal("selectedRequest should succeed with filtered results")
	}
	if req.Host != "b.test" {
		t.Fatalf("selected request host = %q, want %q", req.Host, "b.test")
	}

	m.requestCursor = 9
	m.clampRequestCursor()
	if m.requestCursor != 0 {
		t.Fatalf("requestCursor = %d, want 0 for single filtered row", m.requestCursor)
	}
}

func TestFilteredRequestIndices_EmptyQueryReturnsAll(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.requests = []model.Request{
		{ID: uuid.New(), Host: "a", URL: "/a"},
		{ID: uuid.New(), Host: "b", URL: "/b"},
	}

	m.searchQuery = ""
	idx := m.filteredRequestIndices()
	if len(idx) != 2 {
		t.Fatalf("filteredRequestIndices len = %d, want 2", len(idx))
	}
	if idx[0] != 0 || idx[1] != 1 {
		t.Fatalf("filtered indices = %#v, want [0 1]", idx)
	}
}

func TestSearchEscClearsAndResetsSelectionState(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.showSearch = true
	m.searchQuery = "status:5xx"
	m.requestCursor = 3
	m.requestScroll = 2
	m.detailsScroll = 4

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(Model)

	if got.showSearch {
		t.Fatal("esc should close search mode")
	}
	if got.searchQuery != "" {
		t.Fatal("esc should clear search query")
	}
	if got.requestCursor != 0 || got.requestScroll != 0 || got.detailsScroll != 0 {
		t.Fatalf("scroll/cursor not reset: cursor=%d reqScroll=%d detScroll=%d", got.requestCursor, got.requestScroll, got.detailsScroll)
	}
}

func TestSearchPaneUsesThemedBackgroundFill(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	line := m.renderSearchPane(24, 1)[0]
	if lipgloss.Width(line) != 24 {
		t.Fatalf("search line width = %d, want 24", lipgloss.Width(line))
	}
	if line == "                        " {
		t.Fatal("search pane should render styled content, not raw spaces")
	}
}

func TestSearchPaneShowsQueryText(t *testing.T) {
	t.Parallel()

	m := NewModel(make(chan model.Event), Controls{})
	m.searchQuery = "method:get status:2xx api"
	line := m.renderSearchPane(64, 1)[0]
	plain := ansi.Strip(line)
	if !strings.Contains(plain, "method:get") || !strings.Contains(plain, "status:2xx") || !strings.Contains(plain, "api") {
		t.Fatalf("search pane missing query text, got %q", plain)
	}
}
