//go:build unix

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

type customSignal struct{}

func (customSignal) String() string { return "custom" }
func (customSignal) Signal()        {}

// NOTE: Run these tests with -race in CI for signal/process safety.

func TestConfigureProcessForSignals(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("sh", "-c", "sleep 0.1")
	configureProcessForSignals(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("Setpgid = false, want true")
	}
}

func TestSignalProcess_NilSafe(t *testing.T) {
	t.Parallel()

	if err := SignalProcess(nil, syscall.SIGTERM); err != nil {
		t.Fatalf("SignalProcess(nil) error = %v, want nil", err)
	}

	cmd := &exec.Cmd{}
	if err := SignalProcess(cmd, syscall.SIGTERM); err != nil {
		t.Fatalf("SignalProcess(cmd without process) error = %v, want nil", err)
	}

	cmd.Process = &os.Process{Pid: 0}
	if err := SignalProcess(cmd, syscall.SIGTERM); err != nil {
		t.Fatalf("SignalProcess(pid<=0) error = %v, want nil", err)
	}
}

func TestSignalProcess_ESRCHIsTreatedAsSuccess(t *testing.T) {
	t.Parallel()

	cmd := &exec.Cmd{Process: &os.Process{Pid: 999999}}
	if err := SignalProcess(cmd, syscall.SIGTERM); err != nil {
		t.Fatalf("SignalProcess() error = %v, want nil when process group not found", err)
	}
}

func TestSignalProcess_UsesFallbackOnKillError(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("sh", "-c", "sleep 5")
	configureProcessForSignals(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command error = %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Invalid signal causes syscall.Kill to fail with EINVAL, then fallback to cmd.Process.Signal.
	err := SignalProcess(cmd, syscall.Signal(9999))
	if err == nil {
		t.Fatal("SignalProcess() error = nil, want non-nil for invalid signal")
	}
	if !(errors.Is(err, syscall.EINVAL) || errors.Is(err, os.ErrProcessDone)) {
		// OS/process timing can vary; ensure we at least failed predictably.
		t.Fatalf("SignalProcess() unexpected error: %v", err)
	}
}

func TestSignalProcess_NonSyscallSignalUsesProcessSignal(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("sh", "-c", "sleep 1")
	configureProcessForSignals(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command error = %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	err := SignalProcess(cmd, customSignal{})
	if err == nil {
		t.Fatal("SignalProcess(custom signal) error = nil, want non-nil")
	}
	if errors.Is(err, os.ErrProcessDone) {
		return
	}
	if msg := err.Error(); msg == "" {
		t.Fatalf("unexpected empty error for custom signal: %v", err)
	}
}

func TestProcessAlive(t *testing.T) {
	t.Parallel()

	if ProcessAlive(nil) {
		t.Fatal("ProcessAlive(nil) = true, want false")
	}
	if ProcessAlive(&exec.Cmd{}) {
		t.Fatal("ProcessAlive(cmd without process) = true, want false")
	}

	cmd := exec.Command("sh", "-c", "sleep 0.2")
	configureProcessForSignals(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start command error = %v", err)
	}

	if !ProcessAlive(cmd) {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal("ProcessAlive(running) = false, want true")
	}

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	deadline := time.After(time.Second)
	for {
		if !ProcessAlive(cmd) {
			return
		}
		select {
		case <-deadline:
			t.Fatal("ProcessAlive(exited) stayed true")
		default:
		}
	}
}
