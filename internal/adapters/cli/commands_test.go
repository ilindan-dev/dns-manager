// Package cli contains unit tests for the command-line interface.
// These tests exercise argument validation and basic Cobra wiring without
// starting network services. They verify that commands return user-friendly
// errors on invalid input and enforce expected argument counts.
package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestClientAddCmd_Validation verifies that the add subcommand validates the
// provided IP address and returns an error for malformed input. The test runs
// the Cobra command in-memory and checks the returned error message.
func TestClientAddCmd_Validation(t *testing.T) {
	cmd := newAddCmd()

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"999.999.999.999"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid IP, but got nil")
	}

	expectedErr := "invalid IP address: 999.999.999.999"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

// TestClientAddCmd_ArgsCount ensures the add subcommand enforces the required
// number of arguments. When called with no arguments the command should
// return a Cobra-style argument count error.
func TestClientAddCmd_ArgsCount(t *testing.T) {
	cmd := newAddCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for missing arguments, but got nil")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestServerCmd_InvalidPort checks that the server command validates the
// port argument and returns a clear error when the value cannot be parsed
// as a numeric port. The test executes the command in-memory.
func TestServerCmd_InvalidPort(t *testing.T) {
	cmd := newServerCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"not-a-port"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for invalid port, but got nil")
	}

	expectedErr := "invalid port number: not-a-port"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}
