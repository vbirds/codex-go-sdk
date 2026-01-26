package tests

import (
	"path/filepath"
	"runtime"
)

const (
	archAMD64  = "amd64"
	archARM64  = "arm64"
	osWindows  = "windows"
	binaryName = "codex"
	binaryWin  = "codex.exe"
)

// FindCodexBinary finds the codex binary in the parent codex project.
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
		case archAMD64:
			targetTriple = "x86_64-unknown-linux-musl"
		case archARM64:
			targetTriple = "aarch64-unknown-linux-musl"
		}
	case "darwin":
		switch arch {
		case archAMD64:
			targetTriple = "x86_64-apple-darwin"
		case archARM64:
			targetTriple = "aarch64-apple-darwin"
		}
	case osWindows:
		switch arch {
		case archAMD64:
			targetTriple = "x86_64-pc-windows-msvc"
		case archARM64:
			targetTriple = "aarch64-pc-windows-msvc"
		}
	}

	if targetTriple == "" {
		return ""
	}

	binaryNm := "codex"
	if platform == osWindows {
		binaryNm = binaryWin
	}
	binaryPath := filepath.Join(codexRoot, "codex-rs", "target", "debug", binaryNm)
	return binaryPath
}
