package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lureiny/lookingglass/master/notifier"
	"github.com/lureiny/lookingglass/pkg/logger"
	pb "github.com/lureiny/lookingglass/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Agent represents a registered agent with its connection
type Agent struct {
	Info          *pb.AgentInfo
	Status        pb.AgentStatus
	LastHeartbeat time.Time
	CurrentTasks  int32
	GRPCClient    pb.AgentServiceClient // Deprecated: use stream instead
	GRPCConn      *grpc.ClientConn      // Deprecated: use stream instead
	UseStream     bool                   // If true, use stream communication
}

// AgentStatusChangeCallback is called when an agent's status changes
type AgentStatusChangeCallback func(agents []*Agent)

// Manager manages all registered agents
type Manager struct {
	agents                map[string]*Agent
	mutex                 sync.RWMutex
	heartbeatTimeout      time.Duration
	offlineCheckTicker    *time.Ticker
	stopChan              chan struct{}
	notifier              *notifier.Manager
	eventConfig           *notifier.EventConfig
	statusChangeCallbacks []AgentStatusChangeCallback
}

// NewManager creates a new agent manager
func NewManager(heartbeatTimeout time.Duration, offlineCheckInterval time.Duration) *Manager {
	m := &Manager{
		agents:           make(map[string]*Agent),
		heartbeatTimeout: heartbeatTimeout,
		stopChan:         make(chan struct{}),
	}

	// Start offline check routine
	m.offlineCheckTicker = time.NewTicker(offlineCheckInterval)
	go m.offlineCheckRoutine()

	return m
}

// SetNotifier sets the notification manager and event configuration
func (m *Manager) SetNotifier(n *notifier.Manager, cfg *notifier.EventConfig) {
	m.notifier = n
	m.eventConfig = cfg
}

// OnStatusChange registers a callback to be called when agent status changes
func (m *Manager) OnStatusChange(callback AgentStatusChangeCallback) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.statusChangeCallbacks = append(m.statusChangeCallbacks, callback)
}

// notifyStatusChange notifies all registered callbacks about status changes
// Note: This method should NOT be called while holding the mutex lock
func (m *Manager) notifyStatusChange() {
	// Get a snapshot of all agents
	agents := m.GetAllAgents()

	// Get callbacks snapshot
	m.mutex.RLock()
	callbacks := make([]AgentStatusChangeCallback, len(m.statusChangeCallbacks))
	copy(callbacks, m.statusChangeCallbacks)
	m.mutex.RUnlock()

	// Call all callbacks (without holding the lock to avoid deadlock)
	for _, callback := range callbacks {
		go callback(agents)
	}
}

// Register registers a new agent or updates an existing one (deprecated: use RegisterAgentFromStream)
func (m *Manager) Register(info *pb.AgentInfo) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	logger.Info("Registering agent",
		zap.String("id", info.Id),
		zap.String("name", info.Name),
		zap.String("host", info.Host),
	)

	// Check if agent already exists
	if existingAgent, ok := m.agents[info.Id]; ok {
		// Update existing agent info
		existingAgent.Info = info
		existingAgent.Status = pb.AgentStatus_AGENT_STATUS_ONLINE
		existingAgent.LastHeartbeat = time.Now()

		logger.Info("Agent re-registered",
			zap.String("id", info.Id),
		)
		return nil
	}

	// Create new agent
	agent := &Agent{
		Info:          info,
		Status:        pb.AgentStatus_AGENT_STATUS_ONLINE,
		LastHeartbeat: time.Now(),
		CurrentTasks:  0,
	}

	// Connect to agent's gRPC server
	conn, err := grpc.NewClient(info.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("Failed to connect to agent",
			zap.String("id", info.Id),
			zap.String("host", info.Host),
			zap.Error(err),
		)
		return fmt.Errorf("failed to connect to agent: %w", err)
	}

	agent.GRPCConn = conn
	agent.GRPCClient = pb.NewAgentServiceClient(conn)

	m.agents[info.Id] = agent

	// Log supported task names
	logger.Info("Agent registered successfully",
		zap.String("id", info.Id),
		zap.String("name", info.Name),
		zap.Strings("task_names", info.TaskNames),
	)

	return nil
}

