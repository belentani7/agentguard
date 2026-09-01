// +build windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

func sendStopSignal(proc *os.Process) error {
	// Windows doesn't have SIGSTOP; suspend the process threads instead
	return fmt.Errorf("SIGSTOP not supported on Windows; use task manager or 'agentguard pause' via API")
}

func sendContSignal(proc *os.Process) error {
	// Windows doesn't have SIGCONT
	return fmt.Errorf("SIGCONT not supported on Windows")
}

func SetSysProcAttr() *syscall.SysProcAttr {
	// On Windows, use CREATE_NEW_PROCESS_GROUP instead of Setsid
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
