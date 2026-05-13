// Package ports defines the contracts that the core logic
// requires to interact with external adapters.
package ports

import "context"

// DNSManager defines a layer of use cases for managing DNS servers.
type DNSManager interface {
	// AddDNSServer checks the validity of the IP address and adds it to the configuration,
	// if it is not already there.
	AddDNSServer(ctx context.Context, address string) error

	// RemoveDNSServer removes the IP address from the configuration.
	// Should return an error if the address is not found.
	RemoveDNSServer(ctx context.Context, address string) error

	// ListDNSServers returns a list of current DNS servers, preserving the original order.
	ListDNSServers(ctx context.Context) ([]string, error)
}
