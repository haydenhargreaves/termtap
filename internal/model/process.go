package model

import "os/exec"

type Command struct {
	Name string
	Args []string
}

type Process struct {
	Command Command
	Exec    *exec.Cmd
	Running bool
	Done    chan struct{}
}
