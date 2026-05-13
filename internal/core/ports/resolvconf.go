package ports

import "context"

// ResolvConf provides atomic operations for managing the system's resolver configuration.
type ResolvConf interface {
	// GetServers reads the configuration and returns the ordered list
	// of currently configured nameserver IPs.
	GetServers(ctx context.Context) ([]string, error)

	// RewriteServers atomically replaces all existing nameservers with the provided list.
	// The implementation MUST preserve all non-nameserver directives (e.g., search, options)
	// and guarantee "all-or-nothing" execution to prevent configuration corruption.
	RewriteServers(ctx context.Context, addresses []string) error
}
