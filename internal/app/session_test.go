package app

import (
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func TestStartSession(t *testing.T) {
	t.Run("happy path creates proxy and process", func(t *testing.T) {
		addr := freeTCPAddr(t)
		s, err := StartSession(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, addr)
		if err != nil {
			t.Fatalf("StartSession() error = %v", err)
		}
		if s == nil {
			t.Fatal("StartSession() returned nil session")
		}
		if s.proxy == nil {
			t.Fatal("session.proxy is nil")
		}
		if s.proc == nil {
			t.Fatal("session.proc is nil")
		}

		s.Stop()
		if s.proc != nil && s.proc.Done != nil {
			select {
			case <-s.proc.Done:
			case <-time.After(4 * time.Second):
				t.Fatal("timeout waiting for process stop")
			}
		}
	})

	t.Run("error when proxy creation fails", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")

		s, err := StartSession(model.Command{Name: "sh", Args: []string{"-c", "true"}}, "127.0.0.1:0")
		if err == nil {
			if s != nil {
				s.Stop()
			}
			t.Fatal("StartSession() error = nil, want non-nil")
		}
		if s != nil {
			t.Fatalf("session = %#v, want nil", s)
		}
	})

	t.Run("destroys proxy when process startup fails", func(t *testing.T) {
		addr := freeTCPAddr(t)

		s, err := StartSession(model.Command{Name: "definitely-not-a-real-command"}, addr)
		if err == nil {
			if s != nil {
				s.Stop()
			}
			t.Fatal("StartSession() error = nil, want non-nil")
		}

		deadline := time.After(3 * time.Second)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			ln, listenErr := net.Listen("tcp", addr)
			if listenErr == nil {
				_ = ln.Close()
				break
			}
			select {
			case <-deadline:
				t.Fatalf("address %s did not become reusable, got err: %v", addr, listenErr)
			case <-ticker.C:
			}
		}
	})

	t.Run("stop during restart stops new process and returns ErrSessionStopped", func(t *testing.T) {
		s := &Session{
			ch:   make(chan model.Event, 64),
			cmd:  model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}},
			addr: "127.0.0.1:0",
			proc: &model.Process{Done: make(chan struct{})},
		}
		close(s.proc.Done)

		s.restartMu.Lock()
		errCh := make(chan error, 1)
		stopDone := make(chan struct{})

		go func() {
			errCh <- s.RestartProcess()
		}()
		go func() {
			s.Stop()
			close(stopDone)
		}()

		s.restartMu.Unlock()

		select {
		case err := <-errCh:
			if !errors.Is(err, ErrSessionStopped) {
				t.Fatalf("error = %v, want %v", err, ErrSessionStopped)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("timeout waiting for RestartProcess result")
		}

		select {
		case <-stopDone:
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for Stop completion")
		}
	})
}

func TestSessionStop_Idempotent(t *testing.T) {
	addr := freeTCPAddr(t)
	s, err := StartSession(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, addr)
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	s.Stop()
	s.Stop()

	if s.proc != nil && s.proc.Done != nil {
		select {
		case <-s.proc.Done:
		case <-time.After(4 * time.Second):
			t.Fatal("timeout waiting for process to stop")
		}
	}
}

func TestRestartProcess(t *testing.T) {
	t.Run("nil session returns error", func(t *testing.T) {
		var s *Session
		err := s.RestartProcess()
		if err == nil {
			t.Fatal("RestartProcess() error = nil, want non-nil")
		}
	})

	t.Run("stopped session returns ErrSessionStopped", func(t *testing.T) {
		s := &Session{stopped: true}
		err := s.RestartProcess()
		if !errors.Is(err, ErrSessionStopped) {
			t.Fatalf("error = %v, want %v", err, ErrSessionStopped)
		}
	})

	t.Run("concurrent restart returns ErrRestartInProgress", func(t *testing.T) {
		s := &Session{restarting: true}
		err := s.RestartProcess()
		if !errors.Is(err, ErrRestartInProgress) {
			t.Fatalf("error = %v, want %v", err, ErrRestartInProgress)
		}
	})

	t.Run("timeout waiting for stop returns error", func(t *testing.T) {
		s := &Session{
			ch:   make(chan model.Event, 8),
			cmd:  model.Command{Name: "sh", Args: []string{"-c", "true"}},
			addr: "127.0.0.1:0",
			proc: &model.Process{Done: make(chan struct{})},
		}

		err := s.RestartProcess()
		if err == nil {
			t.Fatal("RestartProcess() error = nil, want non-nil")
		}
	})

	t.Run("successful restart swaps process", func(t *testing.T) {
		addr := freeTCPAddr(t)
		oldProc := &model.Process{Done: make(chan struct{})}
		close(oldProc.Done)

		s := &Session{
			ch:   make(chan model.Event, 32),
			cmd:  model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}},
			addr: addr,
			proc: oldProc,
		}

		err := s.RestartProcess()
		if err != nil {
			t.Fatalf("RestartProcess() error = %v", err)
		}
		if _, ok := waitForEventType(t, s.ch, model.EventTypeProcessRestarting, time.Second); !ok {
			t.Fatalf("did not receive %s event", model.EventTypeProcessRestarting)
		}
		if s.proc == nil {
			t.Fatal("session process is nil after restart")
		}
		if s.proc == oldProc {
			t.Fatal("session process pointer was not swapped")
		}

		StopProcess(s.proc, s.ch, syscall.SIGTERM)
		select {
		case <-s.proc.Done:
		case <-time.After(4 * time.Second):
			t.Fatal("timeout waiting for restarted process to stop")
		}
	})
}

func TestWaitForProcessStop(t *testing.T) {
	t.Parallel()

	t.Run("nil process and nil done are true", func(t *testing.T) {
		t.Parallel()
		if !waitForProcessStop(nil, time.Millisecond) {
			t.Fatal("waitForProcessStop(nil) = false, want true")
		}
		if !waitForProcessStop(&model.Process{}, time.Millisecond) {
			t.Fatal("waitForProcessStop(process with nil done) = false, want true")
		}
	})

	t.Run("done channel closes before timeout", func(t *testing.T) {
		t.Parallel()
		proc := &model.Process{Done: make(chan struct{})}
		close(proc.Done)

		if !waitForProcessStop(proc, time.Second) {
			t.Fatal("waitForProcessStop() = false, want true")
		}
	})

	t.Run("timeout returns false", func(t *testing.T) {
		t.Parallel()
		proc := &model.Process{Done: make(chan struct{})}
		if waitForProcessStop(proc, 20*time.Millisecond) {
			t.Fatal("waitForProcessStop() = true, want false")
		}
	})
}
