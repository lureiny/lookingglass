package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lureiny/lookingglass/pkg/logger"
	"go.uber.org/zap"
)

// BarkConfig holds configuration for Bark notifier
type BarkConfig struct {
	ServerURL string // Bark server URL (e.g., https://api.day.app/YOUR_KEY)
	DeviceKey string // Bark device key (optional if included in ServerURL)
	Sound     string // Notification sound (optional)
	Icon      string // Notification icon URL (optional)
	Group     string // Notification group (optional)
}

// BarkNotifier implements the Notifier interface for Bark
type BarkNotifier struct {
	config     *BarkConfig
	httpClient *http.Client
}

// BarkMessage represents a Bark API message
type BarkMessage struct {
	Title    string `json:"title"`              // Notification title
	Body     string `json:"body"`               // Notification body
	Sound    string `json:"sound,omitempty"`    // Sound name
	Icon     string `json:"icon,omitempty"`     // Icon URL
	Group    string `json:"group,omitempty"`    // Group name
	URL      string `json:"url,omitempty"`      // URL to open when tapped
	Level    string `json:"level,omitempty"`    // active, timeSensitive, passive
	Badge    int    `json:"badge,omitempty"`    // Badge number
	AutoCopy string `json:"autoCopy,omitempty"` // Auto copy content
}

// NewBarkNotifier creates a new Bark notifier
func NewBarkNotifier(config *BarkConfig) (*BarkNotifier, error) {
	if config == nil {
		return nil, fmt.Errorf("bark config is nil")
	}

	if config.ServerURL == "" && config.DeviceKey == "" {
		return nil, fmt.Errorf("bark server_url or device_key must be provided")
	}

	// Build server URL if device key is provided
	if config.DeviceKey != "" && config.ServerURL == "" {
		config.ServerURL = fmt.Sprintf("https://api.day.app/%s", config.DeviceKey)
	}

	return &BarkNotifier{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Name returns the name of this notifier
func (b *BarkNotifier) Name() string {
	return "Bark"
}

// Send sends a notification via Bark
func (b *BarkNotifier) Send(ctx context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	// Build Bark message
	message := &BarkMessage{
		Title: event.Title,
		Body:  event.Message,
		Group: b.config.Group,
		Icon:  b.config.Icon,
		Sound: b.config.Sound,
	}

	// Set level based on priority
	switch event.Priority {
	case 0:
		message.Level = "passive"
	case 2:
		message.Level = "timeSensitive"
	default:
		message.Level = "active"
	}

	// Add metadata to body if present
	if len(event.Metadata) > 0 {
		message.Body += "\n\n"
		for key, value := range event.Metadata {
			if key != "agent_id" { // Skip agent_id in display
				message.Body += fmt.Sprintf("%s: %s\n", key, value)
			}
		}
	}

	// Serialize message
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal bark message: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", b.config.ServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send bark notification: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("bark API returned status %d", resp.StatusCode)
	}

	logger.Debug("Bark notification sent successfully",
		zap.String("title", event.Title),
		zap.Int("status_code", resp.StatusCode),
	)

	return nil
}

// Close closes the Bark notifier
func (b *BarkNotifier) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
