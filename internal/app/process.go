package app

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"termtap.dev/internal/model"
	"termtap.dev/internal/process"
)

func StartProcess(cmd model.Command, addr string, ch chan<- model.Event) (*model.Process, error) {
	ch <- model.Event{
		Time: time.Now().Local(),
		Type: model.EventTypeProcessStarting,
		Body: fmt.Sprintf("starting process '%s'", process.CommandString(cmd)),
	}

	proc := process.NewProcess(cmd, addr, ch)

	if err := proc.Exec.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	process.UpdateStatus(proc, true, ch)

	go waitForProcessExit(proc, ch)

	return proc, nil
}

func StopProcess(proc *model.Process, ch chan<- model.Event, sig syscall.Signal) {
	if proc == nil || proc.Exec == nil || proc.Exec.Process == nil {
		return
	}

	ch <- model.Event{
		Time: time.Now().Local(),
		Type: model.EventTypeProcessSignaled,
		Body: fmt.Sprintf("process received signal '%s'", sig.String()),
		PID:  proc.Exec.Process.Pid,
	}

	_ = process.SignalProcess(proc.Exec, sig)

	go func() {
		time.Sleep(1500 * time.Millisecond)
		if process.ProcessAlive(proc.Exec) {
			_ = process.SignalProcess(proc.Exec, syscall.SIGKILL)
		}
	}()
}

func waitForProcessExit(proc *model.Process, ch chan<- model.Event) {
	if proc == nil || proc.Exec == nil {
		return
	}
	defer func() {
		if proc.Done != nil {
			close(proc.Done)
		}
	}()

	if err := proc.Exec.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			ch <- model.Event{
				Time:     time.Now().Local(),
				Type:     model.EventTypeProcessExited,
				Body:     "process exited",
				PID:      proc.Exec.Process.Pid,
				ExitCode: exitErr.ExitCode(),
			}
			process.UpdateStatus(proc, false, ch)
			return
		}

		ch <- model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeFatal,
			Body: fmt.Sprintf("%q", err),
		}
		process.UpdateStatus(proc, false, ch)
		return
	}

	code := proc.Exec.ProcessState.ExitCode()
	ch <- model.Event{
		Time:     time.Now().Local(),
		Type:     model.EventTypeProcessExited,
		Body:     "process exited",
		PID:      proc.Exec.Process.Pid,
		ExitCode: code,
	}
	process.UpdateStatus(proc, false, ch)
}
