// Package cli provides the command-line interface for dns-manager.
// It exposes a Cobra-based root command with server and client subcommands.
// The client subcommands talk to a gRPC dns-manager server; the server command
// runs an in-process gRPC server that delegates to the service layer.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ilindan-dev/dns-manager/internal/adapters/resolvconf"
	"github.com/ilindan-dev/dns-manager/internal/service"
	pb "github.com/ilindan-dev/dns-manager/proto/dns/v1"

	dnsgrpc "github.com/ilindan-dev/dns-manager/internal/adapters/grpc"
)

const (
	timeout         = 5 * time.Second
	shutdownTimeout = 10 * time.Second
)

// Execute sets up and runs the root cobra command for the dns-manager CLI.
// It registers subcommands and exits the process with non-zero status on error.
func Execute() {
	rootCmd := &cobra.Command{
		Use:               "dns-manager",
		Short:             "Manage DNS servers (server and client modes)",
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	rootCmd.AddCommand(newServerCmd())
	rootCmd.AddCommand(newClientCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// newServerCmd returns the "server" command which starts the gRPC server.
// The command validates args, builds dependencies (resolvconf adapter, service)
// and runs the gRPC server until an interrupt/termination signal is received.
func newServerCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "server [port]",
		Short: "Start the gRPC dns-manager server on the specified port",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			portStr := args[0]

			if _, err := strconv.ParseUint(portStr, 10, 16); err != nil {
				return fmt.Errorf("invalid port number: %s", portStr)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			logger.Info("Starting dns-manager server", "port", portStr, "config", configPath)

			adapter := resolvconf.NewFileAdapter(configPath, logger)
			svc := service.NewDNSManagerService(adapter, logger)
			s := grpc.NewServer()

			pb.RegisterDNSManagerServer(s, dnsgrpc.NewServer(svc))

			lis, err := net.Listen("tcp", ":"+portStr)
			if err != nil {
				return fmt.Errorf("failed to listen on port %s: %w", portStr, err)
			}

			go func() {
				if err := s.Serve(lis); err != nil {
					logger.Error("gRPC server exited", "err", err)
				}
			}()

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			<-ctx.Done()
			logger.Info("Shutting down dns-manager server gracefully...")

			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			stopped := make(chan struct{})
			go func() {
				s.GracefulStop()
				close(stopped)
			}()

			select {
			case <-shutdownCtx.Done():
				logger.Warn("Graceful shutdown timed out, forcing stop")
				s.Stop()
			case <-stopped:
				logger.Info("Server stopped gracefully")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "/etc/resolv.conf", "Path to resolv.conf file")
	return cmd
}

// newClientCmd returns the "client" command grouping add/remove/list subcommands.
func newClientCmd() *cobra.Command {
	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Client commands to manage DNS servers on a dns-manager server",
	}

	clientCmd.PersistentFlags().StringP("server", "s", "localhost:50051", "Address of the dns-manager server")

	clientCmd.AddCommand(newAddCmd())
	clientCmd.AddCommand(newRemoveCmd())
	clientCmd.AddCommand(newListCmd())

	return clientCmd
}

// setupClient is DRY helper for initializing a gRPC client.
func setupClient(cmd *cobra.Command, logger *slog.Logger) (*dnsgrpc.Client, func(), error) {
	addr, _ := cmd.Flags().GetString("server")

	client, err := dnsgrpc.NewClient(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	cleanup := func() {
		if err := client.Close(); err != nil {
			logger.Warn("failed to close gRPC client", "err", err)
		}
	}

	return client, cleanup, nil
}

// newAddCmd returns the "add" subcommand. It connects to the gRPC server and
// issues an AddDNSServer RPC. The command validates the provided IP and uses
// a short timeout to avoid hanging the CLI.
func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [ip]",
		Short: "Add a DNS server to the remote configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ip := args[0]

			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid IP address: %s", ip)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client, cleanup, err := setupClient(cmd, logger)
			if err != nil {
				return err
			}
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			if err := client.AddDNSServer(ctx, ip); err != nil {
				return handleError(err, "failed to add DNS server")
			}

			fmt.Println("DNS server successfully added")
			return nil
		},
	}
}

// newRemoveCmd returns the "remove" subcommand which issues RemoveDNSServer RPCs.
func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [ip]",
		Short: "Remove a DNS server from the remote configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ip := args[0]

			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid IP address: %s", ip)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client, cleanup, err := setupClient(cmd, logger)
			if err != nil {
				return err
			}
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			if err := client.RemoveDNSServer(ctx, ip); err != nil {
				return handleError(err, "failed to remove DNS server")
			}

			fmt.Println("DNS server successfully removed")
			return nil
		},
	}
}

// newListCmd returns the "list" subcommand which requests the current list
// of DNS servers from the remote service and prints them to stderr.
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List DNS servers configured on the remote server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			client, cleanup, err := setupClient(cmd, logger)
			if err != nil {
				return err
			}
			defer cleanup()

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			servers, err := client.ListDNSServers(ctx)
			if err != nil {
				return handleError(err, "failed to list DNS servers")
			}

			if len(servers) == 0 {
				fmt.Println("No DNS servers configured.")
				return nil
			}

			fmt.Println("Configured DNS servers:")
			for _, srv := range servers {
				fmt.Printf(" - %s\n", srv)
			}
			return nil
		},
	}
}

// handleError maps gRPC status codes to user-friendly CLI errors.
func handleError(err error, contextMsg string) error {
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%s: %w", contextMsg, err)
	}

	switch st.Code() {
	case codes.Unavailable:
		return fmt.Errorf("%s: server unavailable", contextMsg)
	case codes.DeadlineExceeded:
		return fmt.Errorf("%s: deadline exceeded", contextMsg)
	case codes.AlreadyExists, codes.NotFound, codes.InvalidArgument:
		return fmt.Errorf("%s: %s", contextMsg, st.Message())
	case codes.PermissionDenied:
		return fmt.Errorf("%s: permission denied (does the server have write access to the config file?)", contextMsg)
	default:
		return fmt.Errorf("%s: internal error (code: %s)", contextMsg, st.Code().String())
	}
}
