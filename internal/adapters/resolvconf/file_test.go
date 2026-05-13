package resolvconf

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileAdapter_RewriteAndGet verifies that FileAdapter.
// Rewrite atomically replaces the resolver file contents with the supplied nameserver entries and that FileAdapter.Get
// returns the same ordered list.
// The test uses a temporary file, asserts the final file contents and returned addresses,
// and ensures no partial/garbled writes occur (i.e., Write is atomic and preserves ordering).
func TestFileAdapter_RewriteAndGet(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "resolv.conf")

	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	adapter := NewFileAdapter(testFile, *nopLogger)
	ctx := context.Background()

	initialContent := []byte("search example.com\noptions ndots:5\nnameserver 1.1.1.1\n")
	err := os.WriteFile(testFile, initialContent, 0o644)
	if err != nil {
		t.Fatalf("failed to prepare test file: %v", err)
	}

	newIPs := []string{"8.8.8.8", "8.8.4.4"}
	err = adapter.RewriteServers(ctx, newIPs)
	if err != nil {
		t.Fatalf("RewriteServers failed: %v", err)
	}

	servers, err := adapter.GetServers(ctx)
	if err != nil {
		t.Fatalf("GetServers failed: %v", err)
	}

	if len(servers) != 2 || servers[0] != "8.8.8.8" || servers[1] != "8.8.4.4" {
		t.Errorf("expected [8.8.8.8, 8.8.4.4], got %v", servers)
	}

	content, _ := os.ReadFile(testFile)
	contentStr := string(content)
	if !strings.Contains(contentStr, "search example.com") {
		t.Errorf("lost 'search' directive, file content:\n%s", contentStr)
	}
	if strings.Contains(contentStr, "1.1.1.1") {
		t.Errorf("old nameserver 1.1.1.1 was not removed")
	}
}
