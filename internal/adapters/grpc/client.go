package grpc

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ilindan-dev/dns-manager/internal/core/ports"

	pb "github.com/ilindan-dev/dns-manager/proto/dns/v1"
)

var (
	// Compile check that Client implements ports.DNSManager.
	_ ports.DNSManager = (*Client)(nil)
	// Compile check that Client implements io.Closer .
	_ io.Closer = (*Client)(nil)
)

// Client is a thin wrapper around the generated pb.DNSManagerClient that manages
// the underlying connection.
type Client struct {
	conn *grpc.ClientConn
	api  pb.DNSManagerClient
}

// NewClient connects to target and returns a Client.
func NewClient(target string) (*Client, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server at %s: %w", target, err)
	}

	return &Client{
		conn: conn,
		api:  pb.NewDNSManagerClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// AddDNSServer sends a request to add a DNS server.
func (c *Client) AddDNSServer(ctx context.Context, address string) error {
	req := &pb.AddDNSServerRequest{Address: address}
	_, err := c.api.AddDNSServer(ctx, req)
	return err
}

// RemoveDNSServer sends a request to delete the DNS server.
func (c *Client) RemoveDNSServer(ctx context.Context, address string) error {
	req := &pb.RemoveDNSServerRequest{Address: address}
	_, err := c.api.RemoveDNSServer(ctx, req)
	return err
}

// ListDNSServers requests a list of current DNS servers.
func (c *Client) ListDNSServers(ctx context.Context) ([]string, error) {
	resp, err := c.api.ListDNSServers(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Addresses, nil
}
