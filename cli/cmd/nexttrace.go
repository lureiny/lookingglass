package cmd

import (
	"fmt"

	"github.com/google/uuid"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	nexttraceTarget string
	nexttraceIPv6   bool
	nexttraceHops   int32
)

var nexttraceCmd = &cobra.Command{
	Use:   "nexttrace",
	Short: "Execute nexttrace command via master",
	Long: `Execute nexttrace route tracing command on a remote agent via master server.
NextTrace is a modern traceroute tool with enhanced features.

Example:
  lookingglass-cli nexttrace --agent=us-west-1 --target=8.8.8.8
  lookingglass-cli nexttrace --agent=eu-central-1 --target=google.com --hops=30`,
	Run: runNextTrace,
}

func init() {
	rootCmd.AddCommand(nexttraceCmd)

	nexttraceCmd.Flags().StringVar(&nexttraceTarget, "target", "", "Target IP address or hostname (required)")
	nexttraceCmd.Flags().BoolVar(&nexttraceIPv6, "ipv6", false, "Use IPv6")
	nexttraceCmd.Flags().Int32Var(&nexttraceHops, "hops", 30, "Maximum number of hops")

	nexttraceCmd.MarkFlagRequired("target")
}

func runNextTrace(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentID == "" {
		exitWithError(fmt.Errorf("--agent flag is required"))
	}
	if nexttraceTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:   uuid.New().String(),
		AgentId:  agentID,
		TaskName: "nexttrace", // New: using task_name field
		Type:     pb.TaskType_TASK_TYPE_NEXTTRACE, // Deprecated
		Timeout:  600, // 10 minutes default timeout
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target: nexttraceTarget,
				Count:  nexttraceHops,
				Ipv6:   nexttraceIPv6,
			},
		},
	}

	// Execute task (using shared function from ping.go)
	if err := executeTask(task); err != nil {
		exitWithError(err)
	}
}
