package client

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	pb "github.com/lureiny/lookingglass/pb"
	"google.golang.org/protobuf/proto"
)

// Note: Request and Response messages are now defined in protobuf
// as pb.WSRequest and pb.WSResponse

// Client represents a WebSocket client for task execution
type Client struct {
	url    string
	conn   *websocket.Conn
	taskID string
}

// NewClient creates a new WebSocket client
func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

// Connect establishes WebSocket connection to master
func (c *Client) Connect() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to master: %w", err)
	}

	c.conn = conn
	return nil
}

// ExecuteTask sends a task execution request and streams the output
func (c *Client) ExecuteTask(ctx context.Context, task *pb.Task) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Create protobuf request
	req := &pb.WSRequest{
		Action: pb.WSRequest_ACTION_EXECUTE,
		Task:   task,
	}

	// Serialize to binary
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send binary message
	if err := c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return fmt.Errorf("failed to send task: %w", err)
	}

	c.taskID = task.TaskId

	// Setup signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Channel for WebSocket messages
	msgChan := make(chan *pb.WSResponse, 10)
	errChan := make(chan error, 1)

	// Start goroutine to read messages
	go func() {
		for {
			_, data, err := c.conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}

			var resp pb.WSResponse
			if err := proto.Unmarshal(data, &resp); err != nil {
				errChan <- fmt.Errorf("failed to unmarshal response: %w", err)
				return
			}
			msgChan <- &resp
		}
	}()

	// Process messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-sigChan:
			fmt.Println("\nReceived interrupt signal, cancelling task...")
			if err := c.cancelTask(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to cancel task: %v\n", err)
			}
			return fmt.Errorf("task cancelled by user")

		case err := <-errChan:
			return fmt.Errorf("websocket error: %w", err)

		case resp := <-msgChan:
			if err := c.handleResponse(resp); err != nil {
				return err
			}

			// Check if task is complete
			if resp.Type == pb.WSResponse_TYPE_COMPLETE {
				return nil
			}
			if resp.Type == pb.WSResponse_TYPE_ERROR {
				return fmt.Errorf("task error: %s", resp.Message)
			}
		}
	}
}

// handleResponse processes a task response
func (c *Client) handleResponse(resp *pb.WSResponse) error {
	switch resp.Type {
	case pb.WSResponse_TYPE_OUTPUT:
		// Print output line
		if resp.Output != "" {
			fmt.Println(resp.Output)
		}

		// Print error message if any
		if resp.Message != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Message)
		}

	case pb.WSResponse_TYPE_ERROR:
		return fmt.Errorf("error: %s", resp.Message)

	case pb.WSResponse_TYPE_COMPLETE:
		if resp.Message != "" {
			fmt.Println(resp.Message)
		}
		return nil

	case pb.WSResponse_TYPE_TASK_STARTED:
		// Task acknowledged, continue waiting for output
		return nil
	}

	return nil
}

// cancelTask sends a cancel request for the current task
func (c *Client) cancelTask() error {
	if c.conn == nil || c.taskID == "" {
		return nil
	}

	// Create protobuf cancel request
	req := &pb.WSRequest{
		Action: pb.WSRequest_ACTION_CANCEL,
		TaskId: c.taskID,
	}

	// Serialize to binary
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal cancel request: %w", err)
	}

	// Send binary message
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	if c.conn != nil {
		// Send close message
		err := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			return err
		}
		return c.conn.Close()
	}
	return nil
}
