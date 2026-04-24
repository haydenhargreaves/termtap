package process

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"termtap.dev/internal/model"
)

func TestCommandString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  model.Command
		want string
	}{
		{
			name: "empty args",
			cmd:  model.Command{Name: "go", Args: []string{}},
			want: "go ",
		},
		{
			name: "multiple args",
			cmd:  model.Command{Name: "curl", Args: []string{"-s", "https://example.com"}},
			want: "curl -s https://example.com",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CommandString(tt.cmd); got != tt.want {
				t.Fatalf("CommandString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInjectEnv(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("sh", "-c", "true")
	injectEnv(cmd, "127.0.0.1:8080")

	mustContain := []string{
		"HTTP_PROXY=http://127.0.0.1:8080",
		"http_proxy=http://127.0.0.1:8080",
		"HTTPS_PROXY=http://127.0.0.1:8080",
		"https_proxy=http://127.0.0.1:8080",
		"NO_PROXY=",
		"no_proxy=",
	}

	for _, kv := range mustContain {
		if !containsEnvEntry(cmd.Env, kv) {
			t.Fatalf("injectEnv() missing env entry %q", kv)
		}
	}
}

func TestReadPipe(t *testing.T) {
	t.Parallel()

	t.Run("stdout lines emit stdout events", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 4)
		input := strings.NewReader("line1\nline2\n")

		readPipe(input, model.EventTypeProcessStdout, ch)

		events := drainEvents(t, ch, 2, time.Second)
		if events[0].Type != model.EventTypeProcessStdout || events[0].Body != "line1" {
			t.Fatalf("event[0] = %#v, want stdout line1", events[0])
		}
		if events[1].Type != model.EventTypeProcessStdout || events[1].Body != "line2" {
			t.Fatalf("event[1] = %#v, want stdout line2", events[1])
		}
	})

	t.Run("stderr lines emit stderr events", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 4)
		input := strings.NewReader("err1\nerr2\n")

		readPipe(input, model.EventTypeProcessStderr, ch)

		events := drainEvents(t, ch, 2, time.Second)
		if events[0].Type != model.EventTypeProcessStderr || events[0].Body != "err1" {
			t.Fatalf("event[0] = %#v, want stderr err1", events[0])
		}
		if events[1].Type != model.EventTypeProcessStderr || events[1].Body != "err2" {
			t.Fatalf("event[1] = %#v, want stderr err2", events[1])
		}
	})
}

func TestUpdateStatus(t *testing.T) {
	t.Parallel()

	t.Run("nil process is no-op", func(t *testing.T) {
		t.Parallel()
		UpdateStatus(nil, true, make(chan model.Event, 1))
	})

	t.Run("state unchanged is no-op", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 1)
		proc := &model.Process{Running: true}
		UpdateStatus(proc, true, ch)

		select {
		case ev := <-ch:
			t.Fatalf("unexpected event for unchanged state: %#v", ev)
		default:
		}
	})

	t.Run("emits started and stopped events with pid", func(t *testing.T) {
		t.Parallel()

		ch := make(chan model.Event, 2)
		proc := &model.Process{
			Exec: &exec.Cmd{Process: &os.Process{Pid: 4321}},
		}

		UpdateStatus(proc, true, ch)
		events := drainEvents(t, ch, 1, time.Second)
		started := events[0]
		if started.Type != model.EventTypeProcessStarted {
			t.Fatalf("started type = %s, want %s", started.Type, model.EventTypeProcessStarted)
		}
		if started.PID != 4321 {
			t.Fatalf("started PID = %d, want %d", started.PID, 4321)
		}
		if !proc.Running {
			t.Fatal("proc.Running = false, want true")
		}

		UpdateStatus(proc, false, ch)
		events = drainEvents(t, ch, 1, time.Second)
		stopped := events[0]
		if stopped.Type != model.EventTypeProcessExited {
			t.Fatalf("stopped type = %s, want %s", stopped.Type, model.EventTypeProcessExited)
		}
		if stopped.PID != 4321 {
			t.Fatalf("stopped PID = %d, want %d", stopped.PID, 4321)
		}
		if proc.Running {
			t.Fatal("proc.Running = true, want false")
		}
	})
}

func TestNewProcess(t *testing.T) {
	t.Parallel()

	ch := make(chan model.Event, 8)
	cmd := model.Command{Name: "sh", Args: []string{"-c", "printf test"}}

	proc := NewProcess(cmd, "127.0.0.1:8080", ch)
	if proc == nil {
		t.Fatal("NewProcess() returned nil")
	}
	if !reflect.DeepEqual(proc.Command, cmd) {
		t.Fatalf("process command = %#v, want %#v", proc.Command, cmd)
	}
	if proc.Exec == nil {
		t.Fatal("process Exec is nil")
	}
	if proc.Running {
		t.Fatal("new process should not be running")
	}
	if proc.Done == nil {
		t.Fatal("Done channel is nil")
	}

	if got, want := proc.Exec.Args[0], "sh"; got != want {
		t.Fatalf("Exec.Args[0] = %q, want %q", got, want)
	}
	if !containsEnvEntry(proc.Exec.Env, "HTTP_PROXY=http://127.0.0.1:8080") {
		t.Fatal("process env missing injected HTTP_PROXY")
	}
}

func containsEnvEntry(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func drainEvents(t *testing.T, ch <-chan model.Event, n int, timeout time.Duration) []model.Event {
	t.Helper()

	events := make([]model.Event, 0, n)
	deadline := time.After(timeout)
	for len(events) < n {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-deadline:
			t.Fatalf("timeout waiting for %d events; got %d", n, len(events))
		}
	}

	return events
}
