package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	masterURL string
	agentID   string
)

var rootCmd = &cobra.Command{
	Use:   "lookingglass-cli",
	Short: "LookingGlass CLI - Network diagnostic tool client",
	Long: `LookingGlass CLI is a command-line client for the LookingGlass distributed network diagnostic system.
It allows you to execute network diagnostic commands (ping, mtr, etc.) on remote agents and view real-time results.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&masterURL, "master", "ws://localhost:8081/ws/task", "Master WebSocket URL")
	rootCmd.PersistentFlags().StringVar(&agentID, "agent", "", "Agent ID to execute the task on (required)")
	rootCmd.MarkPersistentFlagRequired("agent")
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
