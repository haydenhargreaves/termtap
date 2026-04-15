package app

import (
	"sync"
	"syscall"

	"termtap.dev/internal/model"
	"termtap.dev/internal/proxy"
)

type Session struct {
	Messages <-chan model.Message

	msgCh    chan model.Message
	proxy    *model.ProxyServer
	proc     *model.Process
	stopOnce sync.Once
}

func StartSession(cmd model.Command, addr string) (*Session, error) {
	msgs := make(chan model.Message, 256)

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
		Messages: msgs,
		msgCh:    msgs,
		proxy:    ps,
		proc:     proc,
	}, nil
}

func (s *Session) Stop() {
	if s == nil {
		return
	}

	s.stopOnce.Do(func() {
		StopProcess(s.proc, s.msgCh, syscall.SIGTERM)
		proxy.Destroy(s.proxy, s.msgCh)
	})
}
