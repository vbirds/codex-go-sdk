# Codex Go SDK

A Go SDK for the Anthropic Codex CLI agent, providing programmatic access to the Codex agent with feature parity to the TypeScript SDK.

## Installation

```bash
go get github.com/fanwenlin/codex-go-sdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    codex "github.com/fanwenlin/codex-go-sdk"
)

func main() {
    // Create a new Codex client
    client := codex.NewCodex(codex.CodexOptions{
        ApiKey: "your-api-key",
    })

    // Start a new thread
    thread := client.StartThread(codex.ThreadOptions{
        Model:        "claude-3-opus-20250219",
        SandboxMode:  codex.SandboxModeReadOnly,
    })

    // Run a turn and get the response
    result, err := thread.Run("Hello, world!", codex.TurnOptions{})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", result.FinalResponse)
    fmt.Printf("Usage: %d input tokens, %d output tokens\n",
        result.Usage.InputTokens, result.Usage.OutputTokens)
}
```

## Streaming Example

```go
// Start a new thread
thread := client.StartThread(codex.ThreadOptions{})

// Run with streaming
streamed, err := thread.RunStreamed("Tell me a story", codex.TurnOptions{})
if err != nil {
    log.Fatal(err)
}

// Process events as they arrive
for event := range streamed.Events {
    switch e := event.(type) {
    case *codex.ThreadStartedEvent:
        fmt.Printf("Thread started: %s\n", e.ThreadId)

    case *codex.TurnStartedEvent:
        fmt.Println("Turn started")

    case *codex.ItemStartedEvent:
        fmt.Printf("Item started: %s\n", e.Item.GetType())

    case *codex.ItemCompletedEvent:
        fmt.Printf("Item completed: %s\n", e.Item.GetType())

    case *codex.TurnCompletedEvent:
        fmt.Printf("Turn completed: %d input tokens\n", e.Usage.InputTokens)

    case *codex.ThreadErrorEvent:
        fmt.Printf("Error: %s\n", e.Message)
    }
}
```

## Structured Output

```go
schema := map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "title":   map[string]interface{}{"type": "string"},
        "author":  map[string]interface{}{"type": "string"},
        "summary": map[string]interface{}{"type": "string"},
    },
    "required": []string{"title", "summary"},
}

result, err := thread.Run(
    "Summarize this document",
    codex.TurnOptions{
        OutputSchema: schema,
    },
)
```

## Multi-modal Input

```go
input := []codex.UserInput{
    codex.NewTextInput("What's in this image?"),
    codex.NewImageInput("/path/to/image.png"),
}

result, err := thread.Run(input, codex.TurnOptions{})
```

## Thread Management

```go
// Start a new thread
thread := client.StartThread(codex.ThreadOptions{})

// Get the thread ID after the first turn
result, _ := thread.Run("First message", codex.TurnOptions{})
threadID := *thread.ID()

// Later, resume the thread
resumedThread := client.ResumeThread(threadID, codex.ThreadOptions{})
result, _ = resumedThread.Run("Continuing the conversation", codex.TurnOptions{})
```

## Configuration Options

### CodexOptions

- `ApiKey`: API key for authentication
- `BaseUrl`: Base URL for the API
- `CodexPathOverride`: Custom path to the codex binary
- `Env`: Environment variables for the codex process

### ThreadOptions

- `Model`: Model to use (e.g., "claude-3-opus-20250219")
- `SandboxMode`: Sandbox access mode (`read-only`, `workspace-write`, `danger-full-access`)
- `WorkingDirectory`: Working directory for the thread
- `ModelReasoningEffort`: Reasoning effort level (`minimal`, `low`, `medium`, `high`, `xhigh`)
- `NetworkAccessEnabled`: Enable network access
- `WebSearchMode`: Web search mode (`disabled`, `cached`, `live`)
- `WebSearchEnabled`: Legacy web search flag
- `ApprovalPolicy`: Approval mode (`never`, `on-request`, `on-failure`, `untrusted`)
- `AdditionalDirectories`: Additional directories to include
- `SkipGitRepoCheck`: Skip git repository check

### TurnOptions

- `OutputSchema`: JSON schema for structured output
- `Context`: Context for cancellation

## Error Handling

```go
result, err := thread.Run("Some prompt", codex.TurnOptions{})
if err != nil {
    // Handle execution errors
    log.Printf("Execution failed: %v", err)
}

// Turn failures are reported as events
for event := range streamed.Events {
    if turnFailed, ok := event.(*codex.TurnFailedEvent); ok {
        log.Printf("Turn failed: %s", turnFailed.Error.Message)
    }
    if threadError, ok := event.(*codex.ThreadErrorEvent); ok {
        log.Printf("Thread error: %s", threadError.Message)
    }
}
```

## Platform Support

The SDK supports the same platforms as the TypeScript SDK:

- macOS (x86_64, ARM64)
- Linux (x86_64, ARM64)
- Windows (x86_64, ARM64)

## Development

To build and test the SDK:

```bash
# Initialize dependencies
go mod tidy

# Build
go build ./...

# Run tests
go test ./...
```

## License

Same as the main Codex project.
