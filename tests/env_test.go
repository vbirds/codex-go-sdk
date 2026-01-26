package tests

import (
	"os"
	"testing"
)

// TestCodexBinaryPath tests finding the codex binary
func TestCodexBinaryPath(t *testing.T) {
	binaryPath := FindCodexBinary()

	// Skip if binary doesn't exist (codex-rs not compiled yet)
	if binaryPath == "" {
		t.Skip("Codex binary not found (expected if codex-rs hasn't been compiled yet)")
	}

	// Check if file exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Codex binary not found at: %s (run 'cargo build' in codex/codex-rs)", binaryPath)
	}
}

// TestBinaryPathFormat tests the binary path format
func TestBinaryPathFormat(t *testing.T) {
	binaryPath := FindCodexBinary()
	if binaryPath == "" {
		t.Skip("Skipping format test - binary not found")
	}

	// Path should contain correct platform info
	expectedPattern := "codex/codex-rs/target/debug/codex"
	// contains is a helper function to check if string contains substring
	contains := func(s, substr string) bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}

	if !contains(binaryPath, expectedPattern) {
		t.Errorf("Binary path should contain '%s', got: %s", expectedPattern, binaryPath)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s[1:], substr))
}