// RegisterAgentFromStream registers an agent that uses stream communication
func (m *Manager) RegisterAgentFromStream(info *pb.AgentInfo) error {
	m.mutex.Lock()

	logger.Info("Registering stream-based agent",
		zap.String("id", info.Id),
		zap.String("name", info.Name),
	)

	// Check if agent already exists
	if existingAgent, ok := m.agents[info.Id]; ok {
		// Allow re-registration for stream mode (handles reconnection)
		// Check if status changed from offline to online
		wasOffline := existingAgent.Status == pb.AgentStatus_AGENT_STATUS_OFFLINE

		// Update existing agent info
		existingAgent.Info = info
		existingAgent.Status = pb.AgentStatus_AGENT_STATUS_ONLINE
		existingAgent.LastHeartbeat = time.Now()
		existingAgent.UseStream = true
		// Close old gRPC connection if exists
		if existingAgent.GRPCConn != nil {
			existingAgent.GRPCConn.Close()
			existingAgent.GRPCConn = nil
			existingAgent.GRPCClient = nil
		}
		logger.Info("Agent re-registered with stream",
			zap.String("id", info.Id),
			zap.Bool("was_offline", wasOffline),
		)
		m.mutex.Unlock()

		// If agent was offline and now online, notify status change
		if wasOffline {
			logger.Info("Agent came back online",
				zap.String("id", info.Id),
				zap.String("name", info.Name),
			)

			// Send agent online notification
			if m.notifier != nil && m.eventConfig != nil && m.eventConfig.AgentOnline {
				location := info.Location
				if location == "" {
					location = "Unknown"
				}
				event := notifier.NewAgentOnlineEvent(info.Id, info.Name, location)
				m.notifier.Notify(event)
			}

			// Notify status change callbacks
			m.notifyStatusChange()
		}

		return nil
	}

	// Create new agent for stream communication
	agent := &Agent{
		Info:          info,
		Status:        pb.AgentStatus_AGENT_STATUS_ONLINE,
		LastHeartbeat: time.Now(),
		CurrentTasks:  0,
		UseStream:     true,
	}

	m.agents[info.Id] = agent

	// Log supported task names
	// Log task display info
	taskNames := make([]string, len(info.TaskDisplayInfo))
	for i, taskInfo := range info.TaskDisplayInfo {
		taskNames[i] = taskInfo.TaskName
	}

	logger.Info("Stream-based agent registered successfully",
		zap.String("id", info.Id),
		zap.String("name", info.Name),
		zap.Strings("task_names", taskNames),
	)

	// Release lock before sending notifications
	m.mutex.Unlock()

	// Send agent online notification (only for new agents, not re-registration)
	if m.notifier != nil && m.eventConfig != nil && m.eventConfig.AgentOnline {
		location := info.Location
		if location == "" {
			location = "Unknown"
		}
		event := notifier.NewAgentOnlineEvent(info.Id, info.Name, location)
		m.notifier.Notify(event)
	}

	// Notify status change callbacks
	m.notifyStatusChange()

	return nil
}

// UpdateHeartbeat updates an agent's heartbeat timestamp
func (m *Manager) UpdateHeartbeat(agentID string, currentTasks int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.LastHeartbeat = time.Now()
	agent.CurrentTasks = int32(currentTasks)
	agent.Status = pb.AgentStatus_AGENT_STATUS_ONLINE

	logger.Debug("Heartbeat updated",
		zap.String("id", agentID),
		zap.Int("current_tasks", currentTasks),
	)

	return nil
}

// MarkAgentOffline marks a specific agent as offline
func (m *Manager) MarkAgentOffline(agentID string) {
	m.mutex.Lock()

	agent, ok := m.agents[agentID]
	if !ok {
		m.mutex.Unlock()
		return
	}

	if agent.Status == pb.AgentStatus_AGENT_STATUS_ONLINE {
		agent.Status = pb.AgentStatus_AGENT_STATUS_OFFLINE
		logger.Warn("Agent marked as offline",
			zap.String("id", agentID),
			zap.String("name", agent.Info.Name),
		)

		// Release lock before sending notifications
		m.mutex.Unlock()

		// Send agent offline notification
		if m.notifier != nil && m.eventConfig != nil && m.eventConfig.AgentOffline {
			location := agent.Info.Location
			if location == "" {
				location = "Unknown"
			}
			event := notifier.NewAgentOfflineEvent(agent.Info.Id, agent.Info.Name, location)
			m.notifier.Notify(event)
		}

		// Notify status change callbacks
		m.notifyStatusChange()
	} else {
		m.mutex.Unlock()
	}
}

// GetAgent retrieves an agent by ID
func (m *Manager) GetAgent(agentID string) (*Agent, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	return agent, nil
}

// GetAllAgents returns all registered agents
func (m *Manager) GetAllAgents() []*Agent {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agents := make([]*Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}

	return agents
}

// GetOnlineAgents returns all online agents
func (m *Manager) GetOnlineAgents() []*Agent {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Status == pb.AgentStatus_AGENT_STATUS_ONLINE {
			agents = append(agents, agent)
		}
	}

	return agents
}

// IncrementTaskCount increments the task count for an agent
func (m *Manager) IncrementTaskCount(agentID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	agent.CurrentTasks++
	return nil
}

// DecrementTaskCount decrements the task count for an agent
func (m *Manager) DecrementTaskCount(agentID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	if agent.CurrentTasks > 0 {
		agent.CurrentTasks--
	}

	return nil
}

// offlineCheckRoutine periodically checks for offline agents
func (m *Manager) offlineCheckRoutine() {
	for {
		select {
		case <-m.offlineCheckTicker.C:
			m.checkOfflineAgents()

		case <-m.stopChan:
			m.offlineCheckTicker.Stop()
			return
		}
	}
}

