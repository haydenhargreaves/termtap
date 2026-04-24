package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"termtap.dev/internal/model"
)

type mockTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestRoundTripCapturedRequest_Success(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)

	reqURL, err := url.Parse("http://example.com/path?q=1")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL:    reqURL,
		Host:   "example.com",
		Header: http.Header{
			"Connection":    {"X-Hop"},
			"X-Hop":         {"drop"},
			"Authorization": {"Bearer token"},
			"Content-Type":  {"text/plain"},
		},
		Body: io.NopCloser(strings.NewReader("req\nbody")),
	}

	transport := &mockTransport{fn: func(outReq *http.Request) (*http.Response, error) {
		if got := outReq.Header.Get("Connection"); got != "" {
			t.Fatalf("Connection header should be stripped, got %q", got)
		}
		if got := outReq.Header.Get("X-Hop"); got != "" {
			t.Fatalf("header listed in Connection should be stripped, got %q", got)
		}

		data, readErr := io.ReadAll(outReq.Body)
		if readErr != nil {
			t.Fatalf("ReadAll(outReq.Body) error = %v", readErr)
		}
		if got, want := string(data), "req\nbody"; got != want {
			t.Fatalf("request body = %q, want %q", got, want)
		}

		return &http.Response{
			StatusCode: http.StatusCreated,
			Header: http.Header{
				"Set-Cookie":   {"session=top-secret"},
				"Connection":   {"close"},
				"Content-Type": {"text/plain"},
			},
			Body: io.NopCloser(strings.NewReader("resp\nbody")),
		}, nil
	}}

	resp, captured, responsePreview, err := roundTripCapturedRequest(req, transport, ch, "", false)
	if err != nil {
		t.Fatalf("roundTripCapturedRequest() error = %v", err)
	}
	if resp == nil {
		t.Fatal("roundTripCapturedRequest() returned nil response")
	}
	if responsePreview == nil {
		t.Fatal("roundTripCapturedRequest() returned nil response preview")
	}

	if _, readErr := io.ReadAll(resp.Body); readErr != nil {
		t.Fatalf("ReadAll(resp.Body) error = %v", readErr)
	}
	_ = resp.Body.Close()

	if got, want := string(captured.RequestData), `req\nbody`; got != want {
		t.Fatalf("captured.RequestData = %q, want %q", got, want)
	}
	if got, want := string(responsePreview.Preview()), `resp\nbody`; got != want {
		t.Fatalf("responsePreview = %q, want %q", got, want)
	}
	if got := captured.RequestHeaders.Get("Authorization"); got != "[REDACTED]" {
		t.Fatalf("Authorization header = %q, want [REDACTED]", got)
	}
	if got := captured.RequestHeaders.Get("Host"); got != "example.com" {
		t.Fatalf("Host header = %q, want example.com", got)
	}
	if got := captured.ResponseHeaders.Get("Set-Cookie"); got != "[REDACTED]" {
		t.Fatalf("Set-Cookie header = %q, want [REDACTED]", got)
	}
	if got := captured.ResponseHeaders.Get("Connection"); got != "close" {
		t.Fatalf("captured Connection header = %q, want close", got)
	}

	events := drainEvents(t, ch, 1, time.Second)
	if events[0].Type != model.EventTypeRequestStarted {
		t.Fatalf("event type = %s, want %s", events[0].Type, model.EventTypeRequestStarted)
	}
}

func TestRoundTripCapturedRequest_ErrorPath(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)

	reqURL, err := url.Parse("http://example.com/fail")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	req := &http.Request{
		Method: http.MethodPost,
		URL:    reqURL,
		Host:   "example.com",
		Header: http.Header{"Content-Type": {"text/plain"}},
		Body:   io.NopCloser(strings.NewReader("boom\nbody")),
	}

	wantErr := errors.New("upstream failed")
	transport := &mockTransport{fn: func(outReq *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(outReq.Body)
		return nil, wantErr
	}}

	resp, captured, responsePreview, gotErr := roundTripCapturedRequest(req, transport, ch, "", false)
	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("error = %v, want %v", gotErr, wantErr)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if responsePreview != nil {
		t.Fatalf("responsePreview = %#v, want nil", responsePreview)
	}
	if got, want := string(captured.RequestData), `boom\nbody`; got != want {
		t.Fatalf("captured.RequestData = %q, want %q", got, want)
	}

	events := drainEvents(t, ch, 1, time.Second)
	if events[0].Type != model.EventTypeRequestStarted {
		t.Fatalf("event type = %s, want %s", events[0].Type, model.EventTypeRequestStarted)
	}
}

