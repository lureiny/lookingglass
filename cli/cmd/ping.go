package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lureiny/lookingglass/cli/client"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	pingTarget  string
	pingCount   int32
	pingTimeout int32
	pingIPv6    bool
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Execute ping command on a remote agent",
	Long: `Execute ping command on a remote agent and display the results in real-time.

Example:
  lookingglass-cli ping --agent=us-west-1 --target=8.8.8.8 --count=4
  lookingglass-cli ping --agent=eu-central-1 --target=google.com --count=10 --ipv6`,
	Run: runPing,
}

func init() {
	rootCmd.AddCommand(pingCmd)

	pingCmd.Flags().StringVar(&pingTarget, "target", "", "Target IP address or hostname (required)")
	pingCmd.Flags().Int32Var(&pingCount, "count", 4, "Number of ping packets to send")
	pingCmd.Flags().Int32Var(&pingTimeout, "timeout", 5, "Timeout in seconds for each ping")
	pingCmd.Flags().BoolVar(&pingIPv6, "ipv6", false, "Use IPv6")

	pingCmd.MarkFlagRequired("target")
}

func runPing(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentID == "" {
		exitWithError(fmt.Errorf("--agent flag is required"))
	}
	if pingTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:   uuid.New().String(),
		AgentId:  agentID,
		TaskName: "ping", // New: using task_name field
		Type:     pb.TaskType_TASK_TYPE_PING, // Deprecated
		Timeout:  300, // 5 minutes default timeout for the entire task
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target:  pingTarget,
				Count:   pingCount,
				Timeout: pingTimeout,
				Ipv6:    pingIPv6,
			},
		},
	}

	// Execute task
	if err := executeTask(task); err != nil {
		exitWithError(err)
	}
}

func executeTask(task *pb.Task) error {
	// Create WebSocket client
	wsClient := client.NewClient(masterURL)

	// Connect to master
	fmt.Printf("Connecting to master at %s...\n", masterURL)
	if err := wsClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer wsClient.Close()

	fmt.Printf("Connected. Submitting %s task to agent %s...\n", task.Type.String(), task.AgentId)
	fmt.Printf("Task ID: %s\n\n", task.TaskId)

	// Execute task with context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	if err := wsClient.ExecuteTask(ctx, task); err != nil {
		return err
	}

	fmt.Println("\nTask completed successfully.")
	return nil
}
