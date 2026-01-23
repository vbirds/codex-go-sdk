package tests

import (
	"path/filepath"
	"runtime"
)

// FindCodexBinary finds the codex binary in the parent codex project
func FindCodexBinary() string {
	_, currentFile, _, _ := runtime.Caller(0)
	// From tests/helpers.go, go up 2 levels to codex-go-sdk, then to codex
	codexRoot := filepath.Join(filepath.Dir(currentFile), "..", "..", "codex")

	platform := runtime.GOOS
	arch := runtime.GOARCH

	var targetTriple string
	switch platform {
	case "linux":
		switch arch {
		case "amd64":
			targetTriple = "x86_64-unknown-linux-musl"
		case "arm64":
			targetTriple = "aarch64-unknown-linux-musl"
		}
	case "darwin":
		switch arch {
		case "amd64":
			targetTriple = "x86_64-apple-darwin"
		case "arm64":
			targetTriple = "aarch64-apple-darwin"
		}
	case "windows":
		switch arch {
		case "amd64":
			targetTriple = "x86_64-pc-windows-msvc"
		case "arm64":
			targetTriple = "aarch64-pc-windows-msvc"
		}
	}

	if targetTriple == "" {
		return ""
	}

	binaryName := "codex"
	if platform == "windows" {
		binaryName = "codex.exe"
	}

	binaryPath := filepath.Join(codexRoot, "codex-rs", "target", "debug", binaryName)
	return binaryPath
}
