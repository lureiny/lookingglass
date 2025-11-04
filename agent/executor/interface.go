package executor

import (
	"context"

	pb "github.com/lureiny/lookingglass/pb"
)

// Executor defines the interface for command executors
type Executor interface {
	// Execute runs the command and streams output
	Execute(ctx context.Context, task *pb.Task, outputChan chan<- *pb.TaskOutput) error

	// Cancel cancels a running task
	Cancel(taskID string) error
}
