package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"termtap.dev/internal/model"
)

func CommandString(c model.Command) string {
	return fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
}

func NewProcess(cmd model.Command, addr string, ch chan<- model.Event) *model.Process {
	proc := exec.Command(cmd.Name, cmd.Args...)
	configureProcessForSignals(proc)

	injectEnv(proc, addr)

	stdout, err := proc.StdoutPipe()
	if err != nil {
		ch <- model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeWarn,
			Body: fmt.Sprintf("could not open stdout pipe: %q", err),
			PID:  proc.Process.Pid,
		}
	} else {
		go readPipe(stdout, model.EventTypeProcessStdout, ch)
	}

	stderr, err := proc.StderrPipe()
	if err != nil {
		ch <- model.Event{
			Time: time.Now().Local(),
			Type: model.EventTypeWarn,
			Body: fmt.Sprintf("could not open stderr pipe: %q", err),
			PID:  proc.Process.Pid,
		}
	} else {
		go readPipe(stderr, model.EventTypeProcessStderr, ch)
	}

	return &model.Process{
		Command: cmd,
		Exec:    proc,
		Running: false,
		Done:    make(chan struct{}),
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

func readPipe(pipe io.Reader, t model.EventType, ch chan<- model.Event) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		ch <- model.Event{
			Time: time.Now().Local(),
			Type: t,
			Body: scanner.Text(),
		}
	}
}

func UpdateStatus(proc *model.Process, running bool, ch chan<- model.Event) {
	if proc == nil {
		return
	}

	if proc.Running == running {
		return
	}

	proc.Running = running

	var (
		t      model.EventType
		status string
	)
	if running {
		t = model.EventTypeProcessStarted
		status = "running"
	} else {
		t = model.EventTypeProcessExited
		status = "stopped"
	}

	ch <- model.Event{
		Time: time.Now().Local(),
		Type: t,
		Body: fmt.Sprintf("set process status to %s", status),
		PID:  proc.Exec.Process.Pid,
	}
}
