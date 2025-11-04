package server

import (
	"context"
	"fmt"

	"github.com/lureiny/lookingglass/master/agent"
	"github.com/lureiny/lookingglass/pkg/logger"
	pb "github.com/lureiny/lookingglass/pb"
	"go.uber.org/zap"
)

// MasterServer implements the MasterService gRPC server
type MasterServer struct {
	pb.UnimplementedMasterServiceServer
	agentManager      *agent.Manager
	heartbeatInterval int32 // seconds
	streamHandler     *StreamHandler
}

// NewMasterServer creates a new master gRPC server
func NewMasterServer(agentManager *agent.Manager, heartbeatInterval int32, streamHandler *StreamHandler) *MasterServer {
	return &MasterServer{
		agentManager:      agentManager,
		heartbeatInterval: heartbeatInterval,
		streamHandler:     streamHandler,
	}
}

// Register handles agent registration requests
func (s *MasterServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	agentInfo := req.AgentInfo
	if agentInfo == nil {
		logger.Warn("Received registration request with nil agent info")
		return &pb.RegisterResponse{
			Success: false,
			Message: "Agent info is required",
		}, nil
	}

	logger.Info("Agent registration request",
		zap.String("id", agentInfo.Id),
		zap.String("name", agentInfo.Name),
		zap.String("location", agentInfo.Location),
	)

	// Register the agent
	err := s.agentManager.Register(agentInfo)
	if err != nil {
		logger.Error("Failed to register agent",
			zap.String("id", agentInfo.Id),
			zap.Error(err),
		)
		return &pb.RegisterResponse{
			Success: false,
			Message: fmt.Sprintf("Registration failed: %v", err),
		}, nil
	}

	logger.Info("Agent registered successfully",
		zap.String("id", agentInfo.Id),
		zap.String("name", agentInfo.Name),
	)

	return &pb.RegisterResponse{
		Success:           true,
		Message:           "Registration successful",
		HeartbeatInterval: s.heartbeatInterval,
	}, nil
}

// Heartbeat handles agent heartbeat requests
func (s *MasterServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	agentID := req.AgentId

	logger.Debug("Received heartbeat",
		zap.String("agent_id", agentID),
		zap.Int32("current_tasks", req.CurrentTasks),
	)

	// Update agent heartbeat
	err := s.agentManager.UpdateHeartbeat(agentID, int(req.CurrentTasks))
	if err != nil {
		logger.Warn("Failed to update heartbeat",
			zap.String("agent_id", agentID),
			zap.Error(err),
		)
		return &pb.HeartbeatResponse{
			Success: false,
			Message: fmt.Sprintf("Heartbeat failed: %v", err),
		}, nil
	}

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "Heartbeat received",
	}, nil
}

// AgentStream handles bidirectional stream communication with agents
func (s *MasterServer) AgentStream(stream pb.MasterService_AgentStreamServer) error {
	return s.streamHandler.AgentStream(stream)
}
