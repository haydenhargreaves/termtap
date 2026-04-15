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

func StartProcess(cmd model.Command, addr string, ch chan<- model.Message) (*model.Process, error) {
	ch <- model.Message{
		Type: model.MessageTypeProcessStarting,
		Body: fmt.Sprintf("spawning process '%s'", process.CommandString(cmd)),
	}

	proc := process.NewProcess(cmd, addr, ch)

	if err := proc.Exec.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	process.UpdateStatus(proc, true, ch)

	go waitForProcessExit(proc, ch)

	return proc, nil
}

func StopProcess(proc *model.Process, ch chan<- model.Message, sig syscall.Signal) {
	if proc == nil || proc.Exec == nil || proc.Exec.Process == nil {
		return
	}

	ch <- model.Message{
		Type: model.MessageTypeProcessSignaled,
		Body: fmt.Sprintf("process with pid '%d' is being killed", proc.Exec.Process.Pid),
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

func waitForProcessExit(proc *model.Process, ch chan<- model.Message) {
	if proc == nil || proc.Exec == nil {
		return
	}

	if err := proc.Exec.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			ch <- model.Message{
				Type:     model.MessageTypeProcessExited,
				Body:     fmt.Sprintf("process pid '%d' exited", proc.Exec.Process.Pid),
				PID:      proc.Exec.Process.Pid,
				ExitCode: exitErr.ExitCode(),
			}
			process.UpdateStatus(proc, false, ch)
			return
		}

		ch <- model.Message{
			Type: model.MessageTypeFatal,
			Body: fmt.Sprintf("%q", err),
		}
		process.UpdateStatus(proc, false, ch)
		return
	}

	ch <- model.Message{
		Type:     model.MessageTypeProcessExited,
		Body:     fmt.Sprintf("process pid '%d' exited", proc.Exec.Process.Pid),
		PID:      proc.Exec.Process.Pid,
		ExitCode: 0,
	}
	process.UpdateStatus(proc, false, ch)
}
