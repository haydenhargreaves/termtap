//go:build unix

package process

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureProcessForSignals(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func SignalProcess(cmd *exec.Cmd, sig os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	if pid <= 0 {
		return nil
	}

	sysSig, ok := sig.(syscall.Signal)
	if !ok {
		return cmd.Process.Signal(sig)
	}

	err := syscall.Kill(-pid, sysSig)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}

	return cmd.Process.Signal(sig)
}

func ProcessAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	err := syscall.Kill(-cmd.Process.Pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
