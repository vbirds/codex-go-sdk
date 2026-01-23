package tests

import (
	"testing"

	codex "github.com/fanwenlin/codex-go-sdk"
	"github.com/fanwenlin/codex-go-sdk/types"
)

func TestInputNormalization(t *testing.T) {
	// Create a thread to test normalization
	client := codex.NewCodex(types.CodexOptions{})
	_ = client.StartThread(types.ThreadOptions{})

	// We can't directly test private normalizeInput, so we test through Run
	// For this unit test, we'll just verify the types compile

	// Test string input
	input1 := "Hello, world!"
	if input1 == "" {
		t.Error("String input should not be empty")
	}

	// Test UserInput slice
	input2 := []types.UserInput{
		types.NewTextInput("First part"),
		types.NewTextInput("Second part"),
		types.NewImageInput("/path/to/image.png"),
	}

	if len(input2) != 3 {
		t.Errorf("Expected 3 inputs, got %d", len(input2))
	}

	if input2[0].Type != "text" || input2[0].Text != "First part" {
		t.Error("First input should be text type")
	}

	if input2[2].Type != "local_image" || input2[2].Path != "/path/to/image.png" {
		t.Error("Third input should be image type")
	}
}

func TestConstants(t *testing.T) {
	// Test that all constants are defined correctly
	if codex.ApprovalModeNever != "never" {
		t.Error("ApprovalModeNever mismatch")
	}
	if codex.ApprovalModeOnRequest != "on-request" {
		t.Error("ApprovalModeOnRequest mismatch")
	}
	if codex.SandboxModeReadOnly != "read-only" {
		t.Error("SandboxModeReadOnly mismatch")
	}
	if codex.ModelReasoningEffortHigh != "high" {
		t.Error("ModelReasoningEffortHigh mismatch")
	}
	if codex.WebSearchModeLive != "live" {
		t.Error("WebSearchModeLive mismatch")
	}
}

func TestTypeAliases(t *testing.T) {
	// Test that type aliases work
	var event types.ThreadEvent = &types.ThreadStartedEvent{
		Type:     "thread.started",
		ThreadId: "test-123",
	}

	if event.GetType() != "thread.started" {
		t.Error("GetType should return thread.started")
	}

	// Test item types
	var item types.ThreadItem = &types.AgentMessageItem{
		ID:   "msg-1",
		Type: "agent_message",
		Text: "Hello!",
	}

	if item.GetType() != "agent_message" {
		t.Error("Item GetType should return agent_message")
	}

	// Test CommandExecutionItem
	exitCode := 0
	cmdItem := &types.CommandExecutionItem{
		ID:               "cmd-1",
		Type:             "command_execution",
		Command:          "echo test",
		AggregatedOutput: "test\n",
		ExitCode:         &exitCode,
		Status:           types.CommandExecutionStatusCompleted,
	}

	if cmdItem.Status != types.CommandExecutionStatusCompleted {
		t.Error("Command should be completed")
	}
}

func TestTurnCreation(t *testing.T) {
	// Test Turn structure
	usage := &types.Usage{
		InputTokens:       100,
		CachedInputTokens: 20,
		OutputTokens:      50,
	}

	turn := &types.Turn{
		Items: []types.ThreadItem{
			&types.AgentMessageItem{
				ID:   "msg-1",
				Type: "agent_message",
				Text: "Test response",
			},
		},
		FinalResponse: "Test response",
		Usage:         usage,
	}

	if turn.FinalResponse != "Test response" {
		t.Error("Final response mismatch")
	}

	if turn.Usage.InputTokens != 100 {
		t.Error("Input tokens mismatch")
	}
}

func TestCodexClientCreation(t *testing.T) {
	// Test creating Codex client with options
	client := codex.NewCodex(types.CodexOptions{
		ApiKey: "test-key",
		BaseUrl: "https://api.example.com",
	})

	if client == nil {
		t.Error("Client should not be nil")
	}

	// Test creating thread
	thread := client.StartThread(types.ThreadOptions{
		Model:       "claude-3-opus",
		SandboxMode: codex.SandboxModeReadOnly,
	})

	if thread == nil {
		t.Error("Thread should not be nil")
	}

	// Thread ID should be nil initially
	if thread.ID() != nil {
		t.Error("Thread ID should be nil before first turn")
	}
}

