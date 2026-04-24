package proxy

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

// NOTE: Run these tests with -race; they cover concurrent state.

type closeTrackingConn struct {
	closed bool
	mu     sync.Mutex
}

func (c *closeTrackingConn) Read(_ []byte) (int, error)         { return 0, net.ErrClosed }
func (c *closeTrackingConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *closeTrackingConn) Close() error                       { c.mu.Lock(); c.closed = true; c.mu.Unlock(); return nil }
func (c *closeTrackingConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (c *closeTrackingConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (c *closeTrackingConn) SetDeadline(_ time.Time) error      { return nil }
func (c *closeTrackingConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *closeTrackingConn) SetWriteDeadline(_ time.Time) error { return nil }

func (c *closeTrackingConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func TestNewProxyServer_Success(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	ch := make(chan model.Event, 16)
	ps, err := NewProxyServer("127.0.0.1:0", ch)
	if err != nil {
		t.Fatalf("NewProxyServer() error = %v", err)
	}
	t.Cleanup(func() { Destroy(ps, ch) })

	if ps.Listener == nil || *ps.Listener == nil {
		t.Fatal("Listener is nil")
	}
	if ps.Server == nil {
		t.Fatal("Server is nil")
	}
	if ps.Server.Handler == nil {
		t.Fatal("Server.Handler is nil")
	}
	if !strings.HasPrefix(ps.Url, "http://") {
		t.Fatalf("Url = %q, want http:// prefix", ps.Url)
	}
	if !ps.CAReady {
		t.Fatal("CAReady = false, want true")
	}
	if ps.CACertPath == "" {
		t.Fatal("CACertPath should not be empty")
	}
	if got, want := filepath.Dir(ps.CACertPath), filepath.Join(configRoot, caDirName); got != want {
		t.Fatalf("CACertPath dir = %q, want %q", got, want)
	}
	if _, statErr := os.Stat(ps.CACertPath); statErr != nil {
		t.Fatalf("expected CA cert on disk, stat error = %v", statErr)
	}
	if ps.Conns == nil {
		t.Fatal("Conns map is nil")
	}
}

func TestNewProxyServer_CACreatedFlagAcrossRuns(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	ch := make(chan model.Event, 16)
	ps1, err := NewProxyServer("127.0.0.1:0", ch)
	if err != nil {
		t.Fatalf("first NewProxyServer() error = %v", err)
	}
	Destroy(ps1, ch)

	ps2, err := NewProxyServer("127.0.0.1:0", ch)
	if err != nil {
		t.Fatalf("second NewProxyServer() error = %v", err)
	}
	t.Cleanup(func() { Destroy(ps2, ch) })

	if !ps1.CACreated {
		t.Fatal("first NewProxyServer should report CACreated=true")
	}
	if ps2.CACreated {
		t.Fatal("second NewProxyServer should report CACreated=false when CA already exists")
	}
}

func TestNewProxyServer_ErrorWhenCASetupFails(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	ch := make(chan model.Event, 8)
	ps, err := NewProxyServer("127.0.0.1:0", ch)
	if err == nil {
		Destroy(ps, ch)
		t.Fatal("NewProxyServer() error = nil, want non-nil")
	}
	if ps != nil {
		t.Fatalf("proxy server = %#v, want nil", ps)
	}
}

func TestNewProxyServer_ErrorWhenListenFails(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = occupied.Close() })

	addr := occupied.Addr().String()
	ch := make(chan model.Event, 8)
	ps, gotErr := NewProxyServer(addr, ch)
	if gotErr == nil {
		Destroy(ps, ch)
		t.Fatal("NewProxyServer() error = nil, want non-nil")
	}
	if ps != nil {
		t.Fatalf("proxy server = %#v, want nil", ps)
	}
}

func TestDestroy(t *testing.T) {
	t.Parallel()

	t.Run("nil-safe", func(t *testing.T) {
		t.Parallel()
		Destroy(nil, make(chan model.Event, 1))
	})

	t.Run("emits ProxyStopped when server exists", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 2)
		ps := &model.ProxyServer{Server: &http.Server{}, Conns: make(map[net.Conn]struct{})}

		Destroy(ps, ch)

		select {
		case ev := <-ch:
			if ev.Type != model.EventTypeProxyStopped {
				t.Fatalf("event type = %s, want %s", ev.Type, model.EventTypeProxyStopped)
			}
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for ProxyStopped event")
		}
	})
}

func TestConnectionTrackingHelpers(t *testing.T) {
	t.Parallel()

	t.Run("track and untrack are nil-safe", func(t *testing.T) {
		t.Parallel()
		trackConnection(nil, nil)
		untrackConnection(nil, nil)
		closeTrackedConnections(nil)
	})

	t.Run("track/untrack mutate connection set", func(t *testing.T) {
		t.Parallel()

		ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
		c := &closeTrackingConn{}

		trackConnection(ps, c)
		if _, ok := ps.Conns[c]; !ok {
			t.Fatal("connection not tracked")
		}

		untrackConnection(ps, c)
		if _, ok := ps.Conns[c]; ok {
			t.Fatal("connection still tracked after untrack")
		}
	})

	t.Run("closeTrackedConnections closes all tracked conns", func(t *testing.T) {
		t.Parallel()

		ps := &model.ProxyServer{Conns: make(map[net.Conn]struct{})}
		c1 := &closeTrackingConn{}
		c2 := &closeTrackingConn{}
		ps.Conns[c1] = struct{}{}
		ps.Conns[c2] = struct{}{}

		closeTrackedConnections(ps)

		if !c1.isClosed() || !c2.isClosed() {
			t.Fatalf("expected all tracked conns closed, got c1=%v c2=%v", c1.isClosed(), c2.isClosed())
		}
	})
}

func TestDestroy_ShutdownsServerContext(t *testing.T) {
	t.Parallel()

	// Basic smoke test: ensure a real server can be shut down via Destroy.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error = %v", err)
	}

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })}
	go func() {
		_ = srv.Serve(ln)
	}()

	ch := make(chan model.Event, 2)
	ps := &model.ProxyServer{Server: srv, Conns: make(map[net.Conn]struct{})}
	Destroy(ps, ch)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		t.Fatalf("server should be closed after Destroy, got shutdown error %v", err)
	}
}
