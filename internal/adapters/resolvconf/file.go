// Package resolvconf provides utilities to read and update system resolver configuration
// files (for example /etc/resolv.conf). It exposes adapters that parse resolver entries,
// preserve non-nameserver lines and ordering, and perform atomic writes (temp-file + rename).
// The package translates filesystem and validation failures into domain sentinel errors
// (e.g. ErrInvalidIP, ErrPermissionDenied, ErrIO) and is safe for concurrent use by higher-level services.
package resolvconf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ilindan-dev/dns-manager/internal/core/domain"
	"github.com/ilindan-dev/dns-manager/internal/core/ports"
)

const (
	// nameserverPrefix contains the prefix for parsing and recording the DNS server.
	nameserverPrefix = "nameserver "

	// defaultPerms contains permissions for ease of coding.
	defaultPerms = 0o644
)

// Compile check that FileAdapter implements ports.ResolvConf.
var _ ports.ResolvConf = (*FileAdapter)(nil)

// FileAdapter implements ports.ResolvConf to work with a local file.
type FileAdapter struct {
	filePath string
	logger   slog.Logger
}

// NewFileAdapter creates a new adapter instance.
// Passing a path as a parameter allows for easy testing of the adapter on temporary files.
func NewFileAdapter(path string, logger slog.Logger) *FileAdapter {
	return &FileAdapter{
		filePath: path,
		logger:   logger,
	}
}

// GetServers reads a file and returns a list of current DNS server IP addresses.
func (a *FileAdapter) GetServers(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, err := os.Open(a.filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, a.mapError(err, "failed to open resolv.conf")
	}
	defer func() {
		err := file.Close()
		if err != nil {
			a.logger.Error("failed to close resolv.conf file", "error", err)
		}
	}()

	var servers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, nameserverPrefix) {
			ip := strings.TrimSpace(strings.TrimPrefix(line, nameserverPrefix))
			if ip != "" {
				servers = append(servers, ip)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, a.mapError(err, "failed to read resolv.conf")
	}

	return servers, nil
}

// RewriteServers atomically overwrites the file, saving the other settings.
func (a *FileAdapter) RewriteServers(ctx context.Context, addresses []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	newContent, err := a.buildNewContent(addresses)
	if err != nil {
		return err
	}

	return a.atomicSave(newContent)
}

// buildNewContent reads the old file (if any) and generates an updated config.
func (a *FileAdapter) buildNewContent(addresses []string) ([]byte, error) {
	var buffer bytes.Buffer

	file, err := os.Open(a.filePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, a.mapError(err, "failed to open resolv.conf for reading")
	}

	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, nameserverPrefix) {
				buffer.WriteString(scanner.Text() + "\n")
			}
		}
		if err := file.Close(); err != nil {
			a.logger.Error("failed to close resolv.conf file", "error", err)
		}
	}

	for _, ip := range addresses {
		if _, err := fmt.Fprintf(&buffer, "%s%s\n", nameserverPrefix, ip); err != nil {
			return nil, a.mapError(err, "failed to write to buffer")
		}
	}

	return buffer.Bytes(), nil
}

// atomicSave implements a pattern of atomic file rewriting.
func (a *FileAdapter) atomicSave(data []byte) error {
	dir := filepath.Dir(a.filePath)
	tempFile, err := os.CreateTemp(dir, "resolv.conf.tmp.*")
	if err != nil {
		return a.mapError(err, "failed to create temp file")
	}
	tempPath := tempFile.Name()

	defer func() {
		err := os.Remove(tempPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			a.logger.Error("failed to remove temp file", "error", err)
		}
	}()

	if err := tempFile.Chmod(defaultPerms); err != nil {
		_ = tempFile.Close()
		return a.mapError(err, "failed to set permissions on temp file")
	}

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return a.mapError(err, "failed to write to temp file")
	}

	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return a.mapError(err, "failed to sync temp file to disk")
	}

	if err := tempFile.Close(); err != nil {
		return a.mapError(err, "failed to close temp file")
	}

	if err := os.Rename(tempPath, a.filePath); err != nil {
		return a.mapError(err, "failed to atomically replace resolv.conf")
	}

	return nil
}

// mapError maps system fs/os errors to our domains.
func (a *FileAdapter) mapError(err error, msg string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("%s: %w", msg, domain.ErrPermissionDenied)
	}
	return fmt.Errorf("%s: %w (%w)", msg, domain.ErrIO, err)
}
