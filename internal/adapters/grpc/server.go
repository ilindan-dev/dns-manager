// Package grpc provides a gRPC adapter for the DNS manager service.
// It implements the generated protobuf server (pb.DNSManagerServer),
// maps core domain errors to gRPC status codes, and supplies a small
// client wrapper used by tests and callers.
package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ilindan-dev/dns-manager/internal/core/domain"
	"github.com/ilindan-dev/dns-manager/internal/core/ports"
	pb "github.com/ilindan-dev/dns-manager/proto/dns/v1"
)

// Compile check that Server implements pb.DNSManagerServer.
var _ pb.DNSManagerServer = (*Server)(nil)

// Server implements pb.DNSManagerServer and delegates requests to ports.DNSManager.
// It performs request validation and translates domain errors to gRPC status codes.
type Server struct {
	pb.UnimplementedDNSManagerServer
	svc ports.DNSManager
}

// NewServer returns a Server that delegates business logic to the provided DNSManager.
func NewServer(svc ports.DNSManager) *Server {
	return &Server{
		svc: svc,
	}
}

// AddDNSServer validates the request and calls svc.AddDNSServer.
// Returns InvalidArgument for empty address; domain errors are mapped to gRPC codes.
func (s *Server) AddDNSServer(ctx context.Context, req *pb.AddDNSServerRequest) (*emptypb.Empty, error) {
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	err := s.svc.AddDNSServer(ctx, req.Address)
	if err != nil {
		return nil, mapError(err)
	}

	return &emptypb.Empty{}, nil
}

// RemoveDNSServer validates the request and calls svc.RemoveDNSServer.
// Returns InvalidArgument for empty address; domain errors are mapped to gRPC codes.
func (s *Server) RemoveDNSServer(ctx context.Context, req *pb.RemoveDNSServerRequest) (*emptypb.Empty, error) {
	if req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	err := s.svc.RemoveDNSServer(ctx, req.Address)
	if err != nil {
		return nil, mapError(err)
	}

	return &emptypb.Empty{}, nil
}

// ListDNSServers validates the request and calls svc.ListDNSServers.
// Returns domain errors are mapped to gRPC codes.
func (s *Server) ListDNSServers(ctx context.Context, _ *emptypb.Empty) (*pb.ListDNSServersResponse, error) {
	servers, err := s.svc.ListDNSServers(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	return &pb.ListDNSServersResponse{
		Addresses: servers,
	}, nil
}

// mapError converts known domain errors into appropriate gRPC status codes,
// returning user-friendly messages for client responses.
func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidIP):
		return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Errorf(codes.AlreadyExists, "conflict: %v", err)
	case errors.Is(err, domain.ErrNotFound):
		return status.Errorf(codes.NotFound, "not found: %v", err)
	case errors.Is(err, domain.ErrPermissionDenied):
		return status.Errorf(codes.PermissionDenied, "system error: %v", err)
	case errors.Is(err, domain.ErrIO):
		return status.Error(codes.Internal, "internal server error while accessing resolver config")
	default:
		return status.Errorf(codes.Unknown, "unknown error: %v", err)
	}
}
