package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

type failingResponseWriter struct {
	header http.Header
	code   int
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

type hijackFailWriter struct {
	header http.Header
	code   int
}

func (w *hijackFailWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *hijackFailWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *hijackFailWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *hijackFailWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("hijack failed")
}

type dummyConn struct{}

func (dummyConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (dummyConn) Write(p []byte) (int, error)        { return len(p), nil }
func (dummyConn) Close() error                       { return nil }
func (dummyConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (dummyConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (dummyConn) SetDeadline(_ time.Time) error      { return nil }
func (dummyConn) SetReadDeadline(_ time.Time) error  { return nil }
func (dummyConn) SetWriteDeadline(_ time.Time) error { return nil }

type writeConnectFailWriter struct {
	header http.Header
	code   int
}

func (w *writeConnectFailWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *writeConnectFailWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *writeConnectFailWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *writeConnectFailWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := bufio.NewReadWriter(
		bufio.NewReader(strings.NewReader("")),
		bufio.NewWriter(errWriter{}),
	)
	return dummyConn{}, rw, nil
}

func TestProxyHandler_NonConnectRejectsNonAbsoluteURL(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
	h := proxyHandler(ch, nil, ps)

	req := httptest.NewRequest(http.MethodGet, "/not-proxy-form", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	events := drainEvents(t, ch, 1, time.Second)
	if !hasEventType(events, model.EventTypeWarn) {
		t.Fatalf("expected %s event, got %#v", model.EventTypeWarn, events)
	}
}

func TestProxyHandler_NonConnectSuccess(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("pong"))
	}))
	t.Cleanup(upstream.Close)

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
	h := proxyHandler(ch, nil, ps)

	req := httptest.NewRequest(http.MethodGet, upstream.URL+"/ping", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
	if got, want := w.Body.String(), "pong"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got := w.Header().Get("X-Upstream"); got != "yes" {
		t.Fatalf("X-Upstream header = %q, want yes", got)
	}

	events := drainEvents(t, ch, 2, time.Second)
	if !hasEventType(events, model.EventTypeRequestStarted) {
		t.Fatalf("expected %s event", model.EventTypeRequestStarted)
	}
	if !hasEventType(events, model.EventTypeRequestFinished) {
		t.Fatalf("expected %s event", model.EventTypeRequestFinished)
	}
}

func TestProxyHandler_NonConnectUpstreamError(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
	h := proxyHandler(ch, nil, ps)

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1/fail", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadGateway)
	}

	events := drainEvents(t, ch, 2, time.Second)
	if !hasEventType(events, model.EventTypeRequestStarted) {
		t.Fatalf("expected %s event", model.EventTypeRequestStarted)
	}
	if !hasEventType(events, model.EventTypeRequestFailed) {
		t.Fatalf("expected %s event", model.EventTypeRequestFailed)
	}
}

func TestProxyHandler_NonConnectWriteFailureEmitsFailedEvent(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, bytes.NewBufferString("response-body"))
	}))
	t.Cleanup(upstream.Close)

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
	h := proxyHandler(ch, nil, ps)

	req := httptest.NewRequest(http.MethodGet, upstream.URL+"/copy-fail", nil)
	w := &failingResponseWriter{}

	h.ServeHTTP(w, req)

	events := drainEvents(t, ch, 2, time.Second)
	if !hasEventType(events, model.EventTypeRequestStarted) {
		t.Fatalf("expected %s event", model.EventTypeRequestStarted)
	}
	if !hasEventType(events, model.EventTypeRequestFailed) {
		t.Fatalf("expected %s event", model.EventTypeRequestFailed)
	}
}

func TestHandleConnect_EarlyFailures(t *testing.T) {
	t.Parallel()

	transport := &mockTransport{fn: func(req *http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}}

	tests := []struct {
		name     string
		writer   http.ResponseWriter
		req      *http.Request
		ca       *CertificateAuthority
		wantCode int
		wantBody string
		wantStat int
	}{
		{
			name:     "nil CA returns 502 and failed event",
			writer:   httptest.NewRecorder(),
			req:      httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil),
			ca:       nil,
			wantCode: http.StatusBadGateway,
			wantBody: "certificate authority is unavailable",
			wantStat: http.StatusBadGateway,
		},
		{
			name:     "nil CA with host without port still fails predictably",
			writer:   httptest.NewRecorder(),
			req:      &http.Request{Method: http.MethodConnect, Host: "example.com"},
			ca:       nil,
			wantCode: http.StatusBadGateway,
			wantBody: "certificate authority is unavailable",
			wantStat: http.StatusBadGateway,
		},
		{
			name:     "cert mint failure returns 502 and failed event",
			writer:   httptest.NewRecorder(),
			req:      &http.Request{Method: http.MethodConnect, Host: ""},
			ca:       &CertificateAuthority{},
			wantCode: http.StatusBadGateway,
			wantBody: "failed to mint interception certificate",
			wantStat: http.StatusBadGateway,
		},
		{
			name:     "non-hijacker writer returns 500 and failed event",
			writer:   httptest.NewRecorder(),
			req:      httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil),
			ca:       newTestCA(t),
			wantCode: http.StatusInternalServerError,
			wantBody: "hijack is unavailable",
			wantStat: http.StatusInternalServerError,
		},
		{
			name:     "hijack failure returns 500 and failed event",
			writer:   &hijackFailWriter{},
			req:      httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil),
			ca:       newTestCA(t),
			wantCode: http.StatusInternalServerError,
			wantBody: "CONNECT hijack failed",
			wantStat: http.StatusInternalServerError,
		},
		{
			name:     "write connect established failure emits failed event",
			writer:   &writeConnectFailWriter{},
			req:      httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil),
			ca:       newTestCA(t),
			wantCode: 0,
			wantBody: "CONNECT setup failed",
			wantStat: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := make(chan model.Event, 8)
			ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}

			handleConnect(tt.writer, tt.req, ch, transport, tt.ca, ps)

			events := drainEvents(t, ch, 2, time.Second)
			if events[0].Type != model.EventTypeRequestStarted {
				t.Fatalf("expected %s event", model.EventTypeRequestStarted)
			}
			if events[1].Type != model.EventTypeRequestFailed {
				t.Fatalf("expected %s event", model.EventTypeRequestFailed)
			}
			if events[1].Request.Method != http.MethodConnect {
				t.Fatalf("failed event request method = %q, want CONNECT", events[1].Request.Method)
			}
			if events[1].Request.Status != tt.wantStat {
				t.Fatalf("failed event request status = %d, want %d", events[1].Request.Status, tt.wantStat)
			}
			if !events[1].Request.Failed || events[1].Request.Pending {
				t.Fatalf("failed event request flags = pending:%v failed:%v, want pending:false failed:true", events[1].Request.Pending, events[1].Request.Failed)
			}
			if !strings.Contains(events[1].Body, tt.wantBody) {
				t.Fatalf("failed event body %q does not contain %q", events[1].Body, tt.wantBody)
			}

			if recorder, ok := tt.writer.(*httptest.ResponseRecorder); ok {
				if recorder.Code != tt.wantCode {
					t.Fatalf("status = %d, want %d", recorder.Code, tt.wantCode)
				}
			}
			if w, ok := tt.writer.(*hijackFailWriter); ok {
				if w.code != tt.wantCode {
					t.Fatalf("status = %d, want %d", w.code, tt.wantCode)
				}
			}
		})
	}
}

// TODO: Add full TLS MITM loop integration test.
