package client

import "os/exec"

type Exec interface {
	Command(string, []string) *exec.Cmd
}

type Executor struct{}

func (e Executor) Command(name string, arg []string) *exec.Cmd {
	return exec.Command(name, arg...)
}