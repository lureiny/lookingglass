package cmd

import (
	"fmt"

	"github.com/google/uuid"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	agentMtrTarget string
	agentMtrCount  int32
	agentMtrIPv6   bool
)

var agentMtrCmd = &cobra.Command{
	Use:   "agent-mtr",
	Short: "Execute MTR command directly on an agent (bypass master)",
	Long: `Execute MTR command directly on an agent via gRPC, bypassing the master server.
This is useful for testing agent connectivity and functionality independently.

Example:
  lookingglass-cli agent-mtr --host=192.168.1.100:50051 --target=8.8.8.8
  lookingglass-cli agent-mtr --host=agent.example.com:50051 --target=google.com --count=20 --tls`,
	Run: runAgentMtr,
}

func init() {
	rootCmd.AddCommand(agentMtrCmd)

	agentMtrCmd.Flags().StringVar(&agentHost, "host", "", "Agent gRPC address (e.g., localhost:50051) (required)")
	agentMtrCmd.Flags().StringVar(&agentMtrTarget, "target", "", "Target IP address or hostname (required)")
	agentMtrCmd.Flags().Int32Var(&agentMtrCount, "count", 10, "Number of pings to send to each hop")
	agentMtrCmd.Flags().BoolVar(&agentMtrIPv6, "ipv6", false, "Use IPv6")
	agentMtrCmd.Flags().StringVar(&agentAPIKey, "api-key", "", "API key for agent authentication")
	agentMtrCmd.Flags().BoolVar(&agentUseTLS, "tls", false, "Use TLS for gRPC connection")

	agentMtrCmd.MarkFlagRequired("host")
	agentMtrCmd.MarkFlagRequired("target")
}

func runAgentMtr(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentHost == "" {
		exitWithError(fmt.Errorf("--host flag is required"))
	}
	if agentMtrTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:  uuid.New().String(),
		AgentId: "direct-test", // Not used when connecting directly
		Type:    pb.TaskType_TASK_TYPE_MTR,
		Timeout: 600, // 10 minutes default timeout for MTR
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target: agentMtrTarget,
				Count:  agentMtrCount,
				Ipv6:   agentMtrIPv6,
			},
		},
	}

	// Execute task directly on agent (using shared function from agent_ping.go)
	if err := executeDirectTask(task); err != nil {
		exitWithError(err)
	}
}
