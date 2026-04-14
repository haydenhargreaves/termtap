package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"termtap.dev/internal/model"
	"termtap.dev/internal/process"
)

func StartProcess(cmd model.Command, addr string, ch chan<- model.Message, sigCh <-chan os.Signal) {
	ch <- model.Message{
		Type: model.MessageTypeProcessStarting,
		Body: fmt.Sprintf("spawning process '%s'", process.CommandString(cmd)),
	}

	proc := process.NewProcess(cmd, addr, ch)

	if err := proc.Start(); err != nil {
		ch <- model.Message{
			Type: model.MessageTypeProcessExited,
			Body: fmt.Sprintf("%q", err),
		}
		return
	}

	// Listen for SIGTERM from main process
	go func() {
		sig := <-sigCh

		ch <- model.Message{
			Type: model.MessageTypeProcessSignaled,
			Body: fmt.Sprintf("process with pid '%d' is being killed", proc.Process.Pid),
			PID:  proc.Process.Pid,
		}

		if proc.Process != nil {
			_ = proc.Process.Signal(sig)
		}
	}()

	if err := proc.Wait(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			ch <- model.Message{
				Type:     model.MessageTypeProcessExited,
				Body:     "process killed itself",
				PID:      proc.Process.Pid,
				ExitCode: exitErr.ExitCode(),
			}
			return
		}

		ch <- model.Message{
			Type: model.MessageTypeFatal,
			Body: fmt.Sprintf("%q", err),
		}
		return
	}

}
