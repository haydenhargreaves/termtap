package tui

import (
	"errors"
	"net/http"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

func TestNewModelDefaults(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event)
	m := NewModel(ch, Controls{})

	if m.channel != ch {
		t.Fatal("channel not set")
	}
	if len(m.events) != 0 || len(m.requests) != 0 {
		t.Fatal("events/requests should initialize empty")
	}
	if m.width != 0 || m.height != 0 {
		t.Fatal("width/height should initialize zero")
	}
	if m.showEvents || m.showStd || m.showSearch || m.restarting {
		t.Fatal("toggle flags should initialize false")
	}
	if m.searchQuery != "" {
		t.Fatal("search query should initialize empty")
	}
}

func TestInitBatchesEventAndTick(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event)
	m := NewModel(ch, Controls{})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd")
	}
	if _, ok := cmd().(tea.BatchMsg); !ok {
		t.Fatalf("Init cmd message type = %T, want tea.BatchMsg", cmd())
	}
}

func TestWaitForEvent(t *testing.T) {
	t.Parallel()

	t.Run("returns EventMsg when channel has value", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 1)
		ch <- model.Event{Type: model.EventTypeWarn, Body: "hello"}

		msg := waitForEvent(ch)()
		ev, ok := msg.(EventMsg)
		if !ok {
			t.Fatalf("msg type = %T, want EventMsg", msg)
		}
		if ev.value.Body != "hello" {
			t.Fatalf("event body = %q, want %q", ev.value.Body, "hello")
		}
	})

	t.Run("returns ErrMsg when channel closed", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event)
		close(ch)

		msg := waitForEvent(ch)()
		if _, ok := msg.(ErrMsg); !ok {
			t.Fatalf("msg type = %T, want ErrMsg", msg)
		}
	})
}

