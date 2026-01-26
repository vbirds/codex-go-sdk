package codex

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	archAMD64  = "amd64"
	archARM64  = "arm64"
	osWindows  = "windows"
	binaryName = "codex"
	binaryWin  = "codex.exe"
)

// CodexExecArgs represents the arguments for executing a codex command.
type CodexExecArgs struct {
	Input string

	BaseUrl               string
	ApiKey                string
	ThreadId              *string
	Images                []string
	Model                 string
	SandboxMode           string
	WorkingDirectory      string
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

		commandArgs := c.buildCommandArgs(args)
		env := c.setupEnvironment(args)

		cmd, pipes, err := c.createCommand(commandArgs, env)
		if err != nil {
			output <- ExecResult{Error: err}
			return
		}

		if err := cmd.Start(); err != nil {
			output <- ExecResult{Error: fmt.Errorf("failed to start codex: %w", err)}
			return
		}

		c.handleStdin(pipes.stdin, args.Input)
		stderrBuilder, stderrWg := c.captureStderr(pipes.stderr)
		done := c.streamStdout(pipes.stdout, args.Context, cmd, output)

		c.waitForCompletion(done, args.Context, cmd, stderrBuilder, stderrWg, output)
	}()

	return output
}

type commandPipes struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

// buildCommandArgs constructs the command-line arguments for the codex binary.
func (c *CodexExec) buildCommandArgs(args CodexExecArgs) []string {
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

	commandArgs = c.appendWebSearchArgs(commandArgs, args)

	if args.ApprovalPolicy != "" {
		commandArgs = append(commandArgs, "--config", fmt.Sprintf(`approval_policy="%s"`, args.ApprovalPolicy))
	}

	for _, image := range args.Images {
		commandArgs = append(commandArgs, "--image", image)
	}

	if args.ThreadId != nil {
		commandArgs = append(commandArgs, "resume", *args.ThreadId)
	}

	return commandArgs
}

// appendWebSearchArgs adds web search configuration to command arguments.
func (c *CodexExec) appendWebSearchArgs(commandArgs []string, args CodexExecArgs) []string {
	if args.WebSearchMode != "" {
		return append(commandArgs, "--config", fmt.Sprintf(`web_search="%s"`, args.WebSearchMode))
	}
	if args.WebSearchEnabled != nil {
		if *args.WebSearchEnabled {
			return append(commandArgs, "--config", `web_search="live"`)
		}
		return append(commandArgs, "--config", `web_search="disabled"`)
	}
	return commandArgs
}

// setupEnvironment prepares environment variables for the codex command.
func (c *CodexExec) setupEnvironment(args CodexExecArgs) []string {
	env := c.buildBaseEnvironment()
	env = c.ensureOriginatorOverride(env)
	env = c.addApiConfiguration(env, args)
	return env
}

