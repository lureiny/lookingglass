package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/lureiny/lookingglass/cli/client"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	agentPingTarget  string
	agentPingCount   int32
	agentPingTimeout int32
	agentPingIPv6    bool
	agentHost        string
	agentAPIKey      string
	agentUseTLS      bool
)

var agentPingCmd = &cobra.Command{
	Use:   "agent-ping",
	Short: "Execute ping command directly on an agent (bypass master)",
	Long: `Execute ping command directly on an agent via gRPC, bypassing the master server.
This is useful for testing agent connectivity and functionality independently.

Example:
  lookingglass-cli agent-ping --host=192.168.1.100:50051 --target=8.8.8.8 --count=4
  lookingglass-cli agent-ping --host=agent.example.com:50051 --target=google.com --tls --api-key=secret123`,
	Run: runAgentPing,
}

func init() {
	rootCmd.AddCommand(agentPingCmd)

	agentPingCmd.Flags().StringVar(&agentHost, "host", "", "Agent gRPC address (e.g., localhost:50051) (required)")
	agentPingCmd.Flags().StringVar(&agentPingTarget, "target", "", "Target IP address or hostname (required)")
	agentPingCmd.Flags().Int32Var(&agentPingCount, "count", 4, "Number of ping packets to send")
	agentPingCmd.Flags().Int32Var(&agentPingTimeout, "timeout", 5, "Timeout in seconds for each ping")
	agentPingCmd.Flags().BoolVar(&agentPingIPv6, "ipv6", false, "Use IPv6")
	agentPingCmd.Flags().StringVar(&agentAPIKey, "api-key", "", "API key for agent authentication")
	agentPingCmd.Flags().BoolVar(&agentUseTLS, "tls", false, "Use TLS for gRPC connection")

	agentPingCmd.MarkFlagRequired("host")
	agentPingCmd.MarkFlagRequired("target")
}

func runAgentPing(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentHost == "" {
		exitWithError(fmt.Errorf("--host flag is required"))
	}
	if agentPingTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:  uuid.New().String(),
		AgentId: "direct-test", // Not used when connecting directly
		Type:    pb.TaskType_TASK_TYPE_PING,
		Timeout: 300, // 5 minutes default timeout for the entire task
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target:  agentPingTarget,
				Count:   agentPingCount,
				Timeout: agentPingTimeout,
				Ipv6:    agentPingIPv6,
			},
		},
	}

	// Execute task directly on agent
	if err := executeDirectTask(task); err != nil {
		exitWithError(err)
	}
}

func executeDirectTask(task *pb.Task) error {
	// Create gRPC client
	grpcClient := client.NewGRPCClient(agentHost, agentAPIKey, agentUseTLS)

	// Connect to agent
	fmt.Printf("Connecting to agent at %s...\n", agentHost)
	if err := grpcClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer grpcClient.Close()

	// Health check first
	fmt.Printf("Performing health check...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	healthResp, err := grpcClient.HealthCheck(ctx)
	cancel()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Health check failed: %v\n", err)
	} else {
		fmt.Printf("Agent is healthy: %s (current tasks: %d, max concurrent: %d)\n\n",
			healthResp.Message, healthResp.CurrentTasks, healthResp.MaxConcurrent)
	}

	fmt.Printf("Submitting %s task...\n", task.Type.String())
	fmt.Printf("Task ID: %s\n\n", task.TaskId)

	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Create context with timeout
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(task.Timeout)*time.Second)
	defer cancel()

	// Channel for task completion
	done := make(chan error, 1)

	// Execute task in goroutine
	go func() {
		err := grpcClient.ExecuteTask(ctx, task, func(output string) {
			fmt.Print(output)
		})
		done <- err
	}()

	// Wait for completion or interruption
	select {
	case <-sigChan:
		fmt.Println("\nReceived interrupt signal, cancelling task...")
		cancelCtx, cancelCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCancel()
		if err := grpcClient.CancelTask(cancelCtx, task.TaskId); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to cancel task: %v\n", err)
		}
		return fmt.Errorf("task cancelled by user")

	case err := <-done:
		if err != nil {
			return err
		}
	}

	fmt.Println("\nTask completed successfully.")
	return nil
}
