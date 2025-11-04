package client

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lureiny/lookingglass/agent/config"
	"github.com/lureiny/lookingglass/agent/task"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// StreamClient handles bidirectional stream communication with the master server
type StreamClient struct {
	config          *config.Config
	conn            *grpc.ClientConn
	client          pb.MasterServiceClient
	stream          pb.MasterService_AgentStreamClient
	streamMutex     sync.RWMutex
	taskCountFunc   func() int
	taskDisplayInfo []*pb.TaskDisplayInfo // Task display info (name + display_name)
	taskManager     *task.Manager

	// Reconnection management
	stopChan        chan struct{}
	connected       bool
	connectedMutex  sync.RWMutex
	backoffDuration time.Duration
	maxBackoff      time.Duration
	minBackoff      time.Duration

	// Heartbeat
	heartbeatTicker   *time.Ticker
	heartbeatInterval time.Duration
}

// NewStreamClient creates a new stream-based master client
func NewStreamClient(cfg *config.Config, taskCountFunc func() int, taskDisplayInfo []*pb.TaskDisplayInfo, taskMgr *task.Manager) *StreamClient {
	return &StreamClient{
		config:            cfg,
		stopChan:          make(chan struct{}),
		taskCountFunc:     taskCountFunc,
		taskDisplayInfo:   taskDisplayInfo,
		taskManager:       taskMgr,
		connected:         false,
		minBackoff:        1 * time.Second,
		maxBackoff:        60 * time.Second,
		backoffDuration:   1 * time.Second,
		heartbeatInterval: time.Duration(cfg.Master.HeartbeatInterval) * time.Second,
	}
}

// Start establishes the stream connection and starts the client
func (c *StreamClient) Start() error {
	logger.Info("Starting stream client")

	// Initial connection
	if err := c.connect(); err != nil {
		logger.Error("Initial connection failed", zap.Error(err))
		// Start reconnection loop anyway
	}

	// Start reconnection routine
	go c.reconnectionLoop()

	return nil
}

// connect establishes connection and stream to master
func (c *StreamClient) connect() error {
	logger.Info("Connecting to master server",
		zap.String("host", c.config.Master.Host),
	)

	// Setup dial options
	// 客户端将每隔 30 秒发送一次 PING 帧
	connParams := keepalive.ClientParameters{
		Time:                30 * time.Second, // PING 间隔时间
		Timeout:             10 * time.Second, // 等待服务器 PONG 的超时时间
		PermitWithoutStream: true,             // 允许在没有活动应用流时发送 PING
	}
	opts := []grpc.DialOption{grpc.WithKeepaliveParams(connParams)}

	if c.config.Master.TLSEnabled {
		// 使用tls
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(c.config.Master.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}

	c.conn = conn
	c.client = pb.NewMasterServiceClient(conn)

	// Create bidirectional stream with auth metadata
	ctx := c.createContextWithAuth(context.Background())
	stream, err := c.client.AgentStream(ctx)
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to create stream: %w", err)
	}

	c.streamMutex.Lock()
	c.stream = stream
	c.streamMutex.Unlock()

	logger.Info("Stream established with master")

	// Send registration message
	if err := c.sendRegistration(); err != nil {
		c.closeStream()
		return fmt.Errorf("failed to register: %w", err)
	}

	// Wait for registration response
	msg, err := stream.Recv()
	if err != nil {
		c.closeStream()
		return fmt.Errorf("failed to receive registration response: %w", err)
	}

	if msg.Type != pb.MasterMessage_TYPE_REGISTER_RESPONSE {
		c.closeStream()
		return fmt.Errorf("unexpected message type: %v", msg.Type)
	}

	resp := msg.GetRegisterResponse()
	if resp == nil || !resp.Success {
		c.closeStream()
		return fmt.Errorf("registration failed: %s", resp.GetMessage())
	}

	logger.Info("Agent registered successfully via stream",
		zap.String("message", resp.Message),
		zap.Int32("heartbeat_interval", resp.HeartbeatInterval),
	)

	// Update heartbeat interval if provided
	if resp.HeartbeatInterval > 0 {
		c.heartbeatInterval = time.Duration(resp.HeartbeatInterval) * time.Second
	}

	// Mark as connected and reset backoff
	c.setConnected(true)
	c.backoffDuration = c.minBackoff

	// Start heartbeat routine
	c.startHeartbeat()

	// Start message receive loop
	go c.receiveLoop()

	return nil
}