// buildBaseEnvironment creates the base environment from override or system env.
func (c *CodexExec) buildBaseEnvironment() []string {
	if c.envOverride != nil {
		env := make([]string, 0, len(c.envOverride))
		for k, v := range c.envOverride {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		return env
	}
	return os.Environ()
}

// ensureOriginatorOverride ensures CODEX_INTERNAL_ORIGINATOR_OVERRIDE is set.
func (c *CodexExec) ensureOriginatorOverride(env []string) []string {
	for _, e := range env {
		if strings.HasPrefix(e, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=") {
			return env
		}
	}
	return append(env, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=codex_sdk_go")
}

// addApiConfiguration adds API key and base URL to environment.
func (c *CodexExec) addApiConfiguration(env []string, args CodexExecArgs) []string {
	if args.BaseUrl != "" {
		env = append(env, fmt.Sprintf("OPENAI_BASE_URL=%s", args.BaseUrl))
	}
	if args.ApiKey != "" {
		env = append(env, fmt.Sprintf("CODEX_API_KEY=%s", args.ApiKey))
	}
	return env
}

// createCommand creates the command and sets up stdin, stdout, and stderr pipes.
func (c *CodexExec) createCommand(commandArgs []string, env []string) (*exec.Cmd, *commandPipes, error) {
	//nolint:gosec // Executing codex binary with constructed arguments is expected behavior
	cmd := exec.Command(c.executablePath, commandArgs...)
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	pipes := &commandPipes{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	return cmd, pipes, nil
}

// handleStdin writes input to stdin in a goroutine.
func (c *CodexExec) handleStdin(stdin io.WriteCloser, input string) {
	go func() {
		defer func() {
			_ = stdin.Close()
		}()
		if _, err := stdin.Write([]byte(input)); err != nil {
			// Error will be picked up from stderr or exit code
		}
	}()
}

// captureStderr captures stderr output in a goroutine.
func (c *CodexExec) captureStderr(stderr io.ReadCloser) (*strings.Builder, *sync.WaitGroup) {
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

	return &stderrBuilder, &stderrWg
}

// streamStdout streams stdout to the output channel in a goroutine.
func (c *CodexExec) streamStdout(stdout io.ReadCloser, ctx context.Context, cmd *exec.Cmd, output chan ExecResult) chan struct{} {
	done := make(chan struct{})
	scanner := bufio.NewScanner(stdout)

	go func() {
		defer close(done)
		for scanner.Scan() {
			select {
			case output <- ExecResult{Line: scanner.Text()}:
			case <-ctx.Done():
				_ = cmd.Process.Kill()
				return
			}
		}
	}()

	return done
}

// waitForCompletion waits for command completion or context cancellation.
func (c *CodexExec) waitForCompletion(done chan struct{}, ctx context.Context, cmd *exec.Cmd,
	stderrBuilder *strings.Builder, stderrWg *sync.WaitGroup, output chan ExecResult) {
	select {
	case <-done:
		// Scanner finished
	case <-ctx.Done():
		output <- ExecResult{Error: ctx.Err()}
		return
	}

	stderrWg.Wait()

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			output <- ExecResult{
				Error: fmt.Errorf("codex exited with code %d: %s", exitErr.ExitCode(), stderrBuilder.String()),
			}
		} else {
			output <- ExecResult{Error: fmt.Errorf("codex execution failed: %w", err)}
		}
	}
}

// findCodexPath finds the path to the codex binary based on platform and architecture.
func findCodexPath() string {
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
		default:
			panic(fmt.Sprintf("unsupported architecture on linux: %s", arch))
		}
	case "darwin":
		switch arch {
		case archAMD64:
			targetTriple = "x86_64-apple-darwin"
		case archARM64:
			targetTriple = "aarch64-apple-darwin"
		default:
			panic(fmt.Sprintf("unsupported architecture on darwin: %s", arch))
		}
	case osWindows:
		switch arch {
		case archAMD64:
			targetTriple = "x86_64-pc-windows-msvc"
		case archARM64:
			targetTriple = "aarch64-pc-windows-msvc"
		default:
			panic(fmt.Sprintf("unsupported architecture on windows: %s", arch))
		}
	default:
		panic(fmt.Sprintf("unsupported platform: %s (%s)", platform, arch))
	}

	// First, try to find the codex binary relative to the currently running binary
	// This handles the case where codex-orchestrator is installed in $GOPATH/bin
	if execPath, err := os.Executable(); err == nil {
		// Get the directory where the running binary is located
		binDir := filepath.Dir(execPath)

		// Try to find vendor directory in parent directories
		// Look for patterns like:
		// - $GOPATH/bin/codex-orchestrator -> $GOPATH/src/github.com/.../codex-go-sdk/vendor
		// - ./bin/codex-orchestrator -> ./vendor
		// - ./codex-orchestrator -> ./vendor

		// Search parent directories for vendor folder
		currentDir := binDir
		for i := 0; i < 5; i++ { // Search up to 5 levels
			// Try vendor at this level
			vendorRoot := filepath.Join(currentDir, "vendor")
			archRoot := filepath.Join(vendorRoot, targetTriple)
			binaryNm := binaryName
			if platform == osWindows {
				binaryNm = binaryWin
			}
			binaryPath := filepath.Join(archRoot, "codex", binaryNm)

			if _, err := os.Stat(binaryPath); err == nil {
				return binaryPath
			}

			// Move up one directory
			parentDir := filepath.Dir(currentDir)
			if parentDir == currentDir {
				break // Reached filesystem root
			}
			currentDir = parentDir
		}

		// Also try relative to bin directory for local development
		// ./bin/codex-orchestrator -> ../codex-go-sdk/vendor
		vendorRoot := filepath.Join(binDir, "..", "codex-go-sdk", "vendor")
		archRoot := filepath.Join(vendorRoot, targetTriple)
		binaryNm := binaryName
		if platform == osWindows {
			binaryNm = binaryWin
		}
		binaryPath := filepath.Join(archRoot, "codex", binaryNm)

		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath
		}
	}

	// Fallback 1: Try relative to current working directory (original behavior)
	vendorRoot := filepath.Join("..", "codex-go-sdk", "vendor")
	archRoot := filepath.Join(vendorRoot, targetTriple)
	binaryNm := binaryName
	if platform == osWindows {
		binaryNm = binaryWin
	}
	binaryPath := filepath.Join(archRoot, "codex", binaryNm)

	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath
	}

	// Fallback 2: Try in ~/.codex-orchestrator/vendor
	homeDir, err := os.UserHomeDir()
	if err == nil {
		vendorRoot := filepath.Join(homeDir, ".codex-orchestrator", "vendor")
		archRoot := filepath.Join(vendorRoot, targetTriple)
		binaryNm := binaryName
		if platform == osWindows {
			binaryNm = binaryWin
		}
		binaryPath := filepath.Join(archRoot, "codex", binaryNm)

		if _, err := os.Stat(binaryPath); err == nil {
			return binaryPath
		}
	}

	// Last resort: return the default relative path even if it doesn't exist
	// The error will be handled when trying to execute it
	return binaryPath
}
