package server

import (
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/lureiny/lookingglass/master/agent"
	pb "github.com/lureiny/lookingglass/pb"
	"go.uber.org/zap"
)

// TaskOutputHandler interface for handling task outputs
type TaskOutputHandler interface {
	HandleTaskOutput(output *pb.TaskOutput)
}

// StreamHandler handles bidirectional agent streams
type StreamHandler struct {
	agentManager      *agent.Manager
	streamRegistry    *agent.StreamRegistry
	taskOutputHandler TaskOutputHandler
	logger            *zap.Logger
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(agentManager *agent.Manager, streamRegistry *agent.StreamRegistry, logger *zap.Logger) *StreamHandler {
	return &StreamHandler{
		agentManager:   agentManager,
		streamRegistry: streamRegistry,
		logger:         logger,
	}
}

// SetTaskOutputHandler sets the task output handler (typically the scheduler)
func (h *StreamHandler) SetTaskOutputHandler(handler TaskOutputHandler) {
	h.taskOutputHandler = handler
}

// AgentStream handles the bidirectional stream with an agent
func (h *StreamHandler) AgentStream(stream pb.MasterService_AgentStreamServer) error {
	var agentID string
	var registered bool

	// Cleanup on stream close
	defer func() {
		if registered && agentID != "" {
			h.streamRegistry.UnregisterAgentStream(agentID)
			h.agentManager.MarkAgentOffline(agentID)
			h.logger.Info("Agent stream closed",
				zap.String("agent_id", agentID),
			)
		}
	}()

	// Handle incoming messages from agent
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			h.logger.Info("Agent closed stream",
				zap.String("agent_id", agentID),
			)
			return nil
		}
		if err != nil {
			h.logger.Error("Stream receive error",
				zap.String("agent_id", agentID),
				zap.Error(err),
			)
			return err
		}

		// Handle message based on type
		switch msg.Type {
		case pb.AgentMessage_TYPE_REGISTER:
			if err := h.handleRegister(stream, msg); err != nil {
				h.logger.Error("Registration failed",
					zap.Error(err),
				)
				return err
			}
			agentID = msg.GetRegister().GetAgentInfo().GetId()
			registered = true

		case pb.AgentMessage_TYPE_HEARTBEAT:
			if err := h.handleHeartbeat(stream, msg); err != nil {
				h.logger.Error("Heartbeat handling failed",
					zap.String("agent_id", agentID),
					zap.Error(err),
				)
			}

		case pb.AgentMessage_TYPE_TASK_OUTPUT:
			h.handleTaskOutput(msg)

		case pb.AgentMessage_TYPE_TASK_COMPLETE:
			h.handleTaskComplete(msg)

		case pb.AgentMessage_TYPE_TASK_FAILED:
			h.handleTaskFailed(msg)

		default:
			h.logger.Warn("Unknown message type",
				zap.String("agent_id", agentID),
				zap.Int32("type", int32(msg.Type)),
			)
		}
	}
}