// sendRegistration sends a registration message to the master
func (c *StreamClient) sendRegistration() error {
	agentInfo := &pb.AgentInfo{
		Id:              c.config.Agent.ID,
		Name:            c.config.Agent.Name,
		Location:        c.config.Agent.Metadata.Location,
		Ipv4:            c.config.Agent.IPv4,
		Ipv6:            c.config.Agent.IPv6,
		Host:            "", // Not needed for stream-based communication
		MaxConcurrent:   int32(c.config.Agent.MaxConcurrent),
		TaskDisplayInfo: c.taskDisplayInfo, // Send task display info (name + display_name)
		HideIp:          c.config.Agent.HideIP,
		Provider:        c.config.Agent.Metadata.Provider,
		Idc:             c.config.Agent.Metadata.IDC,
		Description:     c.config.Agent.Metadata.Description,
	}

	msg := &pb.AgentMessage{
		RequestId: uuid.New().String(),
		Type:      pb.AgentMessage_TYPE_REGISTER,
		Payload: &pb.AgentMessage_Register{
			Register: &pb.RegisterRequest{
				AgentInfo: agentInfo,
			},
		},
	}

	return c.sendMessage(msg)
}

// startHeartbeat starts the heartbeat routine
func (c *StreamClient) startHeartbeat() {
	// Stop existing ticker if any
	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}

	c.heartbeatTicker = time.NewTicker(c.heartbeatInterval)

	logger.Info("Starting heartbeat",
		zap.Duration("interval", c.heartbeatInterval),
	)

	go func() {
		for {
			select {
			case <-c.heartbeatTicker.C:
				if c.isConnected() {
					if err := c.sendHeartbeat(); err != nil {
						logger.Error("Failed to send heartbeat", zap.Error(err))
						// Stream error will be handled by receiveLoop
					}
				}

			case <-c.stopChan:
				logger.Info("Stopping heartbeat")
				if c.heartbeatTicker != nil {
					c.heartbeatTicker.Stop()
				}
				return
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat message
func (c *StreamClient) sendHeartbeat() error {
	currentTasks := 0
	if c.taskCountFunc != nil {
		currentTasks = c.taskCountFunc()
	}

	msg := &pb.AgentMessage{
		RequestId: uuid.New().String(),
		Type:      pb.AgentMessage_TYPE_HEARTBEAT,
		Payload: &pb.AgentMessage_Heartbeat{
			Heartbeat: &pb.HeartbeatRequest{
				AgentId:      c.config.Agent.ID,
				CurrentTasks: int32(currentTasks),
			},
		},
	}

	logger.Debug("Sending heartbeat",
		zap.Int("current_tasks", currentTasks),
	)

	return c.sendMessage(msg)
}

// receiveLoop continuously receives messages from the master
func (c *StreamClient) receiveLoop() {
	logger.Info("Starting receive loop")

	for {
		select {
		case <-c.stopChan:
			logger.Info("Receive loop stopped")
			return

		default:
			c.streamMutex.RLock()
			stream := c.stream
			c.streamMutex.RUnlock()

			if stream == nil {
				logger.Warn("Stream is nil, waiting for reconnection")
				time.Sleep(1 * time.Second)
				continue
			}

			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					logger.Info("Stream closed by master")
				} else {
					logger.Error("Stream receive error", zap.Error(err))
				}
				c.handleStreamError()
				return
			}

			// Handle message
			go c.handleMasterMessage(msg)
		}
	}
}

// handleMasterMessage processes messages from the master
func (c *StreamClient) handleMasterMessage(msg *pb.MasterMessage) {
	logger.Debug("Received message from master",
		zap.String("type", msg.Type.String()),
		zap.String("request_id", msg.RequestId),
	)

	switch msg.Type {
	case pb.MasterMessage_TYPE_HEARTBEAT_RESPONSE:
		// Heartbeat acknowledgment, no action needed
		logger.Debug("Heartbeat acknowledged")

	case pb.MasterMessage_TYPE_EXECUTE_TASK:
		c.handleExecuteTask(msg)

	case pb.MasterMessage_TYPE_CANCEL_TASK:
		c.handleCancelTask(msg)

	default:
		logger.Warn("Unknown message type from master",
			zap.Int32("type", int32(msg.Type)),
		)
	}
}

// handleExecuteTask processes task execution requests
func (c *StreamClient) handleExecuteTask(msg *pb.MasterMessage) {
	req := msg.GetExecuteTask()
	if req == nil || req.Task == nil {
		logger.Error("Invalid execute task request")
		return
	}

	task := req.Task
	logger.Info("Received task execution request",
		zap.String("task_id", task.TaskId),
		zap.String("type", task.Type.String()),
	)

	// Create output channel
	outputChan := make(chan *pb.TaskOutput, 100)

	// Start task execution in background
	go func() {
		defer close(outputChan)

		ctx := context.Background()
		if err := c.taskManager.Execute(ctx, task, outputChan); err != nil {
			logger.Error("Task execution failed",
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
			// Send error message
			c.sendTaskOutput(&pb.TaskOutput{
				TaskId:       task.TaskId,
				Status:       pb.TaskStatus_TASK_STATUS_FAILED,
				ErrorMessage: err.Error(),
			})
		}
	}()

	// Forward output to master
	for output := range outputChan {
		if err := c.sendTaskOutput(output); err != nil {
			logger.Error("Failed to send task output",
				zap.String("task_id", task.TaskId),
				zap.Error(err),
			)
			break
		}
	}
}

// handleCancelTask processes task cancellation requests
func (c *StreamClient) handleCancelTask(msg *pb.MasterMessage) {
	req := msg.GetCancelTask()
	if req == nil {
		logger.Error("Invalid cancel task request")
		return
	}

	logger.Info("Received task cancellation request",
		zap.String("task_id", req.TaskId),
	)

	if err := c.taskManager.Cancel(req.TaskId); err != nil {
		logger.Error("Failed to cancel task",
			zap.String("task_id", req.TaskId),
			zap.Error(err),
		)
	}
}

// sendTaskOutput sends task output to the master
func (c *StreamClient) sendTaskOutput(output *pb.TaskOutput) error {
	var msgType pb.AgentMessage_Type
	switch output.Status {
	case pb.TaskStatus_TASK_STATUS_COMPLETED:
		msgType = pb.AgentMessage_TYPE_TASK_COMPLETE
	case pb.TaskStatus_TASK_STATUS_FAILED:
		msgType = pb.AgentMessage_TYPE_TASK_FAILED
	default:
		msgType = pb.AgentMessage_TYPE_TASK_OUTPUT
	}

	msg := &pb.AgentMessage{
		RequestId: uuid.New().String(),
		Type:      msgType,
		Payload: &pb.AgentMessage_TaskOutput{
			TaskOutput: output,
		},
	}

	return c.sendMessage(msg)
}

// sendMessage sends a message to the master via the stream
func (c *StreamClient) sendMessage(msg *pb.AgentMessage) error {
	c.streamMutex.RLock()
	stream := c.stream
	c.streamMutex.RUnlock()

	if stream == nil {
		return fmt.Errorf("stream is not connected")
	}

	return stream.Send(msg)
}

// handleStreamError handles stream errors and triggers reconnection
func (c *StreamClient) handleStreamError() {
	logger.Warn("Stream error detected, triggering reconnection")
	c.setConnected(false)
	c.closeStream()
}

// reconnectionLoop handles automatic reconnection with exponential backoff
func (c *StreamClient) reconnectionLoop() {
	for {
		select {
		case <-c.stopChan:
			logger.Info("Reconnection loop stopped")
			return

		default:
			if !c.isConnected() {
				logger.Info("Attempting to reconnect",
					zap.Duration("backoff", c.backoffDuration),
				)

				time.Sleep(c.backoffDuration)

				if err := c.connect(); err != nil {
					logger.Error("Reconnection failed", zap.Error(err))
					// Increase backoff duration (exponential backoff)
					c.backoffDuration *= 2
					if c.backoffDuration > c.maxBackoff {
						c.backoffDuration = c.maxBackoff
					}
				} else {
					logger.Info("Reconnected successfully")
				}
			} else {
				// Sleep a bit to avoid busy waiting
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// closeStream safely closes the stream and connection
func (c *StreamClient) closeStream() {
	c.streamMutex.Lock()
	defer c.streamMutex.Unlock()

	if c.stream != nil {
		c.stream.CloseSend()
		c.stream = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// isConnected returns whether the client is currently connected
func (c *StreamClient) isConnected() bool {
	c.connectedMutex.RLock()
	defer c.connectedMutex.RUnlock()
	return c.connected
}

// setConnected sets the connection status
func (c *StreamClient) setConnected(connected bool) {
	c.connectedMutex.Lock()
	defer c.connectedMutex.Unlock()

	if c.connected != connected {
		logger.Info("Connection status changed",
			zap.Bool("old", c.connected),
			zap.Bool("new", connected),
		)
	}

	c.connected = connected
}

// Stop stops the client and closes all connections
func (c *StreamClient) Stop() {
	logger.Info("Stopping stream client")

	close(c.stopChan)

	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}

	c.closeStream()
}

// createContextWithAuth creates a context with API key in metadata
func (c *StreamClient) createContextWithAuth(ctx context.Context) context.Context {
	md := metadata.New(map[string]string{
		"x-api-key": c.config.Master.APIKey,
	})
	return metadata.NewOutgoingContext(ctx, md)
}
