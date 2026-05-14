package grpc

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ilindan-dev/dns-manager/internal/core/domain"
	pb "github.com/ilindan-dev/dns-manager/proto/dns/v1"
)

const bufSize = 1024 * 1024

// mockDNSManager implements the ports.DNSManager interface for deterministic tests.
type mockDNSManager struct {
	addErr    error
	removeErr error
	listRes   []string
	listErr   error
}

func (m *mockDNSManager) AddDNSServer(_ context.Context, _ string) error { return m.addErr }
func (m *mockDNSManager) RemoveDNSServer(_ context.Context, _ string) error {
	return m.removeErr
}

func (m *mockDNSManager) ListDNSServers(_ context.Context) ([]string, error) {
	return m.listRes, m.listErr
}

// setupTestServer starts a bufconn-backed gRPC server, registers NewServer(mockSvc),
// returns a pb.DNSManagerClient connected to it and a cleanup func (call via defer).
func setupTestServer(t *testing.T, mockSvc *mockDNSManager) (client pb.DNSManagerClient, closer func()) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()

	pb.RegisterDNSManagerServer(s, NewServer(mockSvc))

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client = pb.NewDNSManagerClient(conn)

	closer = func() {
		err := lis.Close()
		if err != nil {
			t.Logf("Error closing listener: %v", err)
		}
		s.Stop()
	}

	return client, closer
}

// TestServer_AddDNSServer - covers success, validation (empty address) and conflict.
func TestServer_AddDNSServer(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockSvc := &mockDNSManager{addErr: nil}
		client, closer := setupTestServer(t, mockSvc)
		defer closer()

		_, err := client.AddDNSServer(ctx, &pb.AddDNSServerRequest{Address: "8.8.8.8"})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	})

	t.Run("Empty Address", func(t *testing.T) {
		client, closer := setupTestServer(t, &mockDNSManager{})
		defer closer()

		_, err := client.AddDNSServer(ctx, &pb.AddDNSServerRequest{Address: ""})
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Fatalf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("Already Exists", func(t *testing.T) {
		mockSvc := &mockDNSManager{addErr: domain.ErrAlreadyExists}
		client, closer := setupTestServer(t, mockSvc)
		defer closer()

		_, err := client.AddDNSServer(ctx, &pb.AddDNSServerRequest{Address: "8.8.8.8"})
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.AlreadyExists {
			t.Fatalf("Expected AlreadyExists, got code %v", st.Code())
		}
	})
}

// TestServer_RemoveDNSServer - covers not-found mapping.
func TestServer_RemoveDNSServer(t *testing.T) {
	ctx := context.Background()

	t.Run("Not Found", func(t *testing.T) {
		mockSvc := &mockDNSManager{removeErr: domain.ErrNotFound}
		client, closer := setupTestServer(t, mockSvc)
		defer closer()

		_, err := client.RemoveDNSServer(ctx, &pb.RemoveDNSServerRequest{Address: "1.1.1.1"})
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Fatalf("Expected NotFound, got code %v", st.Code())
		}
	})
}

// TestServer_ListDNSServers - verifies addresses are returned intact.
func TestServer_ListDNSServers(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		expected := []string{"8.8.8.8", "1.1.1.1"}
		mockSvc := &mockDNSManager{listRes: expected}
		client, closer := setupTestServer(t, mockSvc)
		defer closer()

		resp, err := client.ListDNSServers(ctx, &emptypb.Empty{})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(resp.Addresses) != len(expected) {
			t.Fatalf("Expected %d addresses, got %d", len(expected), len(resp.Addresses))
		}
		for i, addr := range expected {
			if resp.Addresses[i] != addr {
				t.Errorf("Expected address %s, got %s", addr, resp.Addresses[i])
			}
		}
	})
}
