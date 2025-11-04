package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
)

// EventType represents the type of notification event
type EventType string

const (
	EventAgentOnline  EventType = "agent_online"
	EventAgentOffline EventType = "agent_offline"
	EventAgentError   EventType = "agent_error"
	EventTaskFailed   EventType = "task_failed"
)

// Event represents a notification event
type Event struct {
	Type      EventType         // Event type
	Title     string            // Notification title
	Message   string            // Notification message
	Timestamp time.Time         // Event timestamp
	Metadata  map[string]string // Additional metadata (agent_id, agent_name, etc.)
	Priority  int               // Priority level (0=low, 1=normal, 2=high)
}

// Notifier is the interface that all notification providers must implement
type Notifier interface {
	// Name returns the name of the notifier
	Name() string

	// Send sends a notification
	Send(ctx context.Context, event *Event) error

	// Close closes the notifier and releases resources
	Close() error
}

// EventConfig defines which events should trigger notifications
type EventConfig struct {
	AgentOnline  bool
	AgentOffline bool
	AgentError   bool
	TaskFailed   bool
}

// Manager manages multiple notification providers
type Manager struct {
	notifiers []Notifier
	enabled   bool
	eventChan chan *Event
	stopChan  chan struct{}
}

// NewManager creates a new notification manager
func NewManager() *Manager {
	return &Manager{
		notifiers: make([]Notifier, 0),
		enabled:   false,
		eventChan: make(chan *Event, 100), // Buffer for 100 events
		stopChan:  make(chan struct{}),
	}
}

// RegisterNotifier registers a notification provider
func (m *Manager) RegisterNotifier(notifier Notifier) {
	if notifier == nil {
		logger.Warn("Attempted to register nil notifier")
		return
	}

	m.notifiers = append(m.notifiers, notifier)
	logger.Info("Notifier registered", zap.String("name", notifier.Name()))
}

// Start starts the notification manager
func (m *Manager) Start() {
	if len(m.notifiers) == 0 {
		logger.Info("No notifiers registered, notification system disabled")
		m.enabled = false
		return
	}

	m.enabled = true
	logger.Info("Starting notification manager", zap.Int("notifiers", len(m.notifiers)))

	go m.processEvents()
}

// Stop stops the notification manager
func (m *Manager) Stop() {
	if !m.enabled {
		return
	}

	logger.Info("Stopping notification manager")
	close(m.stopChan)

	// Close all notifiers
	for _, notifier := range m.notifiers {
		if err := notifier.Close(); err != nil {
			logger.Error("Failed to close notifier",
				zap.String("notifier", notifier.Name()),
				zap.Error(err),
			)
		}
	}
}

// Notify sends a notification event
func (m *Manager) Notify(event *Event) {
	if !m.enabled {
		return
	}

	if event == nil {
		logger.Warn("Attempted to send nil event")
		return
	}

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Send to event channel (non-blocking)
	select {
	case m.eventChan <- event:
		// Event queued successfully
	default:
		logger.Warn("Event channel full, dropping notification",
			zap.String("type", string(event.Type)),
			zap.String("title", event.Title),
		)
	}
}

// processEvents processes notification events in a separate goroutine
func (m *Manager) processEvents() {
	for {
		select {
		case event := <-m.eventChan:
			m.sendToNotifiers(event)

		case <-m.stopChan:
			logger.Info("Notification manager stopped")
			return
		}
	}
}

// sendToNotifiers sends an event to all registered notifiers
func (m *Manager) sendToNotifiers(event *Event) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.Send(ctx, event); err != nil {
				logger.Error("Failed to send notification",
					zap.String("notifier", n.Name()),
					zap.String("event_type", string(event.Type)),
					zap.Error(err),
				)
			} else {
				logger.Debug("Notification sent",
					zap.String("notifier", n.Name()),
					zap.String("event_type", string(event.Type)),
					zap.String("title", event.Title),
				)
			}
		}(notifier)
	}
}

// Helper functions to create common events

// NewAgentOnlineEvent creates an agent online event
func NewAgentOnlineEvent(agentID, agentName, location string) *Event {
	return &Event{
		Type:     EventAgentOnline,
		Title:    fmt.Sprintf("Agent Online: %s", agentName),
		Message:  fmt.Sprintf("Agent '%s' (%s) is now online", agentName, location),
		Priority: 1,
		Metadata: map[string]string{
			"agent_id":   agentID,
			"agent_name": agentName,
			"location":   location,
		},
	}
}

// NewAgentOfflineEvent creates an agent offline event
func NewAgentOfflineEvent(agentID, agentName, location string) *Event {
	return &Event{
		Type:     EventAgentOffline,
		Title:    fmt.Sprintf("Agent Offline: %s", agentName),
		Message:  fmt.Sprintf("Agent '%s' (%s) went offline", agentName, location),
		Priority: 2, // Higher priority for offline events
		Metadata: map[string]string{
			"agent_id":   agentID,
			"agent_name": agentName,
			"location":   location,
		},
	}
}

// NewAgentErrorEvent creates an agent error event
func NewAgentErrorEvent(agentID, agentName, errorMsg string) *Event {
	return &Event{
		Type:     EventAgentError,
		Title:    fmt.Sprintf("Agent Error: %s", agentName),
		Message:  fmt.Sprintf("Agent '%s' encountered an error: %s", agentName, errorMsg),
		Priority: 2,
		Metadata: map[string]string{
			"agent_id":   agentID,
			"agent_name": agentName,
			"error":      errorMsg,
		},
	}
}
