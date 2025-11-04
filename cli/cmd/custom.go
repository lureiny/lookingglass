package cmd

import (
	"fmt"

	"github.com/google/uuid"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/spf13/cobra"
)

var (
	customTarget   string
	customTaskName string
	customCount    int32
	customTimeout  int32
	customIPv6     bool
)

var customCmd = &cobra.Command{
	Use:   "custom",
	Short: "Execute custom command on a remote agent",
	Long: `Execute a custom command on a remote agent and display the results in real-time.

This command is used for TASK_TYPE_CUSTOM_COMMAND tasks that are defined in the agent configuration.

Example:
  lookingglass-cli custom --agent=us-west-1 --target=https://example.com
  lookingglass-cli custom --agent=eu-central-1 --target=8.8.8.8 --count=10`,
	Run: runCustom,
}

func init() {
	rootCmd.AddCommand(customCmd)

	customCmd.Flags().StringVar(&customTarget, "target", "", "Target IP address, hostname, or URL (required)")
	customCmd.Flags().StringVar(&customTaskName, "task-name", "", "Custom task name (e.g., 'curl_test') (required)")
	customCmd.Flags().Int32Var(&customCount, "count", 4, "Count parameter for custom command")
	customCmd.Flags().Int32Var(&customTimeout, "timeout", 10, "Timeout in seconds for custom command")
	customCmd.Flags().BoolVar(&customIPv6, "ipv6", false, "Use IPv6")

	customCmd.MarkFlagRequired("target")
	customCmd.MarkFlagRequired("task-name")
}

func runCustom(cmd *cobra.Command, args []string) {
	// Validate inputs
	if agentID == "" {
		exitWithError(fmt.Errorf("--agent flag is required"))
	}
	if customTarget == "" {
		exitWithError(fmt.Errorf("--target flag is required"))
	}
	if customTaskName == "" {
		exitWithError(fmt.Errorf("--task-name flag is required"))
	}

	// Create task
	task := &pb.Task{
		TaskId:  uuid.New().String(),
		AgentId: agentID,
		Type:    pb.TaskType_TASK_TYPE_CUSTOM_COMMAND,
		Timeout: 300, // 5 minutes default timeout for the entire task
		Params: &pb.Task_NetworkTest{
			NetworkTest: &pb.NetworkTestParams{
				Target:         customTarget,
				Count:          customCount,
				Timeout:        customTimeout,
				Ipv6:           customIPv6,
				CustomTaskName: customTaskName,
			},
		},
	}

	// Execute task
	if err := executeTask(task); err != nil {
		exitWithError(err)
	}
}
