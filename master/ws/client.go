package ws

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	conn   *websocket.Conn
	server *Server
	send   chan interface{}
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, server *Server) *Client {
	return &Client{
		ID:     uuid.New().String(),
		conn:   conn,
		server: server,
		send:   make(chan interface{}, 256),
	}
}

// Note: RequestMessage and ResponseMessage are now defined in protobuf
// as pb.WSRequest and pb.WSResponse

// ReadMessages reads messages from the WebSocket connection
func (c *Client) ReadMessages() {
	defer func() {
		c.server.UnregisterClient(c.ID)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket read error", zap.Error(err))
			}
			break
		}

		c.handleMessage(message)
	}
}

// WriteMessages writes messages to the WebSocket connection
func (c *Client) WriteMessages() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Serialize protobuf message to binary
			resp, ok := message.(*pb.WSResponse)
			if !ok {
				logger.Error("Invalid message type in send channel")
				continue
			}

			data, err := proto.Marshal(resp)
			if err != nil {
				logger.Error("Failed to marshal response", zap.Error(err))
				return
			}

			if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				logger.Error("Failed to write message", zap.Error(err))
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send sends a message to the client
func (c *Client) Send(message interface{}) error {
	select {
	case c.send <- message:
		return nil
	default:
		return websocket.ErrCloseSent
	}
}

// handleMessage handles an incoming message from the client
func (c *Client) handleMessage(data []byte) {
	var req pb.WSRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		logger.Error("Failed to parse message", zap.Error(err))
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			Message: "Invalid message format",
		})
		return
	}

	logger.Debug("Received message",
		zap.String("action", req.Action.String()),
		zap.String("client_id", c.ID),
	)

	switch req.Action {
	case pb.WSRequest_ACTION_EXECUTE:
		c.handleExecute(&req)
	case pb.WSRequest_ACTION_CANCEL:
		c.handleCancel(&req)
	case pb.WSRequest_ACTION_LIST_AGENTS:
		c.handleListAgents(&req)
	default:
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			Message: "Unknown action: " + req.Action.String(),
		})
	}
}

// handleExecute handles task execution requests
func (c *Client) handleExecute(req *pb.WSRequest) {
	task := req.Task
	if task == nil {
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			Message: "task is required",
		})
		return
	}

	// Validate task_name is provided (basic request validation, not business logic)
	if task.TaskName == "" {
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			Message: "task_name is required",
		})
		return
	}

	// Output handler
	outputHandler := func(output *pb.TaskOutput) {
		// Check task status to determine response type
		var respType pb.WSResponse_Type

		switch output.Status {
		case pb.TaskStatus_TASK_STATUS_COMPLETED:
			respType = pb.WSResponse_TYPE_COMPLETE
		case pb.TaskStatus_TASK_STATUS_FAILED:
			respType = pb.WSResponse_TYPE_ERROR
		case pb.TaskStatus_TASK_STATUS_CANCELLED:
			respType = pb.WSResponse_TYPE_COMPLETE
		default:
			// RUNNING or PENDING status - regular output
			respType = pb.WSResponse_TYPE_OUTPUT
		}

		c.Send(&pb.WSResponse{
			Type:    respType,
			TaskId:  output.TaskId,
			Output:  output.OutputLine,
			Message: output.ErrorMessage,
		})
	}

	// Submit task
	ctx := context.Background()
	err := c.server.scheduler.SubmitTask(ctx, task, c.ID, outputHandler)
	if err != nil {
		logger.Error("Failed to submit task", zap.Error(err))
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			TaskId:  task.TaskId,
			Message: "submit task fail: " + err.Error(),
		})
		return
	}

	// Send acknowledgment
	c.Send(&pb.WSResponse{
		Type:   pb.WSResponse_TYPE_TASK_STARTED,
		TaskId: task.TaskId,
	})
}

// handleCancel handles task cancellation requests
func (c *Client) handleCancel(req *pb.WSRequest) {
	taskId := req.TaskId
	if taskId == "" {
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			Message: "task_id is required",
		})
		return
	}

	err := c.server.scheduler.CancelTask(taskId)
	if err != nil {
		c.Send(&pb.WSResponse{
			Type:    pb.WSResponse_TYPE_ERROR,
			TaskId:  taskId,
			Message: err.Error(),
		})
		return
	}

	c.Send(&pb.WSResponse{
		Type:    pb.WSResponse_TYPE_COMPLETE,
		TaskId:  taskId,
		Message: "Task cancelled successfully",
	})
}

// maskIP masks IP addresses for privacy
// IPv4: 127.0.0.1 -> 127.0.*.*
// IPv6: 2001:0db8:85a3:0000:0000:8a2e:0370:7334 -> 2001:0db8:85a3:****:****:****:****:****
func maskIP(ip string, shouldMask bool) string {
	if !shouldMask || ip == "" {
		return ip
	}

	// Check if it's IPv6 (contains colons)
	if strings.Contains(ip, ":") {
		return maskIPv6Address(ip)
	}

	// IPv4 handling
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		// Not a valid IPv4, return as-is
		return ip
	}

	// Mask last 2 octets
	return parts[0] + "." + parts[1] + ".*.*"
}

// handleListAgents handles agent list requests
func (c *Client) handleListAgents(req *pb.WSRequest) {
	// Get all agents from agent manager
	agents := c.server.agentManager.GetAllAgents()

	// Convert to AgentStatusInfo
	agentInfos := make([]*pb.AgentStatusInfo, 0, len(agents))
	for _, agent := range agents {
		ipv4 := maskIP(agent.Info.Ipv4, agent.Info.HideIp)
		ipv6 := maskIP(agent.Info.Ipv6, agent.Info.HideIp)

		agentInfos = append(agentInfos, &pb.AgentStatusInfo{
			Id:              agent.Info.Id,
			Name:            agent.Info.Name,
			Location:        agent.Info.Location,
			Ipv4:            ipv4,
			Ipv6:            ipv6,
			Status:          agent.Status,
			TaskDisplayInfo: agent.Info.TaskDisplayInfo, // New: using task_display_info
			CurrentTasks:    agent.CurrentTasks,
			MaxConcurrent:   agent.Info.MaxConcurrent,
			Provider:        agent.Info.Provider,
			Idc:             agent.Info.Idc,
			Description:     agent.Info.Description,
		})
	}

	// Send agent list response
	c.Send(&pb.WSResponse{
		Type:   pb.WSResponse_TYPE_AGENT_LIST,
		Agents: agentInfos,
	})

	logger.Debug("Sent agent list",
		zap.String("client_id", c.ID),
		zap.Int("agent_count", len(agentInfos)),
	)
}
