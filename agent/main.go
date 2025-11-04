package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lureiny/lookingglass/agent/client"
	"github.com/lureiny/lookingglass/agent/config"
	"github.com/lureiny/lookingglass/agent/executor"
	"github.com/lureiny/lookingglass/agent/task"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "agent/config.yaml", "path to configuration file")

	Version   = "dev"
	BuildTime = ""
)

func main() {
	if len(os.Args) <= 1 || os.Args[1] == "version" {
		fmt.Printf("LookingGlass\nVersion: %s\nBuild Time: %s\n", Version, BuildTime)
		return
	}

	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize unified logger
	if err := logger.Init(logger.Config{
		Level:   cfg.Log.Level,
		File:    cfg.Log.File,
		Console: cfg.Log.Console,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting LookingGlass Agent",
		zap.String("id", cfg.Agent.ID),
		zap.String("name", cfg.Agent.Name),
		zap.String("location", cfg.Agent.Metadata.Location),
	)

	// Create task manager with executor registry
	taskManager := task.NewManager(executor.GetGlobalRegistry(), cfg.Executor.GlobalConcurrency)

	// Collect task display info (task_name + display_name)
	taskDisplayInfo := []*pb.TaskDisplayInfo{}

	// Register tasks from configuration
	for taskName, taskCfg := range cfg.Executor.Tasks {
		// Skip disabled tasks
		if taskCfg.Enabled != nil && !*taskCfg.Enabled {
			logger.Info("Task disabled, skipping", zap.String("task", taskName))
			continue
		}

		// Determine executor type
		var executorType string

		switch taskName {
		case "ping":
			executorType = "ping"
		case "mtr":
			executorType = "mtr"
		case "nexttrace":
			executorType = "nexttrace"
		default:
			// Custom command task
			if taskCfg.Executor == nil || taskCfg.Executor.Path == "" {
				logger.Error("Custom task must specify executor path",
					zap.String("task", taskName),
				)
				continue
			}
			executorType = "command"
		}

		// Create TaskInfo (TaskType is deprecated, set to 0)
		taskInfo := &task.TaskInfo{
			Name:         taskName,
			DisplayName:  taskCfg.DisplayName,
			TaskType:     pb.TaskType_TASK_TYPE_UNSPECIFIED, // Deprecated field
			ExecutorType: executorType,
			Config:       taskCfg,
			Concurrency:  taskCfg.Concurrency.Max,
		}

		// Register task with task manager
		if err := taskManager.RegisterTask(taskInfo); err != nil {
			logger.Error("Failed to register task",
				zap.String("task", taskName),
				zap.Error(err),
			)
			continue
		}

		// Add task display info to list
		displayName := taskCfg.DisplayName
		if displayName == "" {
			displayName = taskName // Use task_name as fallback
		}

		// Determine if task requires target (default: true)
		requiresTarget := true
		if taskCfg.RequiresTarget != nil {
			requiresTarget = *taskCfg.RequiresTarget
		}

		taskDisplayInfo = append(taskDisplayInfo, &pb.TaskDisplayInfo{
			TaskName:       taskName,
			DisplayName:    displayName,
			Description:    "", // Could add description to config if needed
			RequiresTarget: requiresTarget,
		})

		logger.Info("Task registered",
			zap.String("task", taskName),
			zap.String("display_name", displayName),
			zap.String("executor_type", executorType),
			zap.Int("concurrency", taskCfg.Concurrency.Max),
		)
	}

	// Initialize per-task concurrency semaphores
	taskManager.InitializeTaskSemaphores()

	// Create stream-based master client
	streamClient := client.NewStreamClient(cfg, taskManager.GetCurrentTaskCount, taskDisplayInfo, taskManager)

	// Start stream client (with automatic reconnection)
	if err := streamClient.Start(); err != nil {
		logger.Fatal("Failed to start stream client", zap.Error(err))
	}

	logger.Info("Agent started in stream mode - no gRPC server listening")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down agent...")

	// Graceful shutdown
	streamClient.Stop()

	logger.Info("Agent stopped")
}
