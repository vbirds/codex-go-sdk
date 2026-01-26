package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fanwenlin/codex-go-sdk/types"
)

// Thread represents a conversation thread with the agent.
type Thread struct {
	exec          Exec
	options       types.CodexOptions
	id            *string
	threadOptions types.ThreadOptions
}

// ID returns the thread ID. Populated after the first turn starts.
func (t *Thread) ID() *string {
	return t.id
}

// newThread creates a new Thread instance.
func newThread(exec Exec, options types.CodexOptions, threadOptions types.ThreadOptions, id *string) *Thread {
	return &Thread{
		exec:          exec,
		options:       options,
		id:            id,
		threadOptions: threadOptions,
	}
}

// RunStreamed provides input to the agent and streams events as they are produced.
func (t *Thread) RunStreamed(input types.Input, turnOptions types.TurnOptions) (*types.StreamedTurn, error) {
	events, err := t.runStreamedInternal(input, turnOptions)
	if err != nil {
		return nil, err
	}
	return &types.StreamedTurn{Events: events}, nil
}

// runStreamedInternal is the internal implementation that generates events.
func (t *Thread) runStreamedInternal(input types.Input, turnOptions types.TurnOptions) (chan types.ThreadEvent, error) {
	schemaFile, err := CreateOutputSchemaFile(turnOptions.OutputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to create output schema file: %w", err)
	}

	args := t.buildExecArgs(input, turnOptions, schemaFile)
	events := make(chan types.ThreadEvent)

	go t.processEventStream(args, schemaFile, events)

	return events, nil
}

// buildExecArgs constructs the CodexExecArgs from input and options.
func (t *Thread) buildExecArgs(input types.Input, turnOptions types.TurnOptions, schemaFile *OutputSchemaFile) CodexExecArgs {
	prompt, images := t.normalizeInput(input)
	ctx := t.extractContext(turnOptions)

	var threadId *string
	if t.id != nil {
		threadId = t.id
	}

	return CodexExecArgs{
		Input:                 prompt,
		BaseUrl:               t.options.BaseUrl,
		ApiKey:                t.options.ApiKey,
		ThreadId:              threadId,
		Images:                images,
		Model:                 t.threadOptions.Model,
		SandboxMode:           string(t.threadOptions.SandboxMode),
		WorkingDirectory:      t.threadOptions.WorkingDirectory,
		SkipGitRepoCheck:      t.threadOptions.SkipGitRepoCheck,
		OutputSchemaFile:      schemaFile.SchemaPath,
		ModelReasoningEffort:  string(t.threadOptions.ModelReasoningEffort),
		Context:               ctx,
		NetworkAccessEnabled:  t.threadOptions.NetworkAccessEnabled,
		WebSearchMode:         string(t.threadOptions.WebSearchMode),
		WebSearchEnabled:      t.threadOptions.WebSearchEnabled,
		ApprovalPolicy:        string(t.threadOptions.ApprovalPolicy),
		AdditionalDirectories: t.threadOptions.AdditionalDirectories,
	}
}

// extractContext extracts context.Context from TurnOptions.
func (t *Thread) extractContext(turnOptions types.TurnOptions) context.Context {
	if turnOptions.Context != nil {
		if c, ok := turnOptions.Context.(context.Context); ok {
			return c
		}
	}
	return context.Background()
}

// processEventStream processes the event stream from codex execution.
func (t *Thread) processEventStream(args CodexExecArgs, schemaFile *OutputSchemaFile, events chan types.ThreadEvent) {
	defer close(events)
	defer func() {
		if err := schemaFile.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup schema file: %v\n", err)
		}
	}()

	resultChan := t.exec.Run(args)

	for result := range resultChan {
		if result.Error != nil {
			t.sendErrorEvent(events, result.Error.Error())
			return
		}

		if !t.processResult(result, args, events) {
			return
		}
	}
}

// processResult processes a single result from the exec channel.
func (t *Thread) processResult(result ExecResult, args CodexExecArgs, events chan types.ThreadEvent) bool {
	eventType, err := t.extractEventType(result.Line)
	if err != nil {
		t.sendErrorEvent(events, err.Error())
		return false
	}

	event, err := t.parseEvent(eventType, result.Line)
	if err != nil {
		t.sendErrorEvent(events, err.Error())
		return false
	}

	// Handle thread.started specially to capture thread ID
	if threadStarted, ok := event.(*types.ThreadStartedEvent); ok {
		t.id = &threadStarted.ThreadId
	}

	// Send event to channel
	select {
	case events <- event:
		return true
	case <-args.Context.Done():
		return false
	}
}

