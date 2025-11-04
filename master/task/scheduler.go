package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lureiny/lookingglass/master/agent"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TaskInfo represents information about a running or completed task
type TaskInfo struct {
	Task       *pb.Task
	AgentID    string
	Status     pb.TaskStatus
	CreatedAt  time.Time
	ClientID   string // WebSocket client ID for output routing
	CancelFunc context.CancelFunc
}

// StreamSender interface for sending tasks to agents via stream
type StreamSender interface {
	SendTaskToAgent(agentID string, task *pb.Task) error
	CancelTaskOnAgent(agentID string, taskID string) error
}

// Scheduler manages task scheduling and execution
type Scheduler struct {
	agentManager   *agent.Manager
	streamSender   StreamSender
	globalMaxTasks int
	currentTasks   int
	tasks          map[string]*TaskInfo
	mutex          sync.RWMutex
	outputHandlers map[string]func(*pb.TaskOutput) // Task ID -> output handler
	handlerMutex   sync.RWMutex
}

// NewScheduler creates a new task scheduler
func NewScheduler(agentManager *agent.Manager, globalMaxTasks int) *Scheduler {
	return &Scheduler{
		agentManager:   agentManager,
		globalMaxTasks: globalMaxTasks,
		tasks:          make(map[string]*TaskInfo),
		outputHandlers: make(map[string]func(*pb.TaskOutput)),
	}
}

// SetStreamSender sets the stream sender for stream-based communication
func (s *Scheduler) SetStreamSender(sender StreamSender) {
	s.streamSender = sender
}

// SubmitTask submits a task for execution
func (s *Scheduler) SubmitTask(ctx context.Context, task *pb.Task, clientID string, outputHandler func(*pb.TaskOutput)) error {
	s.mutex.Lock()

	// Check global concurrency limit
	if s.currentTasks >= s.globalMaxTasks {
		s.mutex.Unlock()
		return fmt.Errorf("system busy: global task limit reached (%d/%d)", s.currentTasks, s.globalMaxTasks)
	}

	// Get agent
	agent, err := s.agentManager.GetAgent(task.AgentId)
	if err != nil {
		s.mutex.Unlock()
		return fmt.Errorf("agent not found: %w", err)
	}

	// Check agent status
	if agent.Status != pb.AgentStatus_AGENT_STATUS_ONLINE {
		s.mutex.Unlock()
		return fmt.Errorf("agent is offline: %s", task.AgentId)
	}

	// Check agent concurrency limit
	if agent.CurrentTasks >= agent.Info.MaxConcurrent {
		s.mutex.Unlock()
		return fmt.Errorf("agent busy: task limit reached (%d/%d)", agent.CurrentTasks, agent.Info.MaxConcurrent)
	}

	// Master acts as pure forwarder - no task type validation
	// Agent will validate if it supports the task and return error if not

	// Increment counters
	s.currentTasks++
	s.mutex.Unlock()

	// Increment agent task count
	if err := s.agentManager.IncrementTaskCount(task.AgentId); err != nil {
		s.mutex.Lock()
		s.currentTasks--
		s.mutex.Unlock()
		return err
	}

	// Create task info
	taskCtx, cancel := context.WithCancel(ctx)
	taskInfo := &TaskInfo{
		Task:       task,
		AgentID:    task.AgentId,
		Status:     pb.TaskStatus_TASK_STATUS_PENDING,
		CreatedAt:  time.Now(),
		ClientID:   clientID,
		CancelFunc: cancel,
	}

	// Store task info
	s.mutex.Lock()
	s.tasks[task.TaskId] = taskInfo
	s.mutex.Unlock()

	// Register output handler
	s.handlerMutex.Lock()
	s.outputHandlers[task.TaskId] = outputHandler
	s.handlerMutex.Unlock()

	logger.Info("Task submitted",
		zap.String("task_id", task.TaskId),
		zap.String("agent_id", task.AgentId),
		zap.String("type", task.Type.String()),
	)

	// Execute task asynchronously
	go s.executeTask(taskCtx, taskInfo)

	return nil
}

