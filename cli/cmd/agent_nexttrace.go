package cmd

import (
	"fmt"

	"github.com/google/uuid"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	agentNexttraceTarget string
	agentNexttraceHops   int32
	agentNexttraceIPv6   bool
)

var agentNexttraceCmd = &cobra.Command{
	Use:   "agent-nexttrace",
	Short: "Execute nexttrace command directly on an agent (bypass master)",
	Long: `Execute nexttrace command directly on an agent via gRPC, bypassing the master server.
This is useful for testing agent connectivity and functionality independently.

Example:
  lookingglass-cli agent-nexttrace --host=192.168.1.100:50051 --target=8.8.8.8
  lookingglass-cli agent-nexttrace --host=agent.example.com:50051 --target=google.com --hops=30 --tls`,
	Run: runAgentNexttrace,
}

func init() {
	rootCmd.AddCommand(agentNexttraceCmd)

	agentNexttraceCmd.Flags().StringVar(&agentHost, "host", "", "Agent gRPC address (e.g., localhost:50051) (required)")
	agentNexttraceCmd.Flags().StringVar(&agentNexttraceTarget, "target", "", "Target IP address or hostname (required)")
	agentNexttraceCmd.Flags().Int32Var(&agentNexttraceHops, "hops", 30, "Maximum number of hops")
	agentNexttraceCmd.Flags().BoolVar(&agentNexttraceIPv6, "ipv6", false, "Use IPv6")
	agentNexttraceCmd.Flags().StringVar(&agentAPIKey, "api-key", "", "API key for agent authentication")
	agentNexttraceCmd.Flags().BoolVar(&agentUseTLS, "tls", false, "Use TLS for gRPC connection")

	agentNexttraceCmd.MarkFlagRequired("host")
	agentNexttraceCmd.MarkFlagRequired("target")
}

func runAgentNexttrace(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentHost == "" {
		exitWithError(fmt.Errorf("--host flag is required"))
	}
	if agentNexttraceTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:  uuid.New().String(),
		AgentId: "direct-test", // Not used when connecting directly
		Type:    pb.TaskType_TASK_TYPE_NEXTTRACE,
		Timeout: 600, // 10 minutes default timeout
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target: agentNexttraceTarget,
				Count:  agentNexttraceHops,
				Ipv6:   agentNexttraceIPv6,
			},
		},
	}

	// Execute task directly on agent (using shared function from agent_ping.go)
	if err := executeDirectTask(task); err != nil {
		exitWithError(err)
	}
}