// extractEventType extracts the event type from a JSON line.
func (t *Thread) extractEventType(line string) (string, error) {
	var rawEvent map[string]interface{}
	if err := json.Unmarshal([]byte(line), &rawEvent); err != nil {
		return "", fmt.Errorf("failed to parse event: %v", err)
	}

	eventType, ok := rawEvent["type"].(string)
	if !ok {
		return "", fmt.Errorf("event missing type field: %s", line)
	}

	return eventType, nil
}

// parseEvent parses a JSON line into the appropriate event type.
func (t *Thread) parseEvent(eventType, line string) (types.ThreadEvent, error) {
	parser, ok := eventParsers[eventType]
	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	return parser(line)
}

// sendErrorEvent sends an error event to the events channel.
func (t *Thread) sendErrorEvent(events chan types.ThreadEvent, message string) {
	events <- &types.ThreadErrorEvent{
		Type:    "error",
		Message: message,
	}
}

// eventParser is a function type that parses a JSON line into a ThreadEvent.
type eventParser func(line string) (types.ThreadEvent, error)

// eventParsers maps event types to their parser functions.
var eventParsers = map[string]eventParser{
	"thread.started": func(line string) (types.ThreadEvent, error) {
		event := &types.ThreadStartedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"turn.started": func(line string) (types.ThreadEvent, error) {
		event := &types.TurnStartedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"turn.completed": func(line string) (types.ThreadEvent, error) {
		event := &types.TurnCompletedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"turn.failed": func(line string) (types.ThreadEvent, error) {
		event := &types.TurnFailedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"item.started": func(line string) (types.ThreadEvent, error) {
		event := &types.ItemStartedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"item.updated": func(line string) (types.ThreadEvent, error) {
		event := &types.ItemUpdatedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"item.completed": func(line string) (types.ThreadEvent, error) {
		event := &types.ItemCompletedEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
	"error": func(line string) (types.ThreadEvent, error) {
		event := &types.ThreadErrorEvent{}
		err := json.Unmarshal([]byte(line), event)
		return event, err
	},
}

// Run provides input to the agent and returns the completed turn.
func (t *Thread) Run(input types.Input, turnOptions types.TurnOptions) (*types.Turn, error) {
	events, err := t.runStreamedInternal(input, turnOptions)
	if err != nil {
		return nil, err
	}

	var items []types.ThreadItem
	var finalResponse string
	var usage *types.Usage
	var turnFailure error

	for event := range events {
		switch e := event.(type) {
		case *types.ItemCompletedEvent:
			// Check if this is an agent message to get the final response
			if agentMsg, ok := e.Item.(*types.AgentMessageItem); ok {
				finalResponse = agentMsg.Text
			}
			items = append(items, e.Item)
		case *types.TurnCompletedEvent:
			usage = &e.Usage
		case *types.TurnFailedEvent:
			turnFailure = fmt.Errorf("turn failed: %s", e.Error.Message)
		case *types.ThreadErrorEvent:
			turnFailure = fmt.Errorf("thread error: %s", e.Message)
		}

		if turnFailure != nil {
			break
		}
	}

	if turnFailure != nil {
		return nil, turnFailure
	}

	return &types.Turn{
		Items:         items,
		FinalResponse: finalResponse,
		Usage:         usage,
	}, nil
}

// normalizeInput normalizes the input into a prompt string and image paths.
func (t *Thread) normalizeInput(input types.Input) (prompt string, images []string) {
	// Check if input is a string
	if s, ok := input.(string); ok {
		return s, []string{}
	}

	// Otherwise, assume it's a slice of UserInput
	var promptParts []string
	var imagePaths []string

	// Try to parse as []UserInput
	if inputs, ok := input.([]types.UserInput); ok {
		for _, item := range inputs {
			switch item.Type {
			case "text":
				promptParts = append(promptParts, item.Text)
			case "local_image":
				imagePaths = append(imagePaths, item.Path)
			}
		}
	}

	return strings.Join(promptParts, "\n\n"), imagePaths
}
