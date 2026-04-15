package app

import (
	"sync"
	"syscall"

	"termtap.dev/internal/model"
	"termtap.dev/internal/proxy"
)

type Session struct {
	Events <-chan model.Event

	ch    chan model.Event
	proxy *model.ProxyServer
	proc  *model.Process
	once  sync.Once
}

func StartSession(cmd model.Command, addr string) (*Session, error) {
	msgs := make(chan model.Event, 256)

	ps, err := proxy.NewProxyServer(addr, msgs)
	if err != nil {
		return nil, err
	}

	go StartProxy(ps, msgs)

	proc, err := StartProcess(cmd, addr, msgs)
	if err != nil {
		proxy.Destroy(ps, msgs)
		return nil, err
	}

	return &Session{
		Events: msgs,
		ch:     msgs,
		proxy:  ps,
		proc:   proc,
	}, nil
}

func (s *Session) Stop() {
	if s == nil {
		return
	}

	s.once.Do(func() {
		StopProcess(s.proc, s.ch, syscall.SIGTERM)
		proxy.Destroy(s.proxy, s.ch)
	})
}
