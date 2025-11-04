package cmd

import (
	"fmt"

	"github.com/google/uuid"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	mtrTarget string
	mtrCount  int32
	mtrIPv6   bool
)

var mtrCmd = &cobra.Command{
	Use:   "mtr",
	Short: "Execute MTR (My Traceroute) command on a remote agent",
	Long: `Execute MTR command on a remote agent and display the results in real-time.

MTR combines the functionality of traceroute and ping in a single network diagnostic tool.

Example:
  lookingglass-cli mtr --agent=us-west-1 --target=8.8.8.8
  lookingglass-cli mtr --agent=eu-central-1 --target=google.com --count=20 --ipv6`,
	Run: runMTR,
}

func init() {
	rootCmd.AddCommand(mtrCmd)

	mtrCmd.Flags().StringVar(&mtrTarget, "target", "", "Target IP address or hostname (required)")
	mtrCmd.Flags().Int32Var(&mtrCount, "count", 10, "Number of pings to send to each hop")
	mtrCmd.Flags().BoolVar(&mtrIPv6, "ipv6", false, "Use IPv6")

	mtrCmd.MarkFlagRequired("target")
}

func runMTR(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentID == "" {
		exitWithError(fmt.Errorf("--agent flag is required"))
	}
	if mtrTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:   uuid.New().String(),
		AgentId:  agentID,
		TaskName: "mtr", // New: using task_name field
		Type:     pb.TaskType_TASK_TYPE_MTR, // Deprecated
		Timeout:  600, // 10 minutes default timeout for MTR
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target: mtrTarget,
				Count:  mtrCount,
				Ipv6:   mtrIPv6,
			},
		},
	}

	// Execute task
	if err := executeTask(task); err != nil {
		exitWithError(err)
	}
}
