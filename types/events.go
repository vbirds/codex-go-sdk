package types

// ThreadEvent is the union type for all thread events.
// In Go, we implement this using an interface that all event types implement.
type ThreadEvent interface {
	// Type returns the event type discriminator
	GetType() string
}

// ThreadStartedEvent is emitted when a new thread is started as the first event.
type ThreadStartedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// ThreadId is the identifier of the new thread
	ThreadId string `json:"thread_id"`
}

// GetType returns the event type discriminator
func (e ThreadStartedEvent) GetType() string {
	return e.Type
}

// TurnStartedEvent is emitted when a turn is started by sending a new prompt to the model.
type TurnStartedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
}

// GetType returns the event type discriminator
func (e TurnStartedEvent) GetType() string {
	return e.Type
}

// Usage describes the usage of tokens during a turn.
type Usage struct {
	// InputTokens is the number of input tokens used during the turn
	InputTokens int `json:"input_tokens"`
	// CachedInputTokens is the number of cached input tokens used during the turn
	CachedInputTokens int `json:"cached_input_tokens"`
	// OutputTokens is the number of output tokens used during the turn
	OutputTokens int `json:"output_tokens"`
}

// TurnCompletedEvent is emitted when a turn is completed.
type TurnCompletedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Usage is the token usage for the turn
	Usage Usage `json:"usage"`
}

// GetType returns the event type discriminator
func (e TurnCompletedEvent) GetType() string {
	return e.Type
}

// ThreadError represents a fatal error emitted by the stream.
type ThreadError struct {
	Message string `json:"message"`
}

// TurnFailedEvent indicates that a turn failed with an error.
type TurnFailedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Error is the error that occurred
	Error ThreadError `json:"error"`
}

// GetType returns the event type discriminator
func (e TurnFailedEvent) GetType() string {
	return e.Type
}

// ItemStartedEvent is emitted when a new item is added to the thread.
type ItemStartedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Item is the thread item that was started
	Item ThreadItem `json:"item"`
}

// GetType returns the event type discriminator
func (e ItemStartedEvent) GetType() string {
	return e.Type
}

// ItemUpdatedEvent is emitted when an item is updated.
type ItemUpdatedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Item is the updated thread item
	Item ThreadItem `json:"item"`
}

// GetType returns the event type discriminator
func (e ItemUpdatedEvent) GetType() string {
	return e.Type
}

// ItemCompletedEvent signals that an item has reached a terminal state.
type ItemCompletedEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Item is the completed thread item
	Item ThreadItem `json:"item"`
}

// GetType returns the event type discriminator
func (e ItemCompletedEvent) GetType() string {
	return e.Type
}

// ThreadErrorEvent represents an unrecoverable error emitted by the event stream.
type ThreadErrorEvent struct {
	// Type is the event type discriminator
	Type string `json:"type"`
	// Message is the error message
	Message string `json:"message"`
}

// GetType returns the event type discriminator
func (e ThreadErrorEvent) GetType() string {
	return e.Type
}
