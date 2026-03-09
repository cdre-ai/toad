//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// signalRestart sends SIGUSR1 to the given PID.
func signalRestart(pid int) error {
	return syscall.Kill(pid, syscall.SIGUSR1)
}

// notifyRestart registers a channel to receive SIGUSR1.
func notifyRestart(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGUSR1)
}

// execReplace replaces the current process with the given binary.
func execReplace(binary string, args []string, env []string) error {
	return syscall.Exec(binary, args, env) //nolint:gosec // binary is from os.Executable, not user input
}
