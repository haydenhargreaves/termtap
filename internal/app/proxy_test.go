package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

type staticErrListener struct {
	err error
}

func (l *staticErrListener) Accept() (net.Conn, error) { return nil, l.err }
func (l *staticErrListener) Close() error              { return nil }
func (l *staticErrListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func TestStartProxy_NilGuards(t *testing.T) {
	t.Parallel()

	StartProxy(nil, make(chan model.Event, 1))
	StartProxy(&model.ProxyServer{}, make(chan model.Event, 1))
	StartProxy(&model.ProxyServer{Server: &http.Server{}}, make(chan model.Event, 1))
}

func TestStartProxy_EmitsStartingAndWarnWhenUntrustedCA(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("accept failed")
	var ln net.Listener = &staticErrListener{err: listenErr}

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{
		Listener:   &ln,
		Server:     &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})},
		CAReady:    true,
		CATrusted:  false,
		CACreated:  true,
		CACertPath: "/tmp/test-ca.pem",
	}

	StartProxy(ps, ch)

	events := drainEvents(t, ch, 3, time.Second)
	if !hasType(events, model.EventTypeProxyStarting) {
		t.Fatalf("missing %s event", model.EventTypeProxyStarting)
	}
	if !hasType(events, model.EventTypeWarn) {
		t.Fatalf("missing %s event", model.EventTypeWarn)
	}
	if !containsBody(events, "generated HTTPS interception CA") {
		t.Fatalf("expected generated-CA warning body, got events: %#v", events)
	}
	if !hasType(events, model.EventTypeFatal) {
		t.Fatalf("missing %s event", model.EventTypeFatal)
	}
}

func TestStartProxy_WarnBodyForExistingUntrustedCA(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("accept failed")
	var ln net.Listener = &staticErrListener{err: listenErr}

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{
		Listener:   &ln,
		Server:     &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})},
		CAReady:    true,
		CATrusted:  false,
		CACreated:  false,
		CACertPath: "/tmp/test-ca.pem",
	}

	StartProxy(ps, ch)

	events := drainEvents(t, ch, 3, time.Second)
	if !containsBody(events, "HTTPS interception CA available at") {
		t.Fatalf("expected existing-CA warning body, got events: %#v", events)
	}
}

func TestStartProxy_NoCAWarningWhenNotReady(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("accept failed")
	var ln net.Listener = &staticErrListener{err: listenErr}

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{
		Listener:  &ln,
		Server:    &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})},
		CAReady:   false,
		CATrusted: false,
	}

	StartProxy(ps, ch)

	events := drainEvents(t, ch, 2, time.Second)
	if !hasType(events, model.EventTypeProxyStarting) {
		t.Fatalf("missing %s event", model.EventTypeProxyStarting)
	}
	if hasType(events, model.EventTypeWarn) {
		t.Fatalf("unexpected %s event when CA is not ready: %#v", model.EventTypeWarn, events)
	}
	if !hasType(events, model.EventTypeFatal) {
		t.Fatalf("missing %s event", model.EventTypeFatal)
	}
}

func TestStartProxy_NoCAWarningWhenTrusted(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("accept failed")
	var ln net.Listener = &staticErrListener{err: listenErr}

	ch := make(chan model.Event, 8)
	ps := &model.ProxyServer{
		Listener:  &ln,
		Server:    &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})},
		CAReady:   true,
		CATrusted: true,
	}

	StartProxy(ps, ch)

	events := drainEvents(t, ch, 2, time.Second)
	if !hasType(events, model.EventTypeProxyStarting) {
		t.Fatalf("missing %s event", model.EventTypeProxyStarting)
	}
	if hasType(events, model.EventTypeWarn) {
		t.Fatalf("unexpected %s event when CA is already trusted: %#v", model.EventTypeWarn, events)
	}
	if !hasType(events, model.EventTypeFatal) {
		t.Fatalf("missing %s event", model.EventTypeFatal)
	}
}

func TestStartProxy_SwallowsErrServerClosed(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	ch := make(chan model.Event, 8)
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	ps := &model.ProxyServer{Listener: &ln, Server: server}

	done := make(chan struct{})
	go func() {
		StartProxy(ps, ch)
		close(done)
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for StartProxy to return")
	}

	events := drainEvents(t, ch, 1, time.Second)
	if !hasType(events, model.EventTypeProxyStarting) {
		t.Fatalf("missing %s event", model.EventTypeProxyStarting)
	}
	if hasType(events, model.EventTypeFatal) {
		t.Fatalf("unexpected %s event", model.EventTypeFatal)
	}
}