// checkOfflineAgents marks agents as offline if heartbeat timeout exceeded
func (m *Manager) checkOfflineAgents() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for id, agent := range m.agents {
		if agent.Status == pb.AgentStatus_AGENT_STATUS_ONLINE {
			if now.Sub(agent.LastHeartbeat) > m.heartbeatTimeout {
				agent.Status = pb.AgentStatus_AGENT_STATUS_OFFLINE
				logger.Warn("Agent marked as offline",
					zap.String("id", id),
					zap.String("name", agent.Info.Name),
					zap.Time("last_heartbeat", agent.LastHeartbeat),
				)

				// Send agent offline notification
				if m.notifier != nil && m.eventConfig != nil && m.eventConfig.AgentOffline {
					location := agent.Info.Location
					if location == "" {
						location = "Unknown"
					}
					event := notifier.NewAgentOfflineEvent(agent.Info.Id, agent.Info.Name, location)
					m.notifier.Notify(event)
				}
			}
		}
	}
}

// ExecuteTaskOnAgent executes a task on a specific agent
// Deprecated: This is the old gRPC-based method. For stream-based agents, use StreamHandler.SendTaskToAgent
// This method will be removed once all agents use stream communication
func (m *Manager) ExecuteTaskOnAgent(ctx context.Context, agentID string, task *pb.Task) (pb.AgentService_ExecuteTaskClient, error) {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return nil, err
	}

	if agent.Status != pb.AgentStatus_AGENT_STATUS_ONLINE {
		return nil, fmt.Errorf("agent is offline: %s", agentID)
	}

	if agent.UseStream {
		return nil, fmt.Errorf("agent uses stream communication, use StreamHandler.SendTaskToAgent instead")
	}

	// Execute task on agent via gRPC
	stream, err := agent.GRPCClient.ExecuteTask(ctx, &pb.ExecuteTaskRequest{
		Task: task,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to execute task on agent: %w", err)
	}

	return stream, nil
}

// CancelTaskOnAgent cancels a task on a specific agent
// Deprecated: This is the old gRPC-based method. For stream-based agents, use StreamHandler.CancelTaskOnAgent
// This method will be removed once all agents use stream communication
func (m *Manager) CancelTaskOnAgent(ctx context.Context, agentID string, taskID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}

	if agent.UseStream {
		return fmt.Errorf("agent uses stream communication, use StreamHandler.CancelTaskOnAgent instead")
	}

	resp, err := agent.GRPCClient.CancelTask(ctx, &pb.CancelTaskRequest{
		TaskId: taskID,
	})

	if err != nil {
		return fmt.Errorf("failed to cancel task on agent: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("agent failed to cancel task: %s", resp.Message)
	}

	return nil
}

// SupportsTask checks if an agent supports a specific task type
// DEPRECATED: Use SupportsTaskByName instead. Master should not validate tasks.
func (m *Manager) SupportsTask(agentID string, taskType pb.TaskType) (bool, error) {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return false, err
	}

	for _, supported := range agent.Info.SupportedTasks {
		if supported == taskType {
			return true, nil
		}
	}

	return false, nil
}

// SupportsTaskByName checks if an agent supports a specific task by name
// NOTE: In the new architecture, Master acts as pure forwarder and should not validate.
// This method is provided for informational purposes only.
func (m *Manager) SupportsTaskByName(agentID string, taskName string) (bool, error) {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return false, err
	}

	for _, supported := range agent.Info.TaskNames {
		if supported == taskName {
			return true, nil
		}
	}

	return false, nil
}

// GetAgentsSupportingTask returns all online agents that support a specific task type
// DEPRECATED: Use GetAgentsSupportingTaskByName instead
func (m *Manager) GetAgentsSupportingTask(taskType pb.TaskType) []*Agent {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Status != pb.AgentStatus_AGENT_STATUS_ONLINE {
			continue
		}

		for _, supported := range agent.Info.SupportedTasks {
			if supported == taskType {
				agents = append(agents, agent)
				break
			}
		}
	}

	return agents
}

// GetAgentsSupportingTaskByName returns all online agents that support a specific task by name
func (m *Manager) GetAgentsSupportingTaskByName(taskName string) []*Agent {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Status != pb.AgentStatus_AGENT_STATUS_ONLINE {
			continue
		}

		for _, supported := range agent.Info.TaskNames {
			if supported == taskName {
				agents = append(agents, agent)
				break
			}
		}
	}

	return agents
}

// Stop stops the agent manager
func (m *Manager) Stop() {
	logger.Info("Stopping agent manager")

	close(m.stopChan)

	// Close all agent connections
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, agent := range m.agents {
		if agent.GRPCConn != nil {
			if err := agent.GRPCConn.Close(); err != nil {
				logger.Error("Failed to close agent connection",
					zap.String("id", agent.Info.Id),
					zap.Error(err),
				)
			}
		}
	}
}
