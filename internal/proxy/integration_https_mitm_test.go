package proxy

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

type hijackPipeWriter struct {
	conn net.Conn

	header http.Header
	code   int
}

func (w *hijackPipeWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *hijackPipeWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *hijackPipeWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *hijackPipeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, nil, nil
}

func startMITMConnect(t *testing.T, transport http.RoundTripper) (net.Conn, *tls.Conn, chan model.Event, chan struct{}) {
	t.Helper()

	ca := newTestCA(t)
	clientConn, serverConn := net.Pipe()

	writer := &hijackPipeWriter{conn: serverConn}
	req, err := http.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	if err != nil {
		t.Fatalf("NewRequest(CONNECT) error = %v", err)
	}
	req.Host = "example.com:443"

	ch := make(chan model.Event, 16)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}

	handleDone := make(chan struct{})
	go func() {
		handleConnect(writer, req, ch, transport, ca, ps)
		close(handleDone)
	}()

	reader := bufio.NewReader(clientConn)
	connectResp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = clientConn.Close()
		_ = serverConn.Close()
		t.Fatalf("ReadResponse(CONNECT established) error = %v", err)
	}
	if connectResp.StatusCode != http.StatusOK {
		_ = clientConn.Close()
		_ = serverConn.Close()
		t.Fatalf("CONNECT established status = %d, want %d", connectResp.StatusCode, http.StatusOK)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	tlsClient := tls.Client(clientConn, &tls.Config{
		ServerName: "example.com",
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	})

	if err := tlsClient.Handshake(); err != nil {
		_ = tlsClient.Close()
		_ = serverConn.Close()
		t.Fatalf("tls handshake error = %v", err)
	}

	t.Cleanup(func() {
		_ = tlsClient.Close()
		_ = serverConn.Close()
	})

	return clientConn, tlsClient, ch, handleDone
}

func TestHTTPSE2E_MITMHandleConnectFlow(t *testing.T) {
	transport := &mockTransport{fn: func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" {
			t.Fatalf("inner request scheme = %q, want https", r.URL.Scheme)
		}
		if r.URL.Host == "" {
			t.Fatal("inner request host should be populated")
		}

		return &http.Response{
			StatusCode: http.StatusCreated,
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     http.Header{"Content-Type": {"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("mitm-ok")),
		}, nil
	}}
	clientConn, tlsClient, ch, handleDone := startMITMConnect(t, transport)

	// 3) Send decrypted inner request over tunnel and read MITM response
	innerReq, err := http.NewRequest(http.MethodGet, "https://example.com/inside", nil)
	if err != nil {
		t.Fatalf("NewRequest(inner) error = %v", err)
	}
	innerReq.Host = "example.com"
	innerReq.Close = true
	if err := innerReq.Write(tlsClient); err != nil {
		t.Fatalf("innerReq.Write() error = %v", err)
	}

	innerResp, err := http.ReadResponse(bufio.NewReader(tlsClient), innerReq)
	if err != nil {
		t.Fatalf("ReadResponse(inner) error = %v", err)
	}
	defer innerResp.Body.Close()

	body, err := io.ReadAll(innerResp.Body)
	if err != nil {
		t.Fatalf("ReadAll(inner body) error = %v", err)
	}
	if innerResp.StatusCode != http.StatusCreated {
		t.Fatalf("inner status = %d, want %d", innerResp.StatusCode, http.StatusCreated)
	}
	if got, want := string(body), "mitm-ok"; got != want {
		t.Fatalf("inner body = %q, want %q", got, want)
	}

	_ = tlsClient.Close()
	_ = clientConn.Close()

	select {
	case <-handleDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handleConnect to return")
	}

	events := drainEvents(t, ch, 4, 2*time.Second)
	if events[0].Type != model.EventTypeRequestStarted {
		t.Fatalf("event[0] = %s, want %s", events[0].Type, model.EventTypeRequestStarted)
	}
	if events[1].Type != model.EventTypeRequestStarted {
		t.Fatalf("event[1] = %s, want %s", events[1].Type, model.EventTypeRequestStarted)
	}
	if events[2].Type != model.EventTypeRequestFinished {
		t.Fatalf("event[2] = %s, want %s", events[2].Type, model.EventTypeRequestFinished)
	}
	if events[3].Type != model.EventTypeRequestFinished {
		t.Fatalf("event[3] = %s, want %s", events[3].Type, model.EventTypeRequestFinished)
	}
}

