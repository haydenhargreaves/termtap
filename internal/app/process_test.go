package app

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func TestStartProcess(t *testing.T) {
	t.Parallel()

	t.Run("starts process and marks running", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 32)
		proc, err := StartProcess(model.Command{Name: "sh", Args: []string{"-c", "sleep 0.2"}}, "127.0.0.1:8080", ch)
		if err != nil {
			t.Fatalf("StartProcess() error = %v", err)
		}
		t.Cleanup(func() {
			StopProcess(proc, ch, syscall.SIGTERM)
			select {
			case <-proc.Done:
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting process done in cleanup")
			}
		})

		if proc == nil || proc.Exec == nil {
			t.Fatal("StartProcess() returned nil process/exec")
		}

		events := drainEvents(t, ch, 2, time.Second)
		if !hasType(events, model.EventTypeProcessStarting) {
			t.Fatalf("missing %s event", model.EventTypeProcessStarting)
		}
		if !hasType(events, model.EventTypeProcessStarted) {
			t.Fatalf("missing %s event", model.EventTypeProcessStarted)
		}
	})

	t.Run("returns error when exec start fails", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 8)
		proc, err := StartProcess(model.Command{Name: "definitely-not-a-real-command"}, "127.0.0.1:8080", ch)
		if err == nil {
			if proc != nil && proc.Exec != nil && proc.Exec.Process != nil {
				_ = proc.Exec.Process.Kill()
			}
			t.Fatal("StartProcess() error = nil, want non-nil")
		}

		events := drainEvents(t, ch, 1, time.Second)
		if !hasType(events, model.EventTypeProcessStarting) {
			t.Fatalf("missing %s event", model.EventTypeProcessStarting)
		}
	})
}

func TestStopProcess(t *testing.T) {
	t.Parallel()

	t.Run("nil guards", func(t *testing.T) {
		t.Parallel()
		StopProcess(nil, make(chan model.Event, 1), syscall.SIGTERM)
		StopProcess(&model.Process{}, make(chan model.Event, 1), syscall.SIGTERM)
		StopProcess(&model.Process{Exec: &exec.Cmd{}}, make(chan model.Event, 1), syscall.SIGTERM)
	})

	t.Run("emits signaled event", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 32)
		proc, err := StartProcess(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, "127.0.0.1:8080", ch)
		if err != nil {
			t.Fatalf("StartProcess() error = %v", err)
		}

		StopProcess(proc, ch, syscall.SIGTERM)
		if _, ok := waitForEventType(t, ch, model.EventTypeProcessSignaled, 2*time.Second); !ok {
			t.Fatalf("did not receive %s event", model.EventTypeProcessSignaled)
		}

		select {
		case <-proc.Done:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for process to exit after signal")
		}
	})

	t.Run("schedules deterministic kill escalation hook", func(t *testing.T) {
		t.Parallel()

		origDelay := killEscalationDelay
		origScheduler := scheduleKillEscalation
		defer func() {
			killEscalationMu.Lock()
			killEscalationDelay = origDelay
			scheduleKillEscalation = origScheduler
			killEscalationMu.Unlock()
		}()

		ch := make(chan model.Event, 32)
		proc, err := StartProcess(model.Command{Name: "sh", Args: []string{"-c", "sleep 5"}}, "127.0.0.1:8080", ch)
		if err != nil {
			t.Fatalf("StartProcess() error = %v", err)
		}

		killEscalationMu.Lock()
		killEscalationDelay = 25 * time.Millisecond
		scheduled := make(chan time.Duration, 1)
		scheduleKillEscalation = func(d time.Duration, fn func()) *time.Timer {
			scheduled <- d
			go fn()
			return nil
		}
		killEscalationMu.Unlock()

		StopProcess(proc, ch, syscall.SIGTERM)

		select {
		case d := <-scheduled:
			if d != killEscalationDelay {
				t.Fatalf("scheduled delay = %v, want %v", d, killEscalationDelay)
			}
		case <-time.After(time.Second):
			t.Fatal("kill escalation was not scheduled")
		}

		select {
		case <-proc.Done:
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for process to exit after deterministic escalation")
		}
	})
}

func TestWaitForProcessExit(t *testing.T) {
	t.Parallel()

	t.Run("nil guards are no-op", func(t *testing.T) {
		t.Parallel()
		waitForProcessExit(nil, make(chan model.Event, 1))
		waitForProcessExit(&model.Process{}, make(chan model.Event, 1))
	})

	t.Run("normal exit emits process exited and closes done", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command("sh", "-c", "exit 0")
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() error = %v", err)
		}

		proc := &model.Process{Exec: cmd, Running: true, Done: make(chan struct{})}
		ch := make(chan model.Event, 8)

		waitForProcessExit(proc, ch)

		if _, ok := waitForEventType(t, ch, model.EventTypeProcessExited, time.Second); !ok {
			t.Fatalf("did not receive %s event", model.EventTypeProcessExited)
		}
		select {
		case <-proc.Done:
		case <-time.After(time.Second):
			t.Fatal("Done channel was not closed")
		}
	})

	t.Run("exit error path carries exit code", func(t *testing.T) {
		t.Parallel()

		cmd := exec.Command("sh", "-c", "exit 7")
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() error = %v", err)
		}

		proc := &model.Process{Exec: cmd, Running: true, Done: make(chan struct{})}
		ch := make(chan model.Event, 8)

		waitForProcessExit(proc, ch)

		events := drainEvents(t, ch, 2, time.Second)
		found := false
		for _, ev := range events {
			if ev.Type == model.EventTypeProcessExited && ev.ExitCode == 7 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected ProcessExited with exit code 7, got %#v", events)
		}
		select {
		case <-proc.Done:
		case <-time.After(time.Second):
			t.Fatal("Done channel was not closed")
		}
	})

	t.Run("unexpected wait failure emits fatal", func(t *testing.T) {
		t.Parallel()

		proc := &model.Process{Exec: &exec.Cmd{}, Done: make(chan struct{})}
		ch := make(chan model.Event, 8)

		waitForProcessExit(proc, ch)

		events := drainEvents(t, ch, 1, time.Second)
		if events[0].Type != model.EventTypeFatal {
			t.Fatalf("event type = %s, want %s", events[0].Type, model.EventTypeFatal)
		}
		select {
		case <-proc.Done:
		case <-time.After(time.Second):
			t.Fatal("Done channel was not closed")
		}
	})
}
