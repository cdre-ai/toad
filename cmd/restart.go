package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/scaler-tech/toad/internal/state"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Gracefully restart the running toad daemon",
	Long:  "Sends SIGUSR1 to the running daemon, which drains all in-flight work and restarts with the current binary.",
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	db, err := state.OpenDB()
	if err != nil {
		return fmt.Errorf("opening state db: %w", err)
	}
	defer db.Close()

	stats, err := db.ReadDaemonStats()
	if err != nil {
		return fmt.Errorf("reading daemon stats: %w", err)
	}
	if stats == nil || time.Since(stats.Heartbeat) > 30*time.Second {
		return fmt.Errorf("no running toad daemon found")
	}

	pid := stats.PID
	if pid <= 0 {
		return fmt.Errorf("invalid daemon PID: %d", pid)
	}

	fmt.Printf("Sending restart signal to toad daemon (PID %d)...\n", pid)
	if err := signalRestart(pid); err != nil {
		return fmt.Errorf("sending restart signal to PID %d: %w", pid, err)
	}

	fmt.Println("Restart signal sent. The daemon will drain in-flight work and restart.")
	fmt.Println("Monitor progress with `toad status`.")
	return nil
}