func TestRoundTripCapturedRequest_InterceptedTLSDefaults(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)

	u, err := url.Parse("/secure?p=1")
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
		Header: http.Header{},
	}

	const defaultHost = "api.example.com:443"
	transport := &mockTransport{fn: func(outReq *http.Request) (*http.Response, error) {
		if got := outReq.URL.Scheme; got != "https" {
			t.Fatalf("URL.Scheme = %q, want https", got)
		}
		if got := outReq.URL.Host; got != defaultHost {
			t.Fatalf("URL.Host = %q, want %q", got, defaultHost)
		}
		if got := outReq.Host; got != defaultHost {
			t.Fatalf("Host = %q, want %q", got, defaultHost)
		}

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}}

	_, _, _, gotErr := roundTripCapturedRequest(req, transport, ch, defaultHost, true)
	if gotErr != nil {
		t.Fatalf("roundTripCapturedRequest() error = %v", gotErr)
	}
}

func TestNewConnectRequest(t *testing.T) {
	t.Parallel()

	now := time.Now().Add(-time.Second)
	req := &http.Request{Method: http.MethodConnect, Host: "example.com:443"}

	got := newConnectRequest(req, now)

	if got.Method != http.MethodConnect {
		t.Fatalf("Method = %q, want CONNECT", got.Method)
	}
	if got.Host != req.Host || got.URL != req.Host || got.RawURL != req.Host {
		t.Fatalf("connect request host/url/raw mismatch: %#v", got)
	}
	if !got.Pending || got.Failed {
		t.Fatalf("Pending/Failed = (%v,%v), want (true,false)", got.Pending, got.Failed)
	}
	if got.Status != -1 {
		t.Fatalf("Status = %d, want -1", got.Status)
	}
	if got.StartTime != now {
		t.Fatalf("StartTime = %v, want %v", got.StartTime, now)
	}
	if got.ID == uuid.Nil {
		t.Fatal("ID must be non-zero UUID")
	}
}

func TestStartFinishFailRequestEvents(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 4)
	req := model.Request{
		ID:        uuid.New(),
		Method:    http.MethodGet,
		RawURL:    "http://example.com/a",
		StartTime: time.Now().Add(-3 * time.Millisecond),
		Pending:   true,
	}

	startRequest(ch, req)
	events := drainEvents(t, ch, 1, time.Second)
	startEv := events[0]
	if startEv.Type != model.EventTypeRequestStarted {
		t.Fatalf("start event type = %s, want %s", startEv.Type, model.EventTypeRequestStarted)
	}
	if startEv.Request.Pending != true {
		t.Fatalf("start request pending = %v, want true", startEv.Request.Pending)
	}

	finishRequest(ch, req, http.StatusOK)
	events = drainEvents(t, ch, 1, time.Second)
	finishEv := events[0]
	if finishEv.Type != model.EventTypeRequestFinished {
		t.Fatalf("finish event type = %s, want %s", finishEv.Type, model.EventTypeRequestFinished)
	}
	if finishEv.Request.Pending {
		t.Fatal("finished request should not be pending")
	}
	if finishEv.Request.Failed {
		t.Fatal("finished request should not be failed")
	}
	if finishEv.Request.Status != http.StatusOK {
		t.Fatalf("finished status = %d, want %d", finishEv.Request.Status, http.StatusOK)
	}

	failRequest(ch, req, http.StatusBadGateway, "upstream error")
	events = drainEvents(t, ch, 1, time.Second)
	failEv := events[0]
	if failEv.Type != model.EventTypeRequestFailed {
		t.Fatalf("fail event type = %s, want %s", failEv.Type, model.EventTypeRequestFailed)
	}
	if failEv.Request.Pending {
		t.Fatal("failed request should not be pending")
	}
	if !failEv.Request.Failed {
		t.Fatal("failed request should be marked failed")
	}
	if failEv.Request.Status != http.StatusBadGateway {
		t.Fatalf("failed status = %d, want %d", failEv.Request.Status, http.StatusBadGateway)
	}
}
