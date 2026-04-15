package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"termtap.dev/internal/model"
)

func NewProxyServer(addr string, ch chan<- model.Event) (*model.ProxyServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://%s", listener.Addr().String())

	ps := &model.ProxyServer{
		Listener: &listener,
		Server:   &http.Server{Handler: proxyHandler(ch)},
		Url:      url,
	}

	return ps, nil
}

// BUG: Not sure what all this does
func Destroy(ps *model.ProxyServer, ch chan<- model.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if ps != nil && ps.Server != nil {
		_ = ps.Server.Shutdown(ctx)
		ch <- model.Event{
			Type: model.EventTypeProxyStarted,
			Body: "proxy server was destroyed",
		}
	}
}
