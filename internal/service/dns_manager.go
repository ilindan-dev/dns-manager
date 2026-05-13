// Package service implements application-level services for DNS
// management. DNSManagerService provides concurrency-safe operations to
// add, remove and list nameservers using a ResolvConf adapter (defined
// in the ports package). The service validates and normalizes IP
// addresses, enforces duplicate checks, and returns domain sentinel
// errors for callers to handle (see internal/core/domain/errors.go).
package service

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"github.com/ilindan-dev/dns-manager/internal/core/domain"
	"github.com/ilindan-dev/dns-manager/internal/core/ports"
)

// Check at compile time that DNSManagerService implements the interface ports.DNSManager.
var _ ports.DNSManager = (*DNSManagerService)(nil)

// DNSManagerService implements the domain logic of DNS management (ports.DNSManager).
type DNSManagerService struct {
	resolvConf ports.ResolvConf
	logger     *slog.Logger

	mu sync.RWMutex
}

// NewDNSManagerService creates a new instance of the service NewDNSManagerService.
func NewDNSManagerService(rc ports.ResolvConf, logger *slog.Logger) *DNSManagerService {
	return &DNSManagerService{
		resolvConf: rc,
		logger:     logger,
	}
}

// AddDNSServer checks the IP, looks for duplicates, and atomically adds the server.
func (s *DNSManagerService) AddDNSServer(ctx context.Context, address string) error {
	normalizedIP, err := s.validateAndNormalizeIP(address)
	if err != nil {
		return domain.ErrInvalidIP
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	servers, err := s.resolvConf.GetServers(ctx)
	if err != nil {
		return err
	}

	for _, srv := range servers {
		if srv == normalizedIP {
			return domain.ErrAlreadyExists
		}
	}

	servers = append(servers, normalizedIP)
	return s.resolvConf.RewriteServers(ctx, servers)
}

// RemoveDNSServer deletes the IP address, if it exists.
func (s *DNSManagerService) RemoveDNSServer(ctx context.Context, address string) error {
	normalizedIP, err := s.validateAndNormalizeIP(address)
	if err != nil {
		return domain.ErrInvalidIP
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	servers, err := s.resolvConf.GetServers(ctx)
	if err != nil {
		return err
	}

	foundIdx := -1
	for i, srv := range servers {
		if srv == normalizedIP {
			foundIdx = i
			break
		}
	}

	if foundIdx == -1 {
		return domain.ErrNotFound
	}

	servers = append(servers[:foundIdx], servers[foundIdx+1:]...)

	return s.resolvConf.RewriteServers(ctx, servers)
}

// ListDNSServers returns a list of current servers.
func (s *DNSManagerService) ListDNSServers(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resolvConf.GetServers(ctx)
}

// validateAndNormalizeIP checks the format and returns a canonical string (for example, removes extra zeros).
func (s *DNSManagerService) validateAndNormalizeIP(address string) (string, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return "", domain.ErrInvalidIP
	}
	return ip.String(), nil
}