func TestThreadResumption(t *testing.T) {
	client := codex.NewCodex(types.CodexOptions{})

	// Create thread with ID
	threadID := "existing-thread-123"
	thread := client.ResumeThread(threadID, types.ThreadOptions{})

	if thread == nil {
		t.Error("Resumed thread should not be nil")
	}

	id := thread.ID()
	if id == nil || *id != threadID {
		t.Error("Thread ID should match resumed ID")
	}
}

func TestOutputSchemaFile(t *testing.T) {
	// Test creating output schema file
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"answer": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"answer"},
	}

	schemaFile, err := codex.CreateOutputSchemaFile(schema)
	if err != nil {
		t.Fatalf("Failed to create schema file: %v", err)
	}

	if schemaFile.SchemaPath == "" {
		t.Error("Schema path should not be empty")
	}

	// Cleanup should work
	err = schemaFile.Cleanup()
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	// Test nil schema (should return no-op cleanup)
	schemaFile2, err := codex.CreateOutputSchemaFile(nil)
	if err != nil {
		t.Fatalf("Failed to create nil schema file: %v", err)
	}

	if schemaFile2.SchemaPath != "" {
		t.Error("Nil schema should have empty path")
	}

	err = schemaFile2.Cleanup()
	if err != nil {
		t.Errorf("Nil schema cleanup failed: %v", err)
	}
}

func TestEventTypes(t *testing.T) {
	// Test all event types can be created
	events := []types.ThreadEvent{
		&types.ThreadStartedEvent{
			Type:     "thread.started",
			ThreadId: "thread-123",
		},
		&types.TurnStartedEvent{
			Type: "turn.started",
		},
		&types.TurnCompletedEvent{
			Type: "turn.completed",
			Usage: types.Usage{
				InputTokens:       42,
				CachedInputTokens: 12,
				OutputTokens:      5,
			},
		},
		&types.ItemCompletedEvent{
			Type: "item.completed",
			Item: &types.AgentMessageItem{
				ID:   "msg-1",
				Type: "agent_message",
				Text: "Test",
			},
		},
		&types.ThreadErrorEvent{
			Type:    "error",
			Message: "Test error",
		},
	}

	for _, event := range events {
		if event.GetType() == "" {
			t.Error("Event type should not be empty")
		}
	}
}

func TestItemTypes(t *testing.T) {
	// Test all item types
	items := []types.ThreadItem{
		&types.CommandExecutionItem{
			ID:   "cmd-1",
			Type: "command_execution",
			Command: "ls -la",
			Status: types.CommandExecutionStatusInProgress,
		},
		&types.FileChangeItem{
			ID:   "file-1",
			Type: "file_change",
			Changes: []types.FileUpdateChange{
				{Path: "test.go", Kind: types.PatchChangeKindUpdate},
			},
			Status: types.PatchApplyStatusCompleted,
		},
		&types.McpToolCallItem{
			ID:        "mcp-1",
			Type:      "mcp_tool_call",
			Server:    "test-server",
			Tool:      "test-tool",
			Arguments: map[string]interface{}{"arg": "value"},
			Status:    types.McpToolCallStatusCompleted,
		},
		&types.WebSearchItem{
			ID:    "search-1",
			Type:  "web_search",
			Query: "test query",
		},
		&types.TodoListItem{
			ID:   "todo-1",
			Type: "todo_list",
			Items: []types.TodoItem{
				{Text: "Task 1", Completed: false},
			},
		},
		&types.ErrorItem{
			ID:      "error-1",
			Type:    "error",
			Message: "Test error",
		},
	}

	for _, item := range items {
		if item.GetType() == "" {
			t.Error("Item type should not be empty")
		}
	}
}
