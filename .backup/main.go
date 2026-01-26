// Package codex provides a Go SDK for the Anthropic Codex CLI agent.
// It offers a programmatic interface to interact with the Codex agent,
// supporting both streaming and buffered execution modes.
package codex

// Re-export all types from the types package
import (
	"github.com/fanwenlin/codex-go-sdk/types"
)

// Type aliases for convenience
type (
	// ApprovalMode represents the approval mode for actions
	ApprovalMode = types.ApprovalMode
	// SandboxMode represents the sandbox access mode
	SandboxMode = types.SandboxMode
	// ModelReasoningEffort represents the reasoning effort level for the model
	ModelReasoningEffort = types.ModelReasoningEffort
	// WebSearchMode represents the web search mode
	WebSearchMode = types.WebSearchMode
	// CommandExecutionStatus represents the status of a command execution
	CommandExecutionStatus = types.CommandExecutionStatus
	// PatchChangeKind indicates the type of file change
	PatchChangeKind = types.PatchChangeKind
	// PatchApplyStatus represents the status of a file change
	PatchApplyStatus = types.PatchApplyStatus
	// McpToolCallStatus represents the status of an MCP tool call
	McpToolCallStatus = types.McpToolCallStatus
)

// Constant values for ApprovalMode
const (
	ApprovalModeNever     = types.ApprovalModeNever
	ApprovalModeOnRequest = types.ApprovalModeOnRequest
	ApprovalModeOnFailure = types.ApprovalModeOnFailure
	ApprovalModeUntrusted = types.ApprovalModeUntrusted
)

// Constant values for SandboxMode
const (
	SandboxModeReadOnly       = types.SandboxModeReadOnly
	SandboxModeWorkspaceWrite = types.SandboxModeWorkspaceWrite
	SandboxModeFullAccess     = types.SandboxModeFullAccess
)

// Constant values for ModelReasoningEffort
const (
	ModelReasoningEffortMinimal = types.ModelReasoningEffortMinimal
	ModelReasoningEffortLow     = types.ModelReasoningEffortLow
	ModelReasoningEffortMedium  = types.ModelReasoningEffortMedium
	ModelReasoningEffortHigh    = types.ModelReasoningEffortHigh
	ModelReasoningEffortXHigh   = types.ModelReasoningEffortXHigh
)

// Constant values for WebSearchMode
const (
	WebSearchModeDisabled = types.WebSearchModeDisabled
	WebSearchModeCached   = types.WebSearchModeCached
	WebSearchModeLive     = types.WebSearchModeLive
)

// Constant values for CommandExecutionStatus
const (
	CommandExecutionStatusInProgress = types.CommandExecutionStatusInProgress
	CommandExecutionStatusCompleted  = types.CommandExecutionStatusCompleted
	CommandExecutionStatusFailed     = types.CommandExecutionStatusFailed
)

// Re-export event types
type (
	ThreadEvent        = types.ThreadEvent
	ThreadStartedEvent = types.ThreadStartedEvent
	TurnStartedEvent   = types.TurnStartedEvent
	Usage              = types.Usage
	TurnCompletedEvent = types.TurnCompletedEvent
	ThreadError        = types.ThreadError
	TurnFailedEvent    = types.TurnFailedEvent
	ItemStartedEvent   = types.ItemStartedEvent
	ItemUpdatedEvent   = types.ItemUpdatedEvent
	ItemCompletedEvent = types.ItemCompletedEvent
	ThreadErrorEvent   = types.ThreadErrorEvent
)

// Re-export item types
type (
	ThreadItem           = types.ThreadItem
	CommandExecutionItem = types.CommandExecutionItem
	FileUpdateChange     = types.FileUpdateChange
	FileChangeItem       = types.FileChangeItem
	McpToolCallItem      = types.McpToolCallItem
	McpToolCallResult    = types.McpToolCallResult
	McpToolCallError     = types.McpToolCallError
	AgentMessageItem     = types.AgentMessageItem
	ReasoningItem        = types.ReasoningItem
	WebSearchItem        = types.WebSearchItem
	TodoItem             = types.TodoItem
	TodoListItem         = types.TodoListItem
	ErrorItem            = types.ErrorItem
)

// Re-export option types
type (
	CodexOptions  = types.CodexOptions
	ThreadOptions = types.ThreadOptions
	TurnOptions   = types.TurnOptions
)

// Re-export alias types
type (
	UserInput         = types.UserInput
	Input             = types.Input
	Turn              = types.Turn
	StreamedTurn      = types.StreamedTurn
	RunResult         = types.RunResult
	RunStreamedResult = types.RunStreamedResult
)

// Helper functions for creating inputs
var (
	NewTextInput  = types.NewTextInput
	NewImageInput = types.NewImageInput
)

// Classes and functions are already exported from other files
// No need to re-export them here as they would create circular references
