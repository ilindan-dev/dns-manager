package service

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"testing"

	"github.com/ilindan-dev/dns-manager/internal/core/domain"
)

// fakeResolvConf is a lightweight in-memory implementation of ports.ResolvConf
// used for unit tests. It records rewrite calls and allows injecting errors.
type fakeResolvConf struct {
	mu           sync.Mutex
	servers      []string
	rewriteCalls [][]string
	getErr       error
	rewriteErr   error
}

func (f *fakeResolvConf) GetServers(_ context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	out := make([]string, len(f.servers))
	copy(out, f.servers)
	return out, nil
}

func (f *fakeResolvConf) RewriteServers(_ context.Context, servers []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rewriteErr != nil {
		return f.rewriteErr
	}
	f.servers = make([]string, len(servers))
	copy(f.servers, servers)
	f.rewriteCalls = append(f.rewriteCalls, append([]string(nil), servers...))
	return nil
}

func TestDNSManagerService_AddRemoveList(t *testing.T) {
	ctx := context.Background()
	fake := &fakeResolvConf{servers: []string{"1.1.1.1"}}
	svc := NewDNSManagerService(fake, slog.Default())

	// Add a new server
	if err := svc.AddDNSServer(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("AddDNSServer failed: %v", err)
	}
	got, err := svc.ListDNSServers(ctx)
	if err != nil {
		t.Fatalf("ListDNSServers failed: %v", err)
	}
	expected := []string{"1.1.1.1", "8.8.8.8"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected servers: got %v, want %v", got, expected)
	}

	// Adding duplicate should return ErrAlreadyExists
	if err := svc.AddDNSServer(ctx, "8.8.8.8"); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists; got %v", err)
	}

	// Remove existing
	if err := svc.RemoveDNSServer(ctx, "8.8.8.8"); err != nil {
		t.Fatalf("RemoveDNSServer failed: %v", err)
	}
	got, err = svc.ListDNSServers(ctx)
	if err != nil {
		t.Fatalf("ListDNSServers failed: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"1.1.1.1"}) {
		t.Fatalf("after remove, unexpected servers: %v", got)
	}

	// Removing missing should return ErrNotFound
	if err := svc.RemoveDNSServer(ctx, "8.8.8.8"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound; got %v", err)
	}
}

func TestValidateAndNormalizeIP(t *testing.T) {
	svc := NewDNSManagerService(&fakeResolvConf{}, slog.Default())

	ip, err := svc.validateAndNormalizeIP("2001:db8::1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "2001:db8::1" {
		t.Fatalf("unexpected normalized IP: %s", ip)
	}

	if _, err := svc.validateAndNormalizeIP("not-an-ip"); !errors.Is(err, domain.ErrInvalidIP) {
		t.Fatalf("expected ErrInvalidIP; got %v", err)
	}
}
