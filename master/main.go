package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lureiny/lookingglass/master/agent"
	"github.com/lureiny/lookingglass/master/auth"
	"github.com/lureiny/lookingglass/master/config"
	"github.com/lureiny/lookingglass/master/notifier"
	"github.com/lureiny/lookingglass/master/server"
	"github.com/lureiny/lookingglass/master/task"
	"github.com/lureiny/lookingglass/master/ws"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	configPath = flag.String("config", "master/config.yaml", "path to configuration file")

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

	logger.Info("Starting LookingGlass Master",
		zap.Int("grpc_port", cfg.Server.GRPCPort),
		zap.Int("ws_port", cfg.Server.WSPort),
		zap.String("auth_mode", cfg.Auth.Mode),
	)

	// Create authenticator
	authConfig := &auth.Config{
		Mode:        cfg.GetAuthMode(),
		APIKey:      cfg.Auth.APIKey,
		IPWhitelist: cfg.Auth.IPWhitelist,
	}
	authenticator, err := auth.NewAuthenticator(authConfig)
	if err != nil {
		logger.Fatal("Failed to create authenticator", zap.Error(err))
	}

	// Create notification manager
	notificationManager := notifier.NewManager()

	// Configure notification providers if enabled
	if cfg.Notification.Enabled {
		logger.Info("Notification system enabled")

		// Initialize Bark notifier if configured
		if cfg.Notification.Bark != nil && (cfg.Notification.Bark.ServerURL != "" || cfg.Notification.Bark.DeviceKey != "") {
			barkConfig := &notifier.BarkConfig{
				ServerURL: cfg.Notification.Bark.ServerURL,
				DeviceKey: cfg.Notification.Bark.DeviceKey,
				Sound:     cfg.Notification.Bark.Sound,
				Icon:      cfg.Notification.Bark.Icon,
				Group:     cfg.Notification.Bark.Group,
			}
			barkNotifier, err := notifier.NewBarkNotifier(barkConfig)
			if err != nil {
				logger.Error("Failed to create Bark notifier", zap.Error(err))
			} else {
				notificationManager.RegisterNotifier(barkNotifier)
				logger.Info("Bark notifier registered")
			}
		}

		// Start notification manager
		notificationManager.Start()
	}

	// Create agent manager
	agentManager := agent.NewManager(
		time.Duration(cfg.Agent.HeartbeatTimeout)*time.Second,
		time.Duration(cfg.Agent.OfflineCheckInterval)*time.Second,
	)

	// Set notification manager for agent manager
	if cfg.Notification.Enabled {
		eventConfig := &notifier.EventConfig{
			AgentOnline:  cfg.Notification.Events.AgentOnline,
			AgentOffline: cfg.Notification.Events.AgentOffline,
			AgentError:   cfg.Notification.Events.AgentError,
			TaskFailed:   cfg.Notification.Events.TaskFailed,
		}
		agentManager.SetNotifier(notificationManager, eventConfig)
	}

	// Create stream registry for bidirectional agent streams
	streamRegistry := agent.NewStreamRegistry(logger.Get())

	// Create stream handler
	streamHandler := server.NewStreamHandler(agentManager, streamRegistry, logger.Get())

	// Create task scheduler
	scheduler := task.NewScheduler(
		agentManager,
		cfg.Concurrency.GlobalMax,
	)

	// Wire up scheduler and stream handler (bidirectional dependency)
	scheduler.SetStreamSender(streamHandler)
	streamHandler.SetTaskOutputHandler(scheduler)

	// Create gRPC server with authentication interceptors
	// 配置 Keepalive Enforcement Policy，允许在空闲时进行 PING，并设置最小 PING 间隔
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true, // 核心设置：对于长连接，必须允许无流 PING
	}

	// 配置服务器主动 PING 参数（可选，但推荐）
	kasp := keepalive.ServerParameters{
		Time:    60 * time.Second, // 服务器空闲 60 秒后发送 PING
		Timeout: 20 * time.Second,
	}
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authenticator.UnaryInterceptor()),
		grpc.StreamInterceptor(authenticator.StreamInterceptor()),
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	)

	masterServer := server.NewMasterServer(
		agentManager,
		int32(cfg.Agent.HeartbeatInterval),
		streamHandler,
	)
	pb.RegisterMasterServiceServer(grpcServer, masterServer)

	// Start gRPC server
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		logger.Fatal("Failed to listen on gRPC port",
			zap.Int("port", cfg.Server.GRPCPort),
			zap.Error(err),
		)
	}

	go func() {
		logger.Info("Starting gRPC server",
			zap.Int("port", cfg.Server.GRPCPort),
		)
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatal("Failed to serve gRPC", zap.Error(err))
		}
	}()

	// Create WebSocket server with branding configuration
	branding := &ws.BrandingInfo{
		SiteTitle:  cfg.Branding.SiteTitle,
		LogoURL:    cfg.Branding.LogoURL,
		LogoText:   cfg.Branding.LogoText,
		Subtitle:   cfg.Branding.Subtitle,
		FooterText: cfg.Branding.FooterText,
	}
	wsServer := ws.NewServer(agentManager, scheduler, branding)

	// Register agent status change callback to broadcast updates to WebSocket clients
	agentManager.OnStatusChange(wsServer.BroadcastAgentStatusUpdate)

	// Setup HTTP routes
	http.HandleFunc("/ws", wsServer.HandleWebSocket)
	http.HandleFunc("/api/agents", wsServer.HandleAgentList)
	http.HandleFunc("/api/branding", wsServer.HandleBranding)

	// Serve static files from web/ directory
	fs := http.FileServer(http.Dir("web"))
	http.Handle("/", fs)

	// Create HTTP server for graceful shutdown
	httpServer := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Server.WSPort),
	}

	// Start HTTP/WebSocket server
	go func() {
		logger.Info("Starting HTTP/WebSocket server",
			zap.Int("port", cfg.Server.WSPort),
			zap.String("static_dir", "web/"),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve HTTP/WebSocket", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down master...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Create a done channel to track shutdown completion
	done := make(chan struct{})

	go func() {
		// Shutdown HTTP server
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}

		// Shutdown other components
		agentManager.Stop()
		notificationManager.Stop()

		// Force stop gRPC server (agents will auto-reconnect after master restarts)
		// Using Stop() instead of GracefulStop() because:
		// 1. Agent streams are designed to auto-reconnect on disconnection
		// 2. GracefulStop() waits for all streams to complete, blocking shutdown
		// 3. Force disconnect is acceptable for master restart scenarios
		grpcServer.Stop()

		close(done)
	}()

	// Wait for shutdown to complete or timeout
	select {
	case <-done:
		logger.Info("Master stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("Shutdown timeout exceeded, forcing exit")
	}

	logger.Info("Master stopped")
}
