package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"termtap.dev/internal/model"
)

const (
	proxyReadHeaderTimeout = 10 * time.Second
	proxyIdleTimeout       = 30 * time.Second
)

func NewProxyServer(addr string, ch chan<- model.Event) (*model.ProxyServer, error) {
	ca, err := loadOrCreateCertificateAuthority()
	if err != nil {
		return nil, err
	}

	trusted, err := ca.IsTrustedBySystem()
	if err != nil {
		trusted = false
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://%s", listener.Addr().String())

	ps := &model.ProxyServer{
		Listener:   &listener,
		Url:        url,
		CACertPath: ca.CertPath(),
		CAReady:    true,
		CACreated:  ca.WasCreated(),
		CATrusted:  trusted,
		Conns:      make(map[net.Conn]struct{}),
	}
	ps.Server = &http.Server{
		Handler:           proxyHandler(ch, ca, ps),
		ReadHeaderTimeout: proxyReadHeaderTimeout,
		IdleTimeout:       proxyIdleTimeout,
	}

	return ps, nil
}

// BUG: Not sure what all this does
func Destroy(ps *model.ProxyServer, ch chan<- model.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if ps != nil && ps.Server != nil {
		closeTrackedConnections(ps)
		_ = ps.Server.Shutdown(ctx)
		ch <- model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeProxyStopped,
			Body: "proxy server was destroyed",
		}
	}
}

func trackConnection(ps *model.ProxyServer, conn net.Conn) {
	if ps == nil || conn == nil {
		return
	}

	ps.ConnMu.Lock()
	defer ps.ConnMu.Unlock()
	ps.Conns[conn] = struct{}{}
}

func untrackConnection(ps *model.ProxyServer, conn net.Conn) {
	if ps == nil || conn == nil {
		return
	}

	ps.ConnMu.Lock()
	defer ps.ConnMu.Unlock()
	delete(ps.Conns, conn)
}

func closeTrackedConnections(ps *model.ProxyServer) {
	if ps == nil {
		return
	}

	// Get all of the connections while claiming the mutex.
	// Then close the mutex to allow access to the server object quicker.
	// Then a loop can run to close the connections, without needing access
	// to the server's mutex.
	ps.ConnMu.Lock()
	conns := make([]net.Conn, 0, len(ps.Conns))
	for conn := range ps.Conns {
		conns = append(conns, conn)
	}
	ps.ConnMu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}
