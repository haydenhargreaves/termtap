package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"termtap.dev/internal/model"
	"termtap.dev/internal/process"
)

func StartProcess(cmd model.Command, addr string, ch chan<- model.Message, sigCh <-chan os.Signal) {
	ch <- model.Message{
		Type: model.MessageTypeProcessStarting,
		Body: fmt.Sprintf("spawning process '%s'", process.CommandString(cmd)),
	}

	proc := process.NewProcess(cmd, addr, ch)

	if err := proc.Exec.Start(); err != nil {
		ch <- model.Message{
			Type: model.MessageTypeProcessExited,
			Body: fmt.Sprintf("%q", err),
		}
		return
	}
	process.UpdateStatus(proc, true, ch)

	// Listen for SIGTERM from main process
	go func() {
		sig := <-sigCh

		ch <- model.Message{
			Type: model.MessageTypeProcessSignaled,
			Body: fmt.Sprintf("process with pid '%d' is being killed", proc.Exec.Process.Pid),
			PID:  proc.Exec.Process.Pid,
		}

		if proc.Exec != nil {
			_ = process.SignalProcess(proc.Exec, sig)

			go func() {
				time.Sleep(1500 * time.Millisecond)
				if process.ProcessAlive(proc.Exec) {
					_ = process.SignalProcess(proc.Exec, syscall.SIGKILL)
				}
			}()

			process.UpdateStatus(proc, false, ch)
		}
	}()

	if err := proc.Exec.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			ch <- model.Message{
				Type:     model.MessageTypeProcessExited,
				Body:     fmt.Sprintf("process pid '%d' exited by itself", proc.Exec.Process.Pid),
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

}