func TestMessagesCommands(t *testing.T) {
	t.Parallel()

	t.Run("restartCmd nil restart returns nil", func(t *testing.T) {
		t.Parallel()
		if cmd := restartCmd(nil); cmd != nil {
			t.Fatal("restartCmd(nil) should return nil")
		}
	})

	t.Run("restartCmd wraps restart result", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("boom")
		msg := restartCmd(func() error { return wantErr })()
		rm, ok := msg.(RestartResultMsg)
		if !ok {
			t.Fatalf("msg type = %T, want RestartResultMsg", msg)
		}
		if !errors.Is(rm.err, wantErr) {
			t.Fatalf("restart result error = %v, want %v", rm.err, wantErr)
		}
	})

	t.Run("tickCmd emits TickMsg", func(t *testing.T) {
		t.Parallel()
		cmd := tickCmd()
		if cmd == nil {
			t.Fatal("tickCmd returned nil")
		}

		msgCh := make(chan tea.Msg, 1)
		go func() { msgCh <- cmd() }()

		select {
		case msg := <-msgCh:
			if _, ok := msg.(TickMsg); !ok {
				t.Fatalf("msg type = %T, want TickMsg", msg)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("timeout waiting for tick message")
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	t.Run("window size updates dimensions", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		got := next.(Model)
		if got.width != 120 || got.height != 40 {
			t.Fatalf("dimensions = (%d,%d), want (120,40)", got.width, got.height)
		}
	})

	t.Run("tick updates now and reschedules only with pending requests", func(t *testing.T) {
		t.Parallel()
		now := time.Now()

		m1 := NewModel(make(chan model.Event), Controls{})
		next1, cmd1 := m1.Update(TickMsg{Now: now})
		got1 := next1.(Model)
		if !got1.now.Equal(now) {
			t.Fatal("tick should update now")
		}
		if cmd1 != nil {
			t.Fatal("tick without pending requests should not reschedule")
		}

		m2 := NewModel(make(chan model.Event), Controls{})
		m2.requests = append(m2.requests, model.Request{ID: uuid.New(), Pending: true})
		_, cmd2 := m2.Update(TickMsg{Now: now})
		if cmd2 == nil {
			t.Fatal("tick with pending requests should reschedule")
		}
	})

	t.Run("key handling toggles and quit", func(t *testing.T) {
		t.Parallel()

		m := NewModel(make(chan model.Event), Controls{})
		next, quitCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		_ = next
		if quitCmd == nil {
			t.Fatal("q should return quit cmd")
		}

		nextCtrlC, quitCtrlC := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		_ = nextCtrlC
		if quitCtrlC == nil {
			t.Fatal("ctrl+c should return quit cmd")
		}

		next2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
		if !next2.(Model).showEvents {
			t.Fatal("e should toggle showEvents")
		}

		next3, _ := next2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
		if !next3.(Model).showStd {
			t.Fatal("o should toggle showStd")
		}

		next4, _ := next3.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
		if !next4.(Model).showSearch {
			t.Fatal("/ should enable search")
		}

		nextSearch, _ := next4.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if nextSearch.(Model).searchQuery != "x" {
			t.Fatal("typed rune should update search query")
		}

		nextSearchSpace, _ := nextSearch.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
		if nextSearchSpace.(Model).searchQuery != "x " {
			t.Fatal("space should update search query")
		}

		nextSearch2, _ := nextSearchSpace.(Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
		if nextSearch2.(Model).searchQuery != "x" {
			t.Fatal("backspace should remove one character")
		}

		nextSearch3, _ := nextSearch2.(Model).Update(tea.KeyMsg{Type: tea.KeyBackspace})
		if nextSearch3.(Model).searchQuery != "" {
			t.Fatal("backspace should update search query")
		}

		next5, _ := nextSearch3.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
		if next5.(Model).showSearch {
			t.Fatal("esc should disable search")
		}
		if next5.(Model).searchQuery != "" {
			t.Fatal("esc should clear search query")
		}

		next6, cmd6 := next5.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		if cmd6 != nil {
			t.Fatal("unknown key should not return command")
		}
		if next6.(Model).showEvents != next5.(Model).showEvents ||
			next6.(Model).showStd != next5.(Model).showStd ||
			next6.(Model).showSearch != next5.(Model).showSearch {
			t.Fatal("unknown key should not alter toggle state")
		}
	})

	t.Run("restart key guarded by state and control fn", func(t *testing.T) {
		t.Parallel()

		m := NewModel(make(chan model.Event), Controls{})
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
		if cmd != nil {
			t.Fatal("ctrl+r with nil restart control should return nil cmd")
		}
		if next.(Model).restarting {
			t.Fatal("restarting should remain false when restart control missing")
		}

		m2 := NewModel(make(chan model.Event), Controls{Restart: func() error { return nil }})
		next2, cmd2 := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
		if cmd2 == nil {
			t.Fatal("ctrl+r with restart control should return cmd")
		}
		if !next2.(Model).restarting {
			t.Fatal("restarting should be true after ctrl+r")
		}

		next3, cmd3 := next2.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlR})
		if cmd3 != nil {
			t.Fatal("ctrl+r while restarting should return nil cmd")
		}
		if !next3.(Model).restarting {
			t.Fatal("restarting should stay true while guarded")
		}
	})

	t.Run("ErrMsg pushes warning event", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		next, _ := m.Update(ErrMsg{err: errors.New("closed")})
		got := next.(Model)
		if len(got.events) != 1 {
			t.Fatalf("event len = %d, want 1", len(got.events))
		}
		if got.events[0].Type != model.EventTypeWarn {
			t.Fatalf("event type = %s, want %s", got.events[0].Type, model.EventTypeWarn)
		}
	})

	t.Run("RestartResultMsg clears restarting and warns on error", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		m.restarting = true

		next1, _ := m.Update(RestartResultMsg{err: nil})
		if next1.(Model).restarting {
			t.Fatal("restarting should clear on restart result")
		}

		next2, _ := m.Update(RestartResultMsg{err: errors.New("fail")})
		got2 := next2.(Model)
		if len(got2.events) != 1 || got2.events[0].Type != model.EventTypeWarn {
			t.Fatalf("expected warn event on restart error, got %#v", got2.events)
		}
	})

	t.Run("EventMsg updates events and requests", func(t *testing.T) {
		t.Parallel()
		ch := make(chan model.Event, 2)
		m := NewModel(ch, Controls{})

		reqID := uuid.New()
		startEv := EventMsg{value: model.Event{Type: model.EventTypeRequestStarted, Request: model.Request{ID: reqID, Method: http.MethodGet, Pending: true}}}
		next1, cmd1 := m.Update(startEv)
		if cmd1 == nil {
			t.Fatal("EventMsg should return wait/tick cmd")
		}
		got1 := next1.(Model)
		if len(got1.events) != 1 || len(got1.requests) != 1 {
			t.Fatalf("expected one event and one request, got events=%d requests=%d", len(got1.events), len(got1.requests))
		}

		finishReq := got1.requests[0]
		finishReq.Pending = false
		finishReq.Status = 200
		finishEv := EventMsg{value: model.Event{Type: model.EventTypeRequestFinished, Request: finishReq}}
		next2, cmd2 := got1.Update(finishEv)
		got2 := next2.(Model)
		if got2.requests[0].Pending {
			t.Fatal("request should be updated to non-pending")
		}
		if got2.requests[0].Status != 200 {
			t.Fatalf("request status = %d, want 200", got2.requests[0].Status)
		}

		if cmd2 == nil {
			t.Fatal("expected waitForEvent cmd after finished request")
		}
		ch <- model.Event{Type: model.EventTypeWarn, Body: "next"}
		msg := cmd2()
		if _, ok := msg.(EventMsg); !ok {
			t.Fatalf("cmd2 message type = %T, want EventMsg", msg)
		}
	})
}

