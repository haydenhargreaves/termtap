package app

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"termtap.dev/internal/model"
	"termtap.dev/internal/process"
	"termtap.dev/internal/proxy"
)

type Session struct {
	Events <-chan model.Event

	ch    chan model.Event
	proxy *model.ProxyServer
	proc  *model.Process
	cmd   model.Command
	addr  string
	once  sync.Once

	restartMu  sync.Mutex
	restarting bool
	stopped    bool
}

var ErrRestartInProgress = errors.New("restart already in progress")
var ErrSessionStopped = errors.New("session is stopped")

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
		cmd:    cmd,
		addr:   addr,
	}, nil
}

func (s *Session) Stop() {
	if s == nil {
		return
	}

	s.once.Do(func() {
		s.restartMu.Lock()
		s.stopped = true
		proc := s.proc
		proxyServer := s.proxy
		ch := s.ch
		s.restartMu.Unlock()

		StopProcess(proc, ch, syscall.SIGTERM)
		proxy.Destroy(proxyServer, ch)
	})
}

func (s *Session) RestartProcess() error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}

	s.restartMu.Lock()
	if s.stopped {
		s.restartMu.Unlock()
		return ErrSessionStopped
	}
	if s.restarting {
		s.restartMu.Unlock()
		return ErrRestartInProgress
	}
	s.restarting = true
	current := s.proc
	cmd := s.cmd
	addr := s.addr
	ch := s.ch
	s.restartMu.Unlock()

	defer func() {
		s.restartMu.Lock()
		s.restarting = false
		s.restartMu.Unlock()
	}()

	ch <- model.Event{
		Time: time.Now().Local(),
		Type: model.EventTypeProcessRestarting,
		Body: fmt.Sprintf("restarting process '%s'", process.CommandString(cmd)),
	}

	if current != nil {
		StopProcess(current, ch, syscall.SIGTERM)
		if !waitForProcessStop(current, 3*time.Second) {
			return fmt.Errorf("timeout while waiting for process to stop")
		}
	}

	proc, err := StartProcess(cmd, addr, ch)
	if err != nil {
		return err
	}

	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	if s.stopped {
		StopProcess(proc, ch, syscall.SIGTERM)
		return fmt.Errorf("session stopped during restart: %w", ErrSessionStopped)
	}
	s.proc = proc

	return nil
}

func waitForProcessStop(proc *model.Process, timeout time.Duration) bool {
	if proc == nil || proc.Done == nil {
		return true
	}

	select {
	case <-proc.Done:
		return true
	case <-time.After(timeout):
		return false
	}
}
