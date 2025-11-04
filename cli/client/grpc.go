package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"time"

	pb "github.com/lureiny/lookingglass/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// GRPCClient represents a direct gRPC client for agent testing
type GRPCClient struct {
	conn      *grpc.ClientConn
	client    pb.AgentServiceClient
	agentAddr string
	apiKey    string
	useTLS    bool
}

// NewGRPCClient creates a new gRPC client for direct agent connection
func NewGRPCClient(agentAddr string, apiKey string, useTLS bool) *GRPCClient {
	return &GRPCClient{
		agentAddr: agentAddr,
		apiKey:    apiKey,
		useTLS:    useTLS,
	}
}

// Connect establishes gRPC connection to agent
func (c *GRPCClient) Connect() error {
	var opts []grpc.DialOption

	if c.useTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // For testing; should validate in production
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add timeout for connection
	opts = append(opts, grpc.WithBlock())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, c.agentAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}

	c.conn = conn
	c.client = pb.NewAgentServiceClient(conn)
	return nil
}

// ExecuteTask executes a task on the agent and streams the output
func (c *GRPCClient) ExecuteTask(ctx context.Context, task *pb.Task, outputHandler func(string)) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	// Add API key to metadata if provided
	if c.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", c.apiKey)
	}

	// Create request
	req := &pb.ExecuteTaskRequest{
		Task: task,
	}

	// Call ExecuteTask RPC
	stream, err := c.client.ExecuteTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Read stream
	for {
		output, err := stream.Recv()
		if err == io.EOF {
			// Stream completed
			return nil
		}
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Handle output
		if output.OutputLine != "" {
			outputHandler(output.OutputLine)
		}

		// Check for errors
		if output.ErrorMessage != "" {
			return fmt.Errorf("task error: %s", output.ErrorMessage)
		}

		// Check status
		switch output.Status {
		case pb.TaskStatus_TASK_STATUS_FAILED:
			return fmt.Errorf("task failed: %s", output.ErrorMessage)
		case pb.TaskStatus_TASK_STATUS_CANCELLED:
			return fmt.Errorf("task cancelled")
		case pb.TaskStatus_TASK_STATUS_COMPLETED:
			return nil
		}
	}
}

// CancelTask cancels a running task
func (c *GRPCClient) CancelTask(ctx context.Context, taskID string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	// Add API key to metadata if provided
	if c.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", c.apiKey)
	}

	req := &pb.CancelTaskRequest{
		TaskId: taskID,
	}

	resp, err := c.client.CancelTask(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to cancel task: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("cancel failed: %s", resp.Message)
	}

	return nil
}

// HealthCheck performs a health check on the agent
func (c *GRPCClient) HealthCheck(ctx context.Context) (*pb.HealthCheckResponse, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Add API key to metadata if provided
	if c.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", c.apiKey)
	}

	req := &pb.HealthCheckRequest{
		Timestamp: nil, // Will be set by protobuf
	}

	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}

	return resp, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
