//go:build windows

package main

import (
	"os"
)

// On Windows, we use a dummy signal since SIGHUP doesn't exist.
// The daemon polls for config changes instead.
var sigHUP os.Signal = nil

func sendSignal(pid int, sig os.Signal) error {
	// On Windows, we can't send arbitrary Unix signals.
	// Write a reload flag file instead.
	reloadFile := configDir + "/reload.flag"
	return os.WriteFile(reloadFile, []byte("1"), 0644)
}
