package app

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"termtap.dev/internal/model"
)

type Session struct {
	Messages <-chan model.Message

	sigCh    chan os.Signal
	stopOnce sync.Once
}

func StartSession(cmd model.Command, addr string) (*Session, error) {
	msgs := make(chan model.Message, 256)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go StartProxy(addr, msgs)
	go StartProcess(cmd, addr, msgs, sigCh)

	return &Session{
		Messages: msgs,
		sigCh:    sigCh,
	}, nil
}

func (s *Session) Stop() {
	if s == nil {
		return
	}

	s.stopOnce.Do(func() {
		signal.Stop(s.sigCh)

		select {
		case s.sigCh <- syscall.SIGTERM:
		default:
		}
	})
}