// handleRegister processes agent registration
func (h *StreamHandler) handleRegister(stream pb.MasterService_AgentStreamServer, msg *pb.AgentMessage) error {
	registerReq := msg.GetRegister()
	if registerReq == nil {
		return fmt.Errorf("missing registration data")
	}

	agentInfo := registerReq.GetAgentInfo()
	if agentInfo == nil {
		return fmt.Errorf("missing agent info")
	}

	agentID := agentInfo.GetId()
	h.logger.Info("Processing agent registration",
		zap.String("agent_id", agentID),
		zap.String("agent_name", agentInfo.GetName()),
	)

	// Check for duplicate registration
	if err := h.streamRegistry.RegisterAgentStream(agentID, stream); err != nil {
		// Send failure response
		response := &pb.MasterMessage{
			RequestId: msg.RequestId,
			Type:      pb.MasterMessage_TYPE_REGISTER_RESPONSE,
			Payload: &pb.MasterMessage_RegisterResponse{
				RegisterResponse: &pb.RegisterResponse{
					Success: false,
					Message: err.Error(),
				},
			},
		}
		stream.Send(response)
		return err
	}

	// Register agent in manager
	if err := h.agentManager.RegisterAgentFromStream(agentInfo); err != nil {
		h.streamRegistry.UnregisterAgentStream(agentID)
		response := &pb.MasterMessage{
			RequestId: msg.RequestId,
			Type:      pb.MasterMessage_TYPE_REGISTER_RESPONSE,
			Payload: &pb.MasterMessage_RegisterResponse{
				RegisterResponse: &pb.RegisterResponse{
					Success: false,
					Message: err.Error(),
				},
			},
		}
		stream.Send(response)
		return err
	}

	// Send success response
	response := &pb.MasterMessage{
		RequestId: msg.RequestId,
		Type:      pb.MasterMessage_TYPE_REGISTER_RESPONSE,
		Payload: &pb.MasterMessage_RegisterResponse{
			RegisterResponse: &pb.RegisterResponse{
				Success:           true,
				Message:           "Registration successful",
				HeartbeatInterval: 30, // TODO: get from config
			},
		},
	}

	if err := stream.Send(response); err != nil {
		h.logger.Error("Failed to send registration response",
			zap.String("agent_id", agentID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("Agent registered successfully",
		zap.String("agent_id", agentID),
	)

	return nil
}

// handleHeartbeat processes heartbeat messages
func (h *StreamHandler) handleHeartbeat(stream pb.MasterService_AgentStreamServer, msg *pb.AgentMessage) error {
	heartbeatReq := msg.GetHeartbeat()
	if heartbeatReq == nil {
		return fmt.Errorf("missing heartbeat data")
	}

	agentID := heartbeatReq.GetAgentId()

	// Update last heartbeat time
	h.agentManager.UpdateHeartbeat(agentID, int(heartbeatReq.GetCurrentTasks()))

	// Send acknowledgment
	response := &pb.MasterMessage{
		RequestId: msg.RequestId,
		Type:      pb.MasterMessage_TYPE_HEARTBEAT_RESPONSE,
		Payload: &pb.MasterMessage_HeartbeatResponse{
			HeartbeatResponse: &pb.HeartbeatResponse{
				Success: true,
				Message: "Heartbeat received",
			},
		},
	}

	return stream.Send(response)
}

// handleTaskOutput processes task output messages
func (h *StreamHandler) handleTaskOutput(msg *pb.AgentMessage) {
	output := msg.GetTaskOutput()
	if output == nil {
		h.logger.Warn("Received task output message without payload")
		return
	}

	h.logger.Debug("Received task output",
		zap.String("task_id", output.GetTaskId()),
	)

	// Forward to task scheduler for handling
	if h.taskOutputHandler != nil {
		h.taskOutputHandler.HandleTaskOutput(output)
	}
}

// handleTaskComplete processes task completion messages
func (h *StreamHandler) handleTaskComplete(msg *pb.AgentMessage) {
	output := msg.GetTaskOutput()
	if output == nil {
		h.logger.Warn("Received task complete message without payload")
		return
	}

	h.logger.Info("Task completed",
		zap.String("task_id", output.GetTaskId()),
	)

	// Forward to task scheduler for handling
	if h.taskOutputHandler != nil {
		h.taskOutputHandler.HandleTaskOutput(output)
	}
}

// handleTaskFailed processes task failure messages
func (h *StreamHandler) handleTaskFailed(msg *pb.AgentMessage) {
	output := msg.GetTaskOutput()
	if output == nil {
		h.logger.Warn("Received task failed message without payload")
		return
	}

	h.logger.Warn("Task failed",
		zap.String("task_id", output.GetTaskId()),
		zap.String("error", output.GetErrorMessage()),
	)

	// Forward to task scheduler for handling
	if h.taskOutputHandler != nil {
		h.taskOutputHandler.HandleTaskOutput(output)
	}
}

// SendTaskToAgent sends a task execution request to an agent
func (h *StreamHandler) SendTaskToAgent(agentID string, task *pb.Task) error {
	msg := &pb.MasterMessage{
		RequestId: uuid.New().String(),
		Type:      pb.MasterMessage_TYPE_EXECUTE_TASK,
		Payload: &pb.MasterMessage_ExecuteTask{
			ExecuteTask: &pb.ExecuteTaskRequest{
				Task: task,
			},
		},
	}

	return h.streamRegistry.SendToAgent(agentID, msg)
}

// CancelTaskOnAgent sends a task cancellation request to an agent
func (h *StreamHandler) CancelTaskOnAgent(agentID string, taskID string) error {
	msg := &pb.MasterMessage{
		RequestId: uuid.New().String(),
		Type:      pb.MasterMessage_TYPE_CANCEL_TASK,
		Payload: &pb.MasterMessage_CancelTask{
			CancelTask: &pb.CancelTaskRequest{
				TaskId: taskID,
			},
		},
	}

	return h.streamRegistry.SendToAgent(agentID, msg)
}
