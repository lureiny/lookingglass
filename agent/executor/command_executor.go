package executor

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"time"

	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ArgsBuilder is a function that builds command-line arguments from task parameters
type ArgsBuilder func(*pb.NetworkTestParams) []string

// LineFormatter is an optional function to format output lines
type LineFormatter func(string) string

// CommandExecutor is a generic executor for external commands
type CommandExecutor struct {
	name          string        // Display name for logging
	cmdPath       string        // Path to the command binary
	argsBuilder   ArgsBuilder   // Function to build command arguments
	lineFormatter LineFormatter // Optional formatter for output lines (nil if not needed)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewCommandExecutor creates a new generic command executor
func NewCommandExecutor(name, cmdPath string, argsBuilder ArgsBuilder, lineFormatter LineFormatter) *CommandExecutor {
	return &CommandExecutor{
		name:          name,
		cmdPath:       cmdPath,
		argsBuilder:   argsBuilder,
		lineFormatter: lineFormatter,
	}
}

// Execute executes a command task
func (e *CommandExecutor) Execute(ctx context.Context, task *pb.Task, outputChan chan<- *pb.TaskOutput) error {
	e.ctx, e.cancel = context.WithCancel(ctx)

	// Get network test parameters
	params := task.GetNetworkTest()
	if params == nil {
		return fmt.Errorf("invalid parameters for %s task", e.name)
	}

	// Build command arguments
	args := e.argsBuilder(params)
	cmd := exec.CommandContext(e.ctx, e.cmdPath, args...)

	logger.Info(fmt.Sprintf("Starting %s command", e.name),
		zap.String("task_id", task.TaskId),
		zap.String("target", params.Target),
		zap.Strings("args", args),
	)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Get stderr pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s command: %w", e.name, err)
	}

	// Stream output
	errChan := make(chan error, 1)

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			// Apply line formatter if provided
			if e.lineFormatter != nil {
				line = e.lineFormatter(line)
			}

			select {
			case <-ctx.Done():
				return
			case outputChan <- &pb.TaskOutput{
				TaskId:     task.TaskId,
				OutputLine: line,
				Timestamp:  timestamppb.New(time.Now()),
				Status:     pb.TaskStatus_TASK_STATUS_RUNNING,
			}:
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Error(fmt.Sprintf("Error reading %s stdout", e.name),
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()

			// Apply line formatter if provided
			if e.lineFormatter != nil {
				line = e.lineFormatter(line)
			}

			select {
			case <-ctx.Done():
				return
			case outputChan <- &pb.TaskOutput{
				TaskId:       task.TaskId,
				OutputLine:   line,
				Timestamp:    timestamppb.New(time.Now()),
				Status:       pb.TaskStatus_TASK_STATUS_RUNNING,
				ErrorMessage: scanner.Text(), // Original line without formatting
			}:
			}
		}
		if err := scanner.Err(); err != nil {
			logger.Error(fmt.Sprintf("Error reading %s stderr", e.name),
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
		}
	}()

	// Wait for command to complete
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Error(fmt.Sprintf("%s command failed", e.name),
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
		} else {
			logger.Info(fmt.Sprintf("%s command completed successfully", e.name),
				zap.String("task_id", task.TaskId),
			)
		}
		errChan <- err
	}()

	// Wait for either context cancellation or command completion
	select {
	case <-e.ctx.Done():
		// Kill the process if context is cancelled
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		outputChan <- &pb.TaskOutput{
			TaskId:       task.TaskId,
			Timestamp:    timestamppb.New(time.Now()),
			Status:       pb.TaskStatus_TASK_STATUS_CANCELLED,
			ErrorMessage: "Task cancelled",
		}
		return e.ctx.Err()

	case err := <-errChan:
		if err != nil {
			outputChan <- &pb.TaskOutput{
				TaskId:       task.TaskId,
				Timestamp:    timestamppb.New(time.Now()),
				Status:       pb.TaskStatus_TASK_STATUS_FAILED,
				ErrorMessage: err.Error(),
			}
			return err
		}

		// Success
		outputChan <- &pb.TaskOutput{
			TaskId:    task.TaskId,
			Timestamp: timestamppb.New(time.Now()),
			Status:    pb.TaskStatus_TASK_STATUS_COMPLETED,
		}
		return nil
	}
}

// Cancel cancels a running task
func (e *CommandExecutor) Cancel(taskID string) error {
	if e.cancel != nil {
		logger.Info(fmt.Sprintf("Cancelling %s task", e.name),
			zap.String("task_id", taskID),
		)
		e.cancel()
	}
	return nil
}
