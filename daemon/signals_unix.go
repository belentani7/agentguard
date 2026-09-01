// +build !windows

package daemon

import (
	"os"
	"syscall"
)

func sendStopSignal(proc *os.Process) error {
	return proc.Signal(syscall.SIGSTOP)
}

func sendContSignal(proc *os.Process) error {
	return proc.Signal(syscall.SIGCONT)
}

func SetSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