// executeTask executes a task on an agent
func (s *Scheduler) executeTask(ctx context.Context, taskInfo *TaskInfo) {
	task := taskInfo.Task

	// Update task status to running
	s.updateTaskStatus(task.TaskId, pb.TaskStatus_TASK_STATUS_RUNNING)

	// Check if agent uses stream communication
	agent, err := s.agentManager.GetAgent(task.AgentId)
	if err != nil {
		logger.Error("Failed to get agent",
			zap.String("task_id", task.TaskId),
			zap.Error(err),
		)
		s.handleTaskError(task.TaskId, err)
		return
	}

	if agent.UseStream {
		// Use stream-based communication
		if s.streamSender == nil {
			logger.Error("Stream sender not configured",
				zap.String("task_id", task.TaskId),
			)
			s.handleTaskError(task.TaskId, fmt.Errorf("stream sender not configured"))
			return
		}

		// Send task to agent via stream (fire-and-forget)
		// Outputs will come back asynchronously via HandleTaskOutput
		err := s.streamSender.SendTaskToAgent(task.AgentId, task)
		if err != nil {
			logger.Error("Failed to send task to agent via stream",
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
			s.handleTaskError(task.TaskId, err)
			return
		}

		logger.Info("Task sent to agent via stream",
			zap.String("task_id", task.TaskId),
			zap.String("agent_id", task.AgentId),
		)
		// Task will complete asynchronously via HandleTaskOutput callbacks
		return
	}

	// Legacy gRPC-based communication (deprecated)
	stream, err := s.agentManager.ExecuteTaskOnAgent(ctx, task.AgentId, task)
	if err != nil {
		logger.Error("Failed to execute task on agent",
			zap.String("task_id", task.TaskId),
			zap.Error(err),
		)
		s.handleTaskError(task.TaskId, err)
		return
	}

	// Receive and forward output
	for {
		output, err := stream.Recv()
		if err != nil {
			// Stream ended (successfully or with error)
			if err.Error() == "EOF" {
				logger.Info("Task stream ended",
					zap.String("task_id", task.TaskId),
				)
				s.completeTask(task.TaskId, pb.TaskStatus_TASK_STATUS_COMPLETED)
			} else {
				logger.Error("Task stream error",
					zap.String("task_id", task.TaskId),
					zap.Error(err),
				)
				s.handleTaskError(task.TaskId, err)
			}
			break
		}

		// Filter out message
		if !s.filterOutput(output) {
			// Forward output to client via handler
			s.forwardOutput(output)
		}

		// Check if task is completed or failed
		if output.Status == pb.TaskStatus_TASK_STATUS_COMPLETED {
			s.completeTask(task.TaskId, pb.TaskStatus_TASK_STATUS_COMPLETED)
			break
		} else if output.Status == pb.TaskStatus_TASK_STATUS_FAILED {
			s.completeTask(task.TaskId, pb.TaskStatus_TASK_STATUS_FAILED)
			break
		} else if output.Status == pb.TaskStatus_TASK_STATUS_CANCELLED {
			s.completeTask(task.TaskId, pb.TaskStatus_TASK_STATUS_CANCELLED)
			break
		}
	}
}

// HandleTaskOutput handles task output from stream (called by StreamHandler)
func (s *Scheduler) HandleTaskOutput(output *pb.TaskOutput) {
	if output == nil {
		return
	}

	taskID := output.TaskId

	// Filter out message
	if !s.filterOutput(output) {
		// Forward output to client via handler
		s.forwardOutput(output)
	}

	// Check if task is completed or failed
	if output.Status == pb.TaskStatus_TASK_STATUS_COMPLETED {
		s.completeTask(taskID, pb.TaskStatus_TASK_STATUS_COMPLETED)
	} else if output.Status == pb.TaskStatus_TASK_STATUS_FAILED {
		s.completeTask(taskID, pb.TaskStatus_TASK_STATUS_FAILED)
	} else if output.Status == pb.TaskStatus_TASK_STATUS_CANCELLED {
		s.completeTask(taskID, pb.TaskStatus_TASK_STATUS_CANCELLED)
	}
}

// CancelTask cancels a running task
func (s *Scheduler) CancelTask(taskID string) error {
	s.mutex.RLock()
	taskInfo, ok := s.tasks[taskID]
	s.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Cancel context
	if taskInfo.CancelFunc != nil {
		taskInfo.CancelFunc()
	}

	// Check if agent uses stream
	agent, err := s.agentManager.GetAgent(taskInfo.AgentID)
	if err == nil && agent.UseStream && s.streamSender != nil {
		// Use stream-based cancellation
		err := s.streamSender.CancelTaskOnAgent(taskInfo.AgentID, taskID)
		if err != nil {
			logger.Error("Failed to cancel task on agent via stream",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			// Continue anyway to clean up locally
		}
	} else {
		// Legacy gRPC-based cancellation
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := s.agentManager.CancelTaskOnAgent(ctx, taskInfo.AgentID, taskID)
		if err != nil {
			logger.Error("Failed to cancel task on agent",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
			// Continue anyway to clean up locally
		}
	}

	s.completeTask(taskID, pb.TaskStatus_TASK_STATUS_CANCELLED)

	logger.Info("Task cancelled",
		zap.String("task_id", taskID),
	)

	return nil
}

// updateTaskStatus updates the status of a task
func (s *Scheduler) updateTaskStatus(taskID string, status pb.TaskStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if taskInfo, ok := s.tasks[taskID]; ok {
		taskInfo.Status = status
	}
}

// completeTask marks a task as completed and cleans up resources
func (s *Scheduler) completeTask(taskID string, status pb.TaskStatus) {
	s.mutex.Lock()
	taskInfo, ok := s.tasks[taskID]
	if !ok {
		s.mutex.Unlock()
		return
	}

	taskInfo.Status = status

	// Decrement counters
	s.currentTasks--
	agentID := taskInfo.AgentID
	s.mutex.Unlock()

	// Decrement agent task count
	_ = s.agentManager.DecrementTaskCount(agentID)

	// Send completion notification to client BEFORE removing handler
	// (only if not already sent via forwardOutput, which happens in handleTaskError)
	s.handlerMutex.Lock()
	handler, ok := s.outputHandlers[taskID]
	if ok && handler != nil {
		// Send final status update to client
		// Note: For FAILED status, this is already sent by handleTaskError
		// For COMPLETED/CANCELLED, we need to send it here
		if status == pb.TaskStatus_TASK_STATUS_COMPLETED || status == pb.TaskStatus_TASK_STATUS_CANCELLED {
			handler(&pb.TaskOutput{
				TaskId: taskID,
				Status: status,
			})
		}
	}
	// Now remove the handler
	delete(s.outputHandlers, taskID)
	s.handlerMutex.Unlock()

	logger.Info("Task completed",
		zap.String("task_id", taskID),
		zap.String("status", status.String()),
	)
}

// handleTaskError handles task execution errors
func (s *Scheduler) handleTaskError(taskID string, err error) {
	// Send error output to client
	s.forwardOutput(&pb.TaskOutput{
		TaskId:       taskID,
		Timestamp:    timestamppb.New(time.Now()),
		Status:       pb.TaskStatus_TASK_STATUS_FAILED,
		ErrorMessage: err.Error(),
	})

	s.completeTask(taskID, pb.TaskStatus_TASK_STATUS_FAILED)
}

// forwardOutput forwards task output to the registered handler
func (s *Scheduler) forwardOutput(output *pb.TaskOutput) {
	s.handlerMutex.RLock()
	handler, ok := s.outputHandlers[output.TaskId]
	s.handlerMutex.RUnlock()

	if ok && handler != nil {
		handler(output)
	}
}

// filterOutput determines whether to filter out a given output message, returning true to filter it out
func (s *Scheduler) filterOutput(output *pb.TaskOutput) bool {
	if output == nil {
		return true
	}
	if strings.Contains(output.OutputLine, "MapTrace URL") ||
		strings.Contains(output.OutputLine, "NextTrace") ||
		strings.Contains(output.OutputLine, "IP Geo Data Provider") {
		return true
	}
	return false
}

// GetTask retrieves task information
func (s *Scheduler) GetTask(taskID string) (*TaskInfo, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	taskInfo, ok := s.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return taskInfo, nil
}

// GetCurrentTaskCount returns the current number of running tasks
func (s *Scheduler) GetCurrentTaskCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.currentTasks
}
