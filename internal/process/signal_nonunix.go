//go:build !unix

package process

import (
	"os"
	"os/exec"
)

func configureProcessForSignals(cmd *exec.Cmd) {
	_ = cmd
}

func SignalProcess(cmd *exec.Cmd, sig os.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Signal(sig)
}

func ProcessAlive(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.Process == nil {
		return false
	}

	if cmd.ProcessState == nil {
		return true
	}

	return !cmd.ProcessState.Exited()
}
