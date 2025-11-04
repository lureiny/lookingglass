package auth

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/lureiny/lookingglass/pkg/logger"
	pb "github.com/lureiny/lookingglass/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Authenticator handles agent authentication
type Authenticator interface {
	// Authenticate validates the incoming gRPC request
	Authenticate(ctx context.Context) error

	// UnaryInterceptor returns a gRPC unary interceptor for authentication
	UnaryInterceptor() grpc.UnaryServerInterceptor

	// StreamInterceptor returns a gRPC stream interceptor for authentication
	StreamInterceptor() grpc.StreamServerInterceptor
}

// Config represents authentication configuration
type Config struct {
	Mode        pb.AuthMode
	APIKey      string
	IPWhitelist []string
}

// authenticator implements the Authenticator interface
type authenticator struct {
	config *Config
	ipNets []*net.IPNet
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(config *Config) (Authenticator, error) {
	if config.Mode == pb.AuthMode_AUTH_MODE_UNSPECIFIED {
		return nil, fmt.Errorf("authentication mode must be specified")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	auth := &authenticator{
		config: config,
		ipNets: make([]*net.IPNet, 0),
	}

	// Parse IP whitelist if mode is IP_WHITELIST
	if config.Mode == pb.AuthMode_AUTH_MODE_IP_WHITELIST {
		if len(config.IPWhitelist) == 0 {
			return nil, fmt.Errorf("IP whitelist cannot be empty when using IP_WHITELIST mode")
		}

		for _, ipStr := range config.IPWhitelist {
			// Check if it's a CIDR
			if strings.Contains(ipStr, "/") {
				_, ipNet, err := net.ParseCIDR(ipStr)
				if err != nil {
					return nil, fmt.Errorf("invalid CIDR in whitelist: %s: %w", ipStr, err)
				}
				auth.ipNets = append(auth.ipNets, ipNet)
			} else {
				// Single IP address
				ip := net.ParseIP(ipStr)
				if ip == nil {
					return nil, fmt.Errorf("invalid IP in whitelist: %s", ipStr)
				}
				// Create a /32 (IPv4) or /128 (IPv6) network
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				_, ipNet, _ := net.ParseCIDR(fmt.Sprintf("%s/%d", ipStr, bits))
				auth.ipNets = append(auth.ipNets, ipNet)
			}
		}

		logger.Info("IP whitelist loaded",
			zap.Int("count", len(auth.ipNets)),
		)
	}

	return auth, nil
}

// Authenticate validates the incoming gRPC request
func (a *authenticator) Authenticate(ctx context.Context) error {
	// Extract API key from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	apiKeys := md.Get("x-api-key")
	if len(apiKeys) == 0 {
		return status.Error(codes.Unauthenticated, "missing API key")
	}

	apiKey := apiKeys[0]
	if apiKey != a.config.APIKey {
		logger.Warn("Invalid API key attempt")
		return status.Error(codes.Unauthenticated, "invalid API key")
	}

	// If IP whitelist mode, check client IP
	if a.config.Mode == pb.AuthMode_AUTH_MODE_IP_WHITELIST {
		p, ok := peer.FromContext(ctx)
		if !ok {
			return status.Error(codes.Internal, "failed to get peer info")
		}

		clientIP, _, err := net.SplitHostPort(p.Addr.String())
		if err != nil {
			return status.Error(codes.Internal, "failed to parse client address")
		}

		ip := net.ParseIP(clientIP)
		if ip == nil {
			return status.Error(codes.Internal, "failed to parse client IP")
		}

		// Check if IP is in whitelist
		allowed := false
		for _, ipNet := range a.ipNets {
			if ipNet.Contains(ip) {
				allowed = true
				break
			}
		}

		if !allowed {
			logger.Warn("IP not in whitelist",
				zap.String("client_ip", clientIP),
			)
			return status.Error(codes.PermissionDenied, "IP not in whitelist")
		}
	}

	return nil
}

// UnaryInterceptor returns a gRPC unary interceptor for authentication
func (a *authenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Authenticate the request
		if err := a.Authenticate(ctx); err != nil {
			return nil, err
		}

		// Call the handler
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream interceptor for authentication
func (a *authenticator) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Authenticate the request
		if err := a.Authenticate(ss.Context()); err != nil {
			return err
		}

		// Call the handler
		return handler(srv, ss)
	}
}
