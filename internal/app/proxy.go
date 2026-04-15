package app

import (
	"errors"
	"fmt"
	"net/http"

	"termtap.dev/internal/model"
)

func StartProxy(ps *model.ProxyServer, ch chan<- model.Event) {
	if ps == nil || ps.Server == nil || ps.Listener == nil {
		return
	}

	ch <- model.Event{
		Type: model.EventTypeProxyStarting,
		Body: fmt.Sprintf("proxy server started on %s", (*ps.Listener).Addr().String()),
	}

	if err := ps.Server.Serve(*ps.Listener); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}

		ch <- model.Event{
			Type: model.EventTypeFatal,
			Body: fmt.Sprintf("fatal error in proxy server: %q", err),
		}
		return
	}
}
