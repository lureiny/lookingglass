package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/lureiny/lookingglass/agent/config"
	"github.com/lureiny/lookingglass/agent/executor"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
)

// TaskInfo contains task configuration and metadata
type TaskInfo struct {
	Name         string             // Task name (e.g., "ping", "curl_test")
	DisplayName  string             // Display name for frontend
	TaskType     pb.TaskType        // Protobuf TaskType enum
	ExecutorType string             // Executor type (e.g., "ping", "command")
	Config       *config.TaskConfig // Full task configuration
	Concurrency  int                // Max concurrent tasks
}

// Manager manages task lifecycle: configuration, concurrency, and execution
type Manager struct {
	// Task configurations
	tasks map[string]*TaskInfo
	mutex sync.RWMutex

	// Executor creation
	registry *executor.Registry

	// Runtime management
	runningTasks map[string]context.CancelFunc
	tasksMutex   sync.RWMutex

	// Concurrency control
	globalSemaphore chan struct{}
	taskSemaphores  map[string]chan struct{}
	semaphoreMutex  sync.RWMutex
}

// NewManager creates a new task manager
func NewManager(registry *executor.Registry, globalMaxConcurrent int) *Manager {
	return &Manager{
		tasks:           make(map[string]*TaskInfo),
		registry:        registry,
		runningTasks:    make(map[string]context.CancelFunc),
		globalSemaphore: make(chan struct{}, globalMaxConcurrent),
		taskSemaphores:  make(map[string]chan struct{}),
	}
}

// RegisterTask registers a task with its configuration
func (m *Manager) RegisterTask(info *TaskInfo) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if info.Name == "" {
		return fmt.Errorf("task name cannot be empty")
	}

	m.tasks[info.Name] = info
	return nil
}

// GetTask retrieves task info by name
func (m *Manager) GetTask(taskName string) (*TaskInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	task, ok := m.tasks[taskName]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskName)
	}

	return task, nil
}

// GetAllTasks returns all registered tasks
func (m *Manager) GetAllTasks() []*TaskInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tasks := make([]*TaskInfo, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// GetTaskNames returns all task names
func (m *Manager) GetTaskNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.tasks))
	for name := range m.tasks {
		names = append(names, name)
	}

	return names
}

// HasTask checks if a task exists
func (m *Manager) HasTask(taskName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, ok := m.tasks[taskName]
	return ok
}

// InitializeTaskSemaphores initializes concurrency semaphores for all registered tasks
func (m *Manager) InitializeTaskSemaphores() {
	m.semaphoreMutex.Lock()
	defer m.semaphoreMutex.Unlock()

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, taskInfo := range m.tasks {
		if taskInfo.Concurrency > 0 {
			m.taskSemaphores[taskInfo.Name] = make(chan struct{}, taskInfo.Concurrency)
			logger.Info("Task concurrency initialized",
				zap.String("task", taskInfo.Name),
				zap.Int("max_concurrent", taskInfo.Concurrency),
			)
		}
	}
}

// Execute executes a task by extracting task name from pb.Task
func (m *Manager) Execute(ctx context.Context, pbTask *pb.Task, outputChan chan<- *pb.TaskOutput) error {
	// Extract task name from pb.Task
	taskName, err := m.extractTaskName(pbTask)
	if err != nil {
		return err
	}

	// Get task info
	taskInfo, err := m.GetTask(taskName)
	if err != nil {
		return fmt.Errorf("task not found: %s", taskName)
	}

	// Create executor instance dynamically using registry
	exec, err := m.registry.Create(taskInfo.ExecutorType, taskInfo.Config)
	if err != nil {
		return fmt.Errorf("failed to create executor for task %s: %w", taskName, err)
	}

	// Acquire global semaphore (global concurrency control)
	select {
	case m.globalSemaphore <- struct{}{}:
		defer func() { <-m.globalSemaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Acquire per-task semaphore if configured (per-task concurrency control)
	m.semaphoreMutex.RLock()
	taskSemaphore, hasTaskLimit := m.taskSemaphores[taskName]
	m.semaphoreMutex.RUnlock()

	if hasTaskLimit {
		select {
		case taskSemaphore <- struct{}{}:
			defer func() { <-taskSemaphore }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Create cancellable context for this task
	taskCtx, cancel := context.WithCancel(ctx)

	// Store cancel function for task cancellation
	m.tasksMutex.Lock()
	m.runningTasks[pbTask.TaskId] = cancel
	m.tasksMutex.Unlock()

	// Clean up when done
	defer func() {
		m.tasksMutex.Lock()
		delete(m.runningTasks, pbTask.TaskId)
		m.tasksMutex.Unlock()
	}()

	// Execute the task
	logger.Info("Executing task",
		zap.String("task_id", pbTask.TaskId),
		zap.String("task_name", taskName),
		zap.String("executor_type", taskInfo.ExecutorType),
	)

	return exec.Execute(taskCtx, pbTask, outputChan)
}

// extractTaskName extracts task name from pb.Task
// Simply returns the task_name field directly
func (m *Manager) extractTaskName(pbTask *pb.Task) (string, error) {
	if pbTask.TaskName == "" {
		return "", fmt.Errorf("task_name is required but was empty")
	}
	return pbTask.TaskName, nil
}

// Cancel cancels a running task by task ID
func (m *Manager) Cancel(taskID string) error {
	m.tasksMutex.RLock()
	cancel, ok := m.runningTasks[taskID]
	m.tasksMutex.RUnlock()

	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	cancel()
	logger.Info("Task cancelled", zap.String("task_id", taskID))
	return nil
}

// GetCurrentTaskCount returns the number of currently running tasks
func (m *Manager) GetCurrentTaskCount() int {
	m.tasksMutex.RLock()
	defer m.tasksMutex.RUnlock()
	return len(m.runningTasks)
}
