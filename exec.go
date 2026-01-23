package codex

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// CodexExecArgs represents the arguments for executing a codex command.
type CodexExecArgs struct {
	Input string

	BaseUrl      string
	ApiKey       string
	ThreadId     *string
	Images       []string
	Model        string
	SandboxMode  string
	WorkingDirectory string
	AdditionalDirectories []string
	SkipGitRepoCheck      bool
	OutputSchemaFile      string
	ModelReasoningEffort  string
	Context               context.Context
	NetworkAccessEnabled  bool
	WebSearchMode         string
	WebSearchEnabled      *bool
	ApprovalPolicy        string
}

// ExecResult represents a result from executing codex.
type ExecResult struct {
	Line  string
	Error error
}

// CodexExec handles execution of the codex binary.
type CodexExec struct {
	executablePath string
	envOverride    map[string]string
}

// NewCodexExec creates a new CodexExec instance.
func NewCodexExec(executablePath string, envOverride map[string]string) *CodexExec {
	if executablePath == "" {
		executablePath = findCodexPath()
	}
	return &CodexExec{
		executablePath: executablePath,
		envOverride:    envOverride,
	}
}

// Run executes codex with the given arguments and returns a channel of results.
func (c *CodexExec) Run(args CodexExecArgs) <-chan ExecResult {
	output := make(chan ExecResult)

	go func() {
		defer close(output)

		commandArgs := []string{"exec", "--experimental-json"}

		if args.Model != "" {
			commandArgs = append(commandArgs, "--model", args.Model)
		}

		if args.SandboxMode != "" {
			commandArgs = append(commandArgs, "--sandbox", args.SandboxMode)
		}

		if args.WorkingDirectory != "" {
			commandArgs = append(commandArgs, "--cd", args.WorkingDirectory)
		}

		for _, dir := range args.AdditionalDirectories {
			commandArgs = append(commandArgs, "--add-dir", dir)
		}

		if args.SkipGitRepoCheck {
			commandArgs = append(commandArgs, "--skip-git-repo-check")
		}

		if args.OutputSchemaFile != "" {
			commandArgs = append(commandArgs, "--output-schema", args.OutputSchemaFile)
		}

		if args.ModelReasoningEffort != "" {
			commandArgs = append(commandArgs, "--config", fmt.Sprintf(`model_reasoning_effort="%s"`, args.ModelReasoningEffort))
		}

		if args.NetworkAccessEnabled {
			commandArgs = append(commandArgs, "--config", fmt.Sprintf(`sandbox_workspace_write.network_access=%t`, args.NetworkAccessEnabled))
		}

		if args.WebSearchMode != "" {
			commandArgs = append(commandArgs, "--config", fmt.Sprintf(`web_search="%s"`, args.WebSearchMode))
		} else if args.WebSearchEnabled != nil {
			if *args.WebSearchEnabled {
				commandArgs = append(commandArgs, "--config", `web_search="live"`)
			} else {
				commandArgs = append(commandArgs, "--config", `web_search="disabled"`)
			}
		}

		if args.ApprovalPolicy != "" {
			commandArgs = append(commandArgs, "--config", fmt.Sprintf(`approval_policy="%s"`, args.ApprovalPolicy))
		}

		for _, image := range args.Images {
			commandArgs = append(commandArgs, "--image", image)
		}

		if args.ThreadId != nil {
			commandArgs = append(commandArgs, "resume", *args.ThreadId)
		}

		// Set up environment
		env := os.Environ()
		if c.envOverride != nil {
			env = []string{}
			for k, v := range c.envOverride {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}

		// Check for internal originator override
		foundOriginator := false
		for _, e := range env {
			if strings.HasPrefix(e, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=") {
				foundOriginator = true
				break
			}
		}
		if !foundOriginator {
			env = append(env, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=codex_sdk_go")
		}

		// Set API key and base URL
		if args.BaseUrl != "" {
			env = append(env, fmt.Sprintf("OPENAI_BASE_URL=%s", args.BaseUrl))
		}
		if args.ApiKey != "" {
			env = append(env, fmt.Sprintf("CODEX_API_KEY=%s", args.ApiKey))
		}

		// Create command
		cmd := exec.Command(c.executablePath, commandArgs...)
		cmd.Env = env

		// Set up stdin
		stdin, err := cmd.StdinPipe()
		if err != nil {
			output <- ExecResult{Error: fmt.Errorf("failed to create stdin pipe: %w", err)}
			return
		}

		// Set up stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			output <- ExecResult{Error: fmt.Errorf("failed to create stdout pipe: %w", err)}
			return
		}

		// Set up stderr capture
		stderr, err := cmd.StderrPipe()
		if err != nil {
			output <- ExecResult{Error: fmt.Errorf("failed to create stderr pipe: %w", err)}
			return
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			output <- ExecResult{Error: fmt.Errorf("failed to start codex: %w", err)}
			return
		}

		// Write input to stdin
		go func() {
			defer stdin.Close()
			if _, err := stdin.Write([]byte(args.Input)); err != nil {
				// Error will be picked up from stderr or exit code
			}
		}()

		// Capture stderr
		var stderrBuilder strings.Builder
		var stderrWg sync.WaitGroup
		stderrWg.Add(1)
		go func() {
			defer stderrWg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				stderrBuilder.WriteString(scanner.Text())
				stderrBuilder.WriteString("\n")
			}
		}()

		// Create a context-aware scanner
		scanner := bufio.NewScanner(stdout)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for scanner.Scan() {
				select {
				case output <- ExecResult{Line: scanner.Text()}:
				case <-args.Context.Done():
					cmd.Process.Kill()
					return
				}
			}
		}()

		// Wait for completion or context cancellation
		select {
		case <-done:
			// Scanner finished
		case <-args.Context.Done():
			output <- ExecResult{Error: args.Context.Err()}
			return
		}

		// Wait for stderr
		stderrWg.Wait()

		// Wait for command to finish
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				output <- ExecResult{
					Error: fmt.Errorf("codex exited with code %d: %s", exitErr.ExitCode(), stderrBuilder.String()),
				}
			} else {
				output <- ExecResult{Error: fmt.Errorf("codex execution failed: %w", err)}
			}
			return
		}
	}()

	return output
}

// findCodexPath finds the path to the codex binary based on platform and architecture.
func findCodexPath() string {
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
		default:
			panic(fmt.Sprintf("unsupported architecture on linux: %s", arch))
		}
	case "darwin":
		switch arch {
		case "amd64":
			targetTriple = "x86_64-apple-darwin"
		case "arm64":
			targetTriple = "aarch64-apple-darwin"
		default:
			panic(fmt.Sprintf("unsupported architecture on darwin: %s", arch))
		}
	case "windows":
		switch arch {
		case "amd64":
			targetTriple = "x86_64-pc-windows-msvc"
		case "arm64":
			targetTriple = "aarch64-pc-windows-msvc"
		default:
			panic(fmt.Sprintf("unsupported architecture on windows: %s", arch))
		}
	default:
		panic(fmt.Sprintf("unsupported platform: %s (%s)", platform, arch))
	}

	// Vendor root is expected to be at the same level as the SDK directory
	// We'll use the current working directory as a reference
	vendorRoot := filepath.Join("..", "codex-go-sdk", "vendor")
	archRoot := filepath.Join(vendorRoot, targetTriple)
	binaryName := "codex"
	if platform == "windows" {
		binaryName = "codex.exe"
	}
	binaryPath := filepath.Join(archRoot, "codex", binaryName)

	return binaryPath
}
