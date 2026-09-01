//go:build !windows

package main

import (
	"os"
	"syscall"
)

var sigHUP = syscall.SIGHUP

func sendSignal(pid int, sig os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}
