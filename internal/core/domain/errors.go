// Package domain defines domain-level sentinel errors used by the core
// DNS manager logic. These exported error variables are intended for
// callers to compare using errors.Is so semantics remain stable across
// packages and versions.
package domain

import "errors"

// Sentinel errors used across domain logic.
var (
	// ErrInvalidIP indicates the provided DNS server address has an invalid
	// textual IP format (neither a valid IPv4 nor IPv6). Use when validating
	// client input for AddDNSServer.
	ErrInvalidIP = errors.New("invalid IP address format")

	// ErrAlreadyExists signals an attempt to add a nameserver that is
	// already present in the resolver configuration. Returning this allows
	// callers to treat duplicate-add attempts as a recoverable, explicit
	// condition instead of a generic success/failure.
	ErrAlreadyExists = errors.New("DNS server already exists")

	// ErrNotFound indicates the requested DNS server address was not found
	// when attempting removal.
	ErrNotFound = errors.New("DNS server not found")

	// ErrPermissionDenied indicates the process lacks permissions to modify
	// the resolver configuration (for example, writing /etc/resolv.conf).
	ErrPermissionDenied = errors.New("permission denied while accessing resolver configuration")

	// ErrIO represents an unexpected I/O error while reading or writing the
	// resolver configuration. When returning ErrIO, wrap the underlying
	// error with fmt.Errorf("%w") so callers can inspect the cause.
	ErrIO = errors.New("I/O error while accessing resolver configuration")
)