func TestHTTPSE2E_MITMUpstreamErrorReturnsHTTPErrorInsideTunnel(t *testing.T) {
	transport := &mockTransport{fn: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upstream exploded")
	}}

	clientConn, tlsClient, ch, handleDone := startMITMConnect(t, transport)

	innerReq, err := http.NewRequest(http.MethodGet, "https://example.com/fail", nil)
	if err != nil {
		t.Fatalf("NewRequest(inner) error = %v", err)
	}
	innerReq.Host = "example.com"
	if err := innerReq.Write(tlsClient); err != nil {
		t.Fatalf("innerReq.Write() error = %v", err)
	}

	innerResp, err := http.ReadResponse(bufio.NewReader(tlsClient), innerReq)
	if err != nil {
		t.Fatalf("ReadResponse(inner) error = %v", err)
	}
	defer innerResp.Body.Close()

	if innerResp.StatusCode != http.StatusBadGateway {
		t.Fatalf("inner status = %d, want %d", innerResp.StatusCode, http.StatusBadGateway)
	}

	_ = tlsClient.Close()
	_ = clientConn.Close()

	select {
	case <-handleDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handleConnect to return")
	}

	events := drainEvents(t, ch, 4, 2*time.Second)
	if events[0].Type != model.EventTypeRequestStarted ||
		events[1].Type != model.EventTypeRequestStarted ||
		events[2].Type != model.EventTypeRequestFailed ||
		events[3].Type != model.EventTypeRequestFailed {
		t.Fatalf("unexpected event sequence: %#v", events)
	}
}

func TestHTTPSE2E_MITMHandshakeFailureEmitsConnectFailed(t *testing.T) {
	ca := newTestCA(t)

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	writer := &hijackPipeWriter{conn: serverConn}
	req, err := http.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	if err != nil {
		t.Fatalf("NewRequest(CONNECT) error = %v", err)
	}
	req.Host = "example.com:443"

	ch := make(chan model.Event, 16)
	ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
	transport := &mockTransport{fn: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("should not be called")
	}}

	handleDone := make(chan struct{})
	go func() {
		handleConnect(writer, req, ch, transport, ca, ps)
		close(handleDone)
	}()

	reader := bufio.NewReader(clientConn)
	connectResp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("ReadResponse(CONNECT established) error = %v", err)
	}
	if connectResp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT established status = %d, want %d", connectResp.StatusCode, http.StatusOK)
	}

	// Write plaintext (not TLS handshake) to force handshake failure.
	if _, err := clientConn.Write([]byte("not tls handshake")); err != nil {
		t.Fatalf("client write error = %v", err)
	}
	_ = clientConn.Close()

	select {
	case <-handleDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handleConnect to return on handshake failure")
	}

	events := drainEvents(t, ch, 2, 2*time.Second)
	if events[0].Type != model.EventTypeRequestStarted || events[1].Type != model.EventTypeRequestFailed {
		t.Fatalf("unexpected event sequence: %#v", events)
	}
	if !strings.Contains(events[1].Body, "TLS handshake with client failed") {
		t.Fatalf("failed event body = %q, want handshake failure details", events[1].Body)
	}
}

func TestHTTPSE2E_MITMDecryptedReadFailureEmitsConnectFailed(t *testing.T) {
	transport := &mockTransport{fn: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("should not be called")
	}}

	clientConn, tlsClient, ch, handleDone := startMITMConnect(t, transport)

	// Send malformed decrypted payload so ReadRequest fails (non-EOF branch).
	if _, err := tlsClient.Write([]byte("BAD REQUEST\r\n\r\n")); err != nil {
		t.Fatalf("tlsClient.Write() error = %v", err)
	}
	_ = tlsClient.Close()
	_ = clientConn.Close()

	select {
	case <-handleDone:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for handleConnect to return on decrypted read failure")
	}

	events := drainEvents(t, ch, 2, 2*time.Second)
	if events[0].Type != model.EventTypeRequestStarted || events[1].Type != model.EventTypeRequestFailed {
		t.Fatalf("unexpected event sequence: %#v", events)
	}
	if !strings.Contains(events[1].Body, "failed to read decrypted HTTPS request") {
		t.Fatalf("failed event body = %q, want read failure details", events[1].Body)
	}
}
