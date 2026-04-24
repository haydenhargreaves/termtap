package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func TestHTTPProxyE2E_WithClientProxyConfig(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	}))
	t.Cleanup(upstream.Close)

	ch := make(chan model.Event, 64)
	ps, err := NewProxyServer("127.0.0.1:0", ch)
	if err != nil {
		t.Fatalf("NewProxyServer() error = %v", err)
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- ps.Server.Serve(*ps.Listener)
	}()

	t.Cleanup(func() {
		Destroy(ps, ch)
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for proxy server to stop")
		}
	})

	proxyURL, err := url.Parse(ps.Url)
	if err != nil {
		t.Fatalf("url.Parse(proxy) error = %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   3 * time.Second,
	}

	resp, err := client.Get(upstream.URL + "/ping")
	if err != nil {
		t.Fatalf("client.Get() error = %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(response) error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got, want := string(body), "pong"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	events := drainEvents(t, ch, 2, 2*time.Second)
	if !hasEventType(events, model.EventTypeRequestStarted) {
		t.Fatalf("missing %s event", model.EventTypeRequestStarted)
	}
	if !hasEventType(events, model.EventTypeRequestFinished) {
		t.Fatalf("missing %s event", model.EventTypeRequestFinished)
	}
}

func TestHTTPProxyE2E_UpstreamFailureEmitsRequestFailed(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	ch := make(chan model.Event, 64)
	ps, err := NewProxyServer("127.0.0.1:0", ch)
	if err != nil {
		t.Fatalf("NewProxyServer() error = %v", err)
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- ps.Server.Serve(*ps.Listener)
	}()

	t.Cleanup(func() {
		Destroy(ps, ch)
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for proxy server to stop")
		}
	})

	proxyURL, err := url.Parse(ps.Url)
	if err != nil {
		t.Fatalf("url.Parse(proxy) error = %v", err)
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   3 * time.Second,
	}

	resp, reqErr := client.Get("http://127.0.0.1:1/unreachable")
	if reqErr != nil {
		t.Fatalf("client.Get() error = %v; proxy should reply with mapped HTTP status", reqErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}

	events := drainEvents(t, ch, 2, 3*time.Second)
	if !hasEventType(events, model.EventTypeRequestStarted) {
		t.Fatalf("missing %s event", model.EventTypeRequestStarted)
	}
	if !hasEventType(events, model.EventTypeRequestFailed) {
		t.Fatalf("missing %s event", model.EventTypeRequestFailed)
	}
}
