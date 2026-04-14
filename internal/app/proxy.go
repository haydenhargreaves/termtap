package app

import (
	"fmt"

	"termtap.dev/internal/model"
	"termtap.dev/internal/proxy"
)

func StartProxy(addr string, ch chan<- model.Message) {
	ps, err := proxy.NewProxyServer(addr, ch)
	if err != nil {
		ch <- model.Message{
			Type: model.MessageTypeFatal,
			Body: fmt.Sprintf("%q", err),
		}
		return
	}
	defer proxy.Destory(ps, ch)

	ch <- model.Message{
		Type: model.MessageTypeProxyStarting,
		Body: fmt.Sprintf("proxy server started on %s", addr),
	}

	if err := ps.Server.Serve(*ps.Listener); err != nil {
		ch <- model.Message{
			Type: model.MessageTypeFatal,
			Body: fmt.Sprintf("%q", err),
		}
		return
	}
}
