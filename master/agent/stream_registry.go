package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	pb "github.com/lureiny/lookingglass/pb"
	"go.uber.org/zap"
)

// StreamRegistry manages agent streams and pending requests
type StreamRegistry struct {
	mu              sync.RWMutex
	agentStreams    map[string]pb.MasterService_AgentStreamServer // agentID -> stream
	pendingRequests map[string]chan *pb.AgentMessage               // requestID -> response channel
	logger          *zap.Logger
}

// NewStreamRegistry creates a new stream registry
func NewStreamRegistry(logger *zap.Logger) *StreamRegistry {
	return &StreamRegistry{
		agentStreams:    make(map[string]pb.MasterService_AgentStreamServer),
		pendingRequests: make(map[string]chan *pb.AgentMessage),
		logger:          logger,
	}
}

// RegisterAgentStream registers a new agent stream
// If agent is already registered, replaces the old stream (handles reconnection)
func (r *StreamRegistry) RegisterAgentStream(agentID string, stream pb.MasterService_AgentStreamServer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agentStreams[agentID]; exists {
		r.logger.Warn("Agent stream already exists, replacing with new stream (reconnection)",
			zap.String("agent_id", agentID),
		)
		// Replace the old stream with the new one
		// The old stream's goroutine will detect disconnection and clean up
	}

	r.agentStreams[agentID] = stream
	r.logger.Info("Agent stream registered",
		zap.String("agent_id", agentID),
	)
	return nil
}

// UnregisterAgentStream removes an agent stream
func (r *StreamRegistry) UnregisterAgentStream(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.agentStreams, agentID)
	r.logger.Info("Agent stream unregistered",
		zap.String("agent_id", agentID),
	)
}

// GetAgentStream returns the stream for a specific agent
func (r *StreamRegistry) GetAgentStream(agentID string) (pb.MasterService_AgentStreamServer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stream, exists := r.agentStreams[agentID]
	return stream, exists
}

// SendToAgent sends a message to a specific agent
func (r *StreamRegistry) SendToAgent(agentID string, msg *pb.MasterMessage) error {
	stream, exists := r.GetAgentStream(agentID)
	if !exists {
		return fmt.Errorf("agent %s not connected", agentID)
	}

	if err := stream.Send(msg); err != nil {
		r.logger.Error("Failed to send message to agent",
			zap.String("agent_id", agentID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// SendAndWaitForResponse sends a message and waits for response
func (r *StreamRegistry) SendAndWaitForResponse(ctx context.Context, agentID string, msg *pb.MasterMessage, timeout time.Duration) (*pb.AgentMessage, error) {
	// Create response channel
	responseChan := make(chan *pb.AgentMessage, 1)

	// Register pending request
	r.mu.Lock()
	r.pendingRequests[msg.RequestId] = responseChan
	r.mu.Unlock()

	// Cleanup on return
	defer func() {
		r.mu.Lock()
		delete(r.pendingRequests, msg.RequestId)
		r.mu.Unlock()
		close(responseChan)
	}()

	// Send message
	if err := r.SendToAgent(agentID, msg); err != nil {
		return nil, err
	}

	// Wait for response
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case resp := <-responseChan:
		return resp, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("timeout waiting for response from agent %s", agentID)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleResponse routes a response to the appropriate pending request
func (r *StreamRegistry) HandleResponse(msg *pb.AgentMessage) {
	r.mu.RLock()
	responseChan, exists := r.pendingRequests[msg.RequestId]
	r.mu.RUnlock()

	if !exists {
		r.logger.Warn("Received response for unknown request",
			zap.String("request_id", msg.RequestId),
		)
		return
	}

	// Send response (non-blocking)
	select {
	case responseChan <- msg:
	default:
		r.logger.Warn("Response channel full, dropping response",
			zap.String("request_id", msg.RequestId),
		)
	}
}

// IsAgentConnected checks if an agent is connected
func (r *StreamRegistry) IsAgentConnected(agentID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.agentStreams[agentID]
	return exists
}

// GetConnectedAgentCount returns the number of connected agents
func (r *StreamRegistry) GetConnectedAgentCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.agentStreams)
}
