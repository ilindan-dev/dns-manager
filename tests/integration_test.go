// Package tests contains integration tests that exercise the full gRPC
// stack against a temporary resolv.conf file. Tests start an in-process
// gRPC server and use the client adapter to perform end-to-end operations.
package tests

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dnsgrpc "github.com/ilindan-dev/dns-manager/internal/adapters/grpc"
	"github.com/ilindan-dev/dns-manager/internal/adapters/resolvconf"
	"github.com/ilindan-dev/dns-manager/internal/service"
	pb "github.com/ilindan-dev/dns-manager/proto/dns/v1"
)

// setupTestEnvironment prepares a temporary resolv.conf, starts a gRPC server
// bound to a random local port and returns a connected client together with
// a cleanup function. The cleanup function stops the server and closes the client.
func setupTestEnvironment(t *testing.T) (client *dnsgrpc.Client, closer func()) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "resolv.conf")

	initialContent := []byte("# This file is managed by test\nsearch localdomain\n")
	if err := os.WriteFile(configPath, initialContent, 0o644); err != nil {
		t.Fatalf("failed to create initial resolv.conf: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := resolvconf.NewFileAdapter(configPath, logger)
	svc := service.NewDNSManagerService(adapter, logger)

	grpcServer := grpc.NewServer()
	pb.RegisterDNSManagerServer(grpcServer, dnsgrpc.NewServer(svc))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			panic(err)
		}
	}()

	time.Sleep(10 * time.Millisecond)

	client, err = dnsgrpc.NewClient(lis.Addr().String())
	if err != nil {
		grpcServer.Stop()
		t.Fatalf("failed to create client: %v", err)
	}

	closer = func() {
		_ = client.Close()
		grpcServer.Stop()
	}

	return client, closer
}

// assertList fetches the list of configured DNS servers using the client
// and fails the test if the result does not exactly match the expected slice.
func assertList(ctx context.Context, t *testing.T, client *dnsgrpc.Client, expected []string) {
	t.Helper()
	servers, err := client.ListDNSServers(ctx)
	if err != nil {
		t.Fatalf("ListDNSServers failed: %v", err)
	}
	if servers == nil {
		servers = []string{}
	}
	if !reflect.DeepEqual(servers, expected) {
		t.Fatalf("expected %v, got %v", expected, servers)
	}
}

// TestIntegration_EndToEnd performs an end-to-end scenario against the
// in-process gRPC server. It verifies the initial empty state, adds a DNS
// server, asserts duplicate-add conflict, adds another server, removes one
// and checks the final state.
func TestIntegration_EndToEnd(t *testing.T) {
	client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	assertList(ctx, t, client, []string{})

	if err := client.AddDNSServer(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("AddDNSServer failed: %v", err)
	}

	assertList(ctx, t, client, []string{"8.8.8.8"})

	err := client.AddDNSServer(ctx, "8.8.8.8")
	if st, ok := status.FromError(err); !ok || st.Code() != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists error, got %v", err)
	}

	if err := client.AddDNSServer(ctx, "1.1.1.1"); err != nil {
		t.Fatalf("AddDNSServer failed: %v", err)
	}

	if err := client.RemoveDNSServer(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("RemoveDNSServer failed: %v", err)
	}

	assertList(ctx, t, client, []string{"1.1.1.1"})
}
