package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

// NOTE: Run with -race; this validates cross-component concurrency.

func TestSessionIntegration_LifecycleAndRequestEvents(t *testing.T) {
	addr := freeTCPAddr(t)
	s, err := StartSession(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, addr)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(upstream.Close)

	startupEvents := collectUntilTypes(t, s.Events, []model.EventType{
		model.EventTypeProxyStarting,
		model.EventTypeProcessStarting,
		model.EventTypeProcessStarted,
	}, 3*time.Second)

	proxyURL, err := url.Parse(s.proxy.Url)
	if err != nil {
		t.Fatalf("url.Parse(proxy) error = %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   3 * time.Second,
	}

	resp, err := client.Get(upstream.URL + "/session")
	if err != nil {
		t.Fatalf("proxy request error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	requestEvents := collectUntilTypes(t, s.Events, []model.EventType{
		model.EventTypeRequestStarted,
		model.EventTypeRequestFinished,
	}, 3*time.Second)

	s.Stop()
	select {
	case <-s.proc.Done:
	case <-time.After(4 * time.Second):
		t.Fatal("timeout waiting for process stop")
	}

	shutdownEvents := collectUntilTypes(t, s.Events, []model.EventType{
		model.EventTypeProxyStopped,
		model.EventTypeProcessSignaled,
		model.EventTypeProcessExited,
	}, 4*time.Second)

	if !isBefore(startupEvents, model.EventTypeProcessStarting, model.EventTypeProcessStarted) {
		t.Fatalf("expected %s before %s in startup events: %#v", model.EventTypeProcessStarting, model.EventTypeProcessStarted, startupEvents)
	}
	if !isBefore(requestEvents, model.EventTypeRequestStarted, model.EventTypeRequestFinished) {
		t.Fatalf("expected %s before %s in request events: %#v", model.EventTypeRequestStarted, model.EventTypeRequestFinished, requestEvents)
	}
	if !isBefore(shutdownEvents, model.EventTypeProcessSignaled, model.EventTypeProcessExited) {
		t.Fatalf("expected %s before %s in shutdown events: %#v", model.EventTypeProcessSignaled, model.EventTypeProcessExited, shutdownEvents)
	}
}

func TestSessionIntegration_RestartProcessEmitsLifecycleEvents(t *testing.T) {
	addr := freeTCPAddr(t)
	s, err := StartSession(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, addr)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	t.Cleanup(func() { s.Stop() })

	_ = collectUntilTypes(t, s.Events, []model.EventType{
		model.EventTypeProcessStarted,
		model.EventTypeProxyStarting,
	}, 3*time.Second)

	if err := s.RestartProcess(); err != nil {
		t.Fatalf("RestartProcess() error = %v", err)
	}

	events := collectUntilTypes(t, s.Events, []model.EventType{
		model.EventTypeProcessRestarting,
		model.EventTypeProcessSignaled,
		model.EventTypeProcessExited,
		model.EventTypeProcessStarting,
		model.EventTypeProcessStarted,
	}, 4*time.Second)

	if !isBefore(events, model.EventTypeProcessRestarting, model.EventTypeProcessStarted) {
		t.Fatalf("expected restarting before process started, got %#v", events)
	}
}
