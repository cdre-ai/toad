//go:build windows

package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

// signalRestart is not supported on Windows.
func signalRestart(_ int) error {
	return fmt.Errorf("restart signal is not supported on Windows")
}

// notifyRestart is a no-op on Windows (no SIGUSR1).
func notifyRestart(_ chan<- os.Signal) {}

// execReplace re-launches the binary as a new process on Windows
// (syscall.Exec is not available).
func execReplace(binary string, args []string, env []string) error {
	cmd := exec.Command(binary, args[1:]...) //nolint:gosec // binary is from os.Executable
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	os.Exit(0)
	return nil // unreachable
}
