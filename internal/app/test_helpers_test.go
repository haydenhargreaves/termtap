package app

import (
	"net"
	"strings"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func drainEvents(t *testing.T, ch <-chan model.Event, n int, timeout time.Duration) []model.Event {
	t.Helper()

	out := make([]model.Event, 0, n)
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-deadline:
			t.Fatalf("timeout waiting for %d events, got %d", n, len(out))
		}
	}

	return out
}

func hasType(events []model.Event, typ model.EventType) bool {
	for _, ev := range events {
		if ev.Type == typ {
			return true
		}
	}
	return false
}

func containsBody(events []model.Event, part string) bool {
	for _, ev := range events {
		if strings.Contains(ev.Body, part) {
			return true
		}
	}
	return false
}

func waitForEventType(t *testing.T, ch <-chan model.Event, typ model.EventType, timeout time.Duration) (model.Event, bool) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case ev := <-ch:
			if ev.Type == typ {
				return ev, true
			}
		case <-deadline:
			return model.Event{}, false
		}
	}
}

func collectUntilTypes(t *testing.T, ch <-chan model.Event, required []model.EventType, timeout time.Duration) []model.Event {
	t.Helper()

	need := make(map[model.EventType]bool, len(required))
	for _, typ := range required {
		need[typ] = true
	}

	events := make([]model.Event, 0, len(required)+8)
	deadline := time.After(timeout)
	for len(need) > 0 {
		select {
		case ev := <-ch:
			events = append(events, ev)
			delete(need, ev.Type)
		case <-deadline:
			t.Fatalf("timeout waiting for required events: remaining=%v, collected=%#v", need, events)
		}
	}

	return events
}

func isBefore(events []model.Event, first, second model.EventType) bool {
	firstIdx := -1
	secondIdx := -1
	for i, ev := range events {
		if ev.Type == first && firstIdx == -1 {
			firstIdx = i
		}
		if ev.Type == second && secondIdx == -1 {
			secondIdx = i
		}
	}

	return firstIdx >= 0 && secondIdx >= 0 && firstIdx < secondIdx
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
