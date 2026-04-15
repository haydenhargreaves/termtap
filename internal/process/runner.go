package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"termtap.dev/internal/model"
)

func CommandString(c model.Command) string {
	return fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
}

func NewProcess(cmd model.Command, addr string, ch chan<- model.Message) *model.Process {
	proc := exec.Command(cmd.Name, cmd.Args...)
	configureProcessForSignals(proc)

	injectEnv(proc, addr)

	stdout, err := proc.StdoutPipe()
	if err != nil {
		ch <- model.Message{
			Type: model.MessageTypeWarn,
			Body: fmt.Sprintf("could not open stdout pipe: %q", err),
			PID:  proc.Process.Pid,
		}
	} else {
		go readPipe(stdout, model.MessageTypeProcessStdout, ch)
	}

	stderr, err := proc.StderrPipe()
	if err != nil {
		ch <- model.Message{
			Type: model.MessageTypeWarn,
			Body: fmt.Sprintf("could not open stderr pipe: %q", err),
			PID:  proc.Process.Pid,
		}
	} else {
		go readPipe(stderr, model.MessageTypeProcessStderr, ch)
	}

	return &model.Process{
		Command: cmd,
		Exec:    proc,
		Running: false,
	}
}

func injectEnv(proc *exec.Cmd, addr string) {
	proxyAddr := "http://" + addr
	injected := []string{
		"HTTP_PROXY=" + proxyAddr,
		"http_proxy=" + proxyAddr,
		"HTTPS_PROXY=" + proxyAddr, // TODO: HTTP NOT SUPPORTED
		"https_proxy=" + proxyAddr,
		// "ALL_PROXY=" + proxyAddr,
		// "all_proxy=" + proxyAddr,
		"NO_PROXY=",
		"no_proxy=",
	}

	proc.Env = append(os.Environ(), injected...)
}

func readPipe(pipe io.Reader, t model.MessageType, ch chan<- model.Message) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- model.Message{
			Type: t,
			Body: scanner.Text(),
		}
	}
}

func UpdateStatus(proc *model.Process, running bool, ch chan<- model.Message) {
	if proc == nil {
		return
	}

	if proc.Running == running {
		return
	}

	proc.Running = running

	var (
		t      model.MessageType
		status string
	)
	if running {
		t = model.MessageTypeProcessStarted
		status = "running"
	} else {
		t = model.MessageTypeProcessExited
		status = "stopped"
	}

	ch <- model.Message{
		Type: t,
		Body: fmt.Sprintf("Set process pid '%d' status to %s", proc.Exec.Process.Pid, status),
		PID:  proc.Exec.Process.Pid,
	}
}