func TestModelHelpers(t *testing.T) {
	t.Parallel()

	t.Run("pushEvent trims to maxEvents", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		for range maxEvents + 5 {
			m.pushEvent(model.Event{Body: "x"})
		}
		if len(m.events) != maxEvents {
			t.Fatalf("events len = %d, want %d", len(m.events), maxEvents)
		}
	})

	t.Run("createRequest ignores CONNECT and trims", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		m.createRequest(model.Request{Method: http.MethodConnect})
		if len(m.requests) != 0 {
			t.Fatal("CONNECT request should be ignored")
		}

		for range maxRequests + 3 {
			m.createRequest(model.Request{ID: uuid.New(), Method: http.MethodGet})
		}
		if len(m.requests) != maxRequests {
			t.Fatalf("requests len = %d, want %d", len(m.requests), maxRequests)
		}
	})

	t.Run("updateRequest updates only matching request", func(t *testing.T) {
		t.Parallel()
		id1 := uuid.New()
		id2 := uuid.New()
		m := NewModel(make(chan model.Event), Controls{})
		m.requests = []model.Request{{ID: id1, Status: 100}, {ID: id2, Status: 101}}

		m.updateRequest(model.Request{ID: id2, Status: 202})
		if m.requests[0].Status != 100 || m.requests[1].Status != 202 {
			t.Fatalf("unexpected statuses after update: %#v", m.requests)
		}
	})

	t.Run("hasPendingRequests true and false", func(t *testing.T) {
		t.Parallel()
		m := NewModel(make(chan model.Event), Controls{})
		if m.hasPendingRequests() {
			t.Fatal("empty model should not have pending requests")
		}
		m.requests = []model.Request{{ID: uuid.New(), Pending: false}, {ID: uuid.New(), Pending: true}}
		if !m.hasPendingRequests() {
			t.Fatal("expected pending requests to be true")
		}
	})
}
