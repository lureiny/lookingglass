package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lureiny/lookingglass/master/agent"
	"github.com/lureiny/lookingglass/master/task"
	pb "github.com/lureiny/lookingglass/pb"
	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add proper origin checking for production
		return true
	},
}

// BrandingInfo contains branding customization information
type BrandingInfo struct {
	SiteTitle  string `json:"site_title"`
	LogoURL    string `json:"logo_url"`
	LogoText   string `json:"logo_text"`
	Subtitle   string `json:"subtitle"`
	FooterText string `json:"footer_text"`
}

// Server handles WebSocket connections from frontend clients
type Server struct {
	agentManager *agent.Manager
	scheduler    *task.Scheduler
	clients      map[string]*Client
	clientsMutex sync.RWMutex
	branding     *BrandingInfo
}

// NewServer creates a new WebSocket server
func NewServer(agentManager *agent.Manager, scheduler *task.Scheduler, branding *BrandingInfo) *Server {
	return &Server{
		agentManager: agentManager,
		scheduler:    scheduler,
		clients:      make(map[string]*Client),
		branding:     branding,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade WebSocket", zap.Error(err))
		return
	}

	// Create client
	client := NewClient(conn, s)

	// Register client
	s.clientsMutex.Lock()
	s.clients[client.ID] = client
	s.clientsMutex.Unlock()

	fields := []zap.Field{
		zap.String("client_id", client.ID),
		zap.String("remote_addr", r.RemoteAddr),
	}

	xForwardFor := r.Header.Get("X-Forwarded-For")

	if len(xForwardFor) > 0 {
		fields = append(fields, zap.String("real_ip", xForwardFor))
	}

	logger.Info("New WebSocket client connected", fields...)

	// Start client handler
	go client.ReadMessages()
	go client.WriteMessages()
}

// HandleAgentList handles HTTP GET request for agent list
func (s *Server) HandleAgentList(w http.ResponseWriter, r *http.Request) {
	agents := s.agentManager.GetAllAgents()

	type AgentResponse struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Location      string `json:"location"`
		IPv4          string `json:"ipv4"`
		IPv6          string `json:"ipv6"`
		Status        string `json:"status"`
		CurrentTasks  int32  `json:"current_tasks"`
		MaxConcurrent int32  `json:"max_concurrent"`
	}

	response := make([]AgentResponse, 0, len(agents))
	for _, agent := range agents {
		status := "offline"
		if agent.Status == pb.AgentStatus_AGENT_STATUS_ONLINE {
			status = "online"
		}

		response = append(response, AgentResponse{
			ID:            agent.Info.Id,
			Name:          agent.Info.Name,
			Location:      agent.Info.Location,
			IPv4:          maskIPAddress(agent.Info.Ipv4, agent.Info.HideIp),
			IPv6:          maskIPAddress(agent.Info.Ipv6, agent.Info.HideIp),
			Status:        status,
			CurrentTasks:  agent.CurrentTasks,
			MaxConcurrent: agent.Info.MaxConcurrent,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"agents": response,
	})
}

// maskIPAddress masks IP addresses for privacy (supports both IPv4 and IPv6)
// IPv4: 127.0.0.1 -> 127.0.*.*
// IPv6: 2001:0db8:85a3:0000:0000:8a2e:0370:7334 -> 2001:0db8:85a3:****:****:****:****:****
func maskIPAddress(ip string, shouldMask bool) string {
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

// UnregisterClient removes a client from the server
func (s *Server) UnregisterClient(clientID string) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	delete(s.clients, clientID)

	logger.Info("Client disconnected",
		zap.String("client_id", clientID),
	)
}

// SendToClient sends a message to a specific client
func (s *Server) SendToClient(clientID string, message interface{}) error {
	s.clientsMutex.RLock()
	client, ok := s.clients[clientID]
	s.clientsMutex.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	return client.Send(message)
}

// BroadcastToAll sends a message to all connected clients
func (s *Server) BroadcastToAll(message interface{}) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	for _, client := range s.clients {
		_ = client.Send(message)
	}
}

// BroadcastAgentStatusUpdate broadcasts agent status update to all connected clients
func (s *Server) BroadcastAgentStatusUpdate(agents []*agent.Agent) {
	// Convert to AgentStatusInfo
	agentInfos := make([]*pb.AgentStatusInfo, 0, len(agents))
	for _, ag := range agents {
		ipv4 := maskIPAddress(ag.Info.Ipv4, ag.Info.HideIp)
		ipv6 := maskIPAddress(ag.Info.Ipv6, ag.Info.HideIp)

		agentInfos = append(agentInfos, &pb.AgentStatusInfo{
			Id:              ag.Info.Id,
			Name:            ag.Info.Name,
			Location:        ag.Info.Location,
			Ipv4:            ipv4,
			Ipv6:            ipv6,
			Status:          ag.Status,
			TaskDisplayInfo: ag.Info.TaskDisplayInfo, // New: using task_display_info
			CurrentTasks:    ag.CurrentTasks,
			MaxConcurrent:   ag.Info.MaxConcurrent,
			Provider:        ag.Info.Provider,
			Idc:             ag.Info.Idc,
			Description:     ag.Info.Description,
		})
	}

	// Create and broadcast update message
	response := &pb.WSResponse{
		Type:   pb.WSResponse_TYPE_AGENT_STATUS_UPDATE,
		Agents: agentInfos,
	}

	s.BroadcastToAll(response)

	logger.Debug("Broadcasted agent status update",
		zap.Int("agent_count", len(agentInfos)),
		zap.Int("client_count", len(s.clients)),
	)
}

// HandleBranding handles HTTP GET request for branding configuration
func (s *Server) HandleBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.branding)
}
