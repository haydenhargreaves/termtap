package tui

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"termtap.dev/internal/app"
	"termtap.dev/internal/model"
)

func TestShellExampleProducesRequestData(t *testing.T) {
	scriptPath := shellExamplePath(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "shell-example-ok")
	}))
	t.Cleanup(upstream.Close)

	t.Setenv("TERM_TAP_CURL_URL", upstream.URL+"/demo")

	addr := freeTCPAddr(t)
	s, err := app.StartSession(model.Command{Name: "sh", Args: []string{scriptPath}}, addr)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	t.Cleanup(s.Stop)

	m := NewModel(s.Events, Controls{})
	if next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}); next != nil {
		m = next.(Model)
	}

	deadline := time.After(6 * time.Second)
	for {
		select {
		case ev := <-s.Events:
			next, _ := m.Update(EventMsg{value: ev})
			m = next.(Model)
			if len(m.requests) > 0 && !m.requests[0].Pending && m.requests[0].Status == http.StatusOK && string(m.requests[0].ResponseData) == "shell-example-ok" {
				if got := string(m.requests[0].ResponseData); got != "shell-example-ok" {
					t.Fatalf("response data = %q, want %q", got, "shell-example-ok")
				}
				s.Stop()
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for request data; requests=%#v", m.requests)
		}
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("listener close error = %v", err)
	}
	return addr
}

func shellExamplePath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	path := filepath.Join(filepath.Dir(file), "..", "..", "examples", "shell", "curl.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat shell example: %v", err)
	}
	return path
}
