package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"termtap.dev/internal/model"
)

func StartProxy(ps *model.ProxyServer, ch chan<- model.Event) {
	if ps == nil || ps.Server == nil || ps.Listener == nil {
		return
	}

	ch <- model.Event{
		Time: time.Now().Local(),
		Type: model.EventTypeProxyStarting,
		Body: fmt.Sprintf("proxy server started on %s", (*ps.Listener).Addr().String()),
	}

	if ps.CAReady && !ps.CATrusted {
		body := fmt.Sprintf("HTTPS interception CA available at %s; trust this certificate to inspect HTTPS traffic", ps.CACertPath)
		eventType := model.EventTypeWarn
		if ps.CACreated {
			body = fmt.Sprintf("generated HTTPS interception CA at %s; trust this certificate to inspect HTTPS traffic", ps.CACertPath)
		}

		ch <- model.Event{
			Time: time.Now().Local(),
			Type: eventType,
			Body: body,
		}
	}

	if err := ps.Server.Serve(*ps.Listener); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}

		ch <- model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeFatal,
			Body: fmt.Sprintf("fatal error in proxy server: %q", err),
		}
		return
	}
}
