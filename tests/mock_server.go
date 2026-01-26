package tests

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// SseEvent represents a Server-Sent Event
type SseEvent struct {
	Type string
	Data map[string]interface{}
}

// SseResponseBody represents an SSE response
type SseResponseBody struct {
	Kind   string
	Events []SseEvent
}

// RecordedRequest represents a captured HTTP request
type RecordedRequest struct {
	Body    string
	JSON    map[string]interface{}
	Headers http.Header
}

// MockServer represents the HTTP test server
type MockServer struct {
	Server   *http.Server
	URL      string
	Requests []*RecordedRequest
	events   chan SseResponseBody
	status   int
	listener net.Listener
}

// StartMockServer starts an HTTP server that mocks the OpenAI Responses API
func StartMockServer(events []SseResponseBody, statusCode int) (*MockServer, error) {
	ms := &MockServer{
		Requests: []*RecordedRequest{},
		events:   make(chan SseResponseBody, len(events)),
		status:   statusCode,
	}

	// Seed the events channel
	for _, event := range events {
		ms.events <- event
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/responses", ms.handleResponses)

	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	ms.listener = listener

	ms.Server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ms.URL = fmt.Sprintf("http://%s", listener.Addr().String())

	// Start serving
	go func() {
		_ = ms.Server.Serve(listener)
	}()

	return ms, nil
}

// handleResponses handles the /responses endpoint
func (ms *MockServer) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	scanner := bufio.NewScanner(r.Body)
	var bodyStr string
	for scanner.Scan() {
		bodyStr += scanner.Text() + "\n"
	}

	// Parse JSON
	var jsonBody map[string]interface{}
	_ = json.Unmarshal([]byte(bodyStr), &jsonBody)

	// Record request
	ms.Requests = append(ms.Requests, &RecordedRequest{
		Body:    bodyStr,
		JSON:    jsonBody,
		Headers: r.Header,
	})

	// Get next response
	eventBatch := <-ms.events

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(ms.status)

	// Write SSE events
	for _, event := range eventBatch.Events {
		_, _ = fmt.Fprintf(w, "event: %s\n", event.Type)
		data, _ := json.Marshal(event.Data)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
	}

	//nolint:errcheck // Flush() doesn't return an error, this is a false positive
	w.(http.Flusher).Flush()
}

// Close stops the server
func (ms *MockServer) Close() error {
	return ms.Server.Close()
}

// GetRequests returns all recorded requests
func (ms *MockServer) GetRequests() []*RecordedRequest {
	return ms.Requests
}

// Helper functions for creating SSE events

const (
	defaultResponseID = "resp_mock"
	defaultMessageID  = "msg_mock"
)

// Sse creates an SSE response body
func Sse(events ...SseEvent) SseResponseBody {
	return SseResponseBody{
		Kind:   "sse",
		Events: events,
	}
}

// ResponseStarted creates a response.created event
func ResponseStarted(responseID ...string) SseEvent {
	id := defaultResponseID
	if len(responseID) > 0 {
		id = responseID[0]
	}
	return SseEvent{
		Type: "response.created",
		Data: map[string]interface{}{
			"response": map[string]interface{}{
				"id": id,
			},
		},
	}
}

// AssistantMessage creates a response.output_item.done event with assistant message
func AssistantMessage(text string, itemID ...string) SseEvent {
	id := defaultMessageID
	if len(itemID) > 0 {
		id = itemID[0]
	}
	return SseEvent{
		Type: "response.output_item.done",
		Data: map[string]interface{}{
			"item": map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"id":   id,
				"content": []map[string]interface{}{
					{
						"type": "output_text",
						"text": text,
					},
				},
			},
		},
	}
}

// ShellCall creates a response.output_item.done event with shell function call
func ShellCall() SseEvent {
	return SseEvent{
		Type: "response.output_item.done",
		Data: map[string]interface{}{
			"item": map[string]interface{}{
				"type":      "function_call",
				"call_id":   fmt.Sprintf("call_id%d", len("test")),
				"name":      "shell",
				"arguments": `{"command":["bash","-lc","echo 'Hello, world!'"],"timeout_ms":100}`,
			},
		},
	}
}

// ResponseFailed creates an error event
func ResponseFailed(errorMessage string) SseEvent {
	return SseEvent{
		Type: "error",
		Data: map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "rate_limit_exceeded",
				"message": errorMessage,
			},
		},
	}
}

// ResponseCompleted creates a response.completed event
func ResponseCompleted(responseID ...string) SseEvent {
	id := defaultResponseID
	if len(responseID) > 0 {
		id = responseID[0]
	}
	return SseEvent{
		Type: "response.completed",
		Data: map[string]interface{}{
			"response": map[string]interface{}{
				"id": id,
				"usage": map[string]interface{}{
					"input_tokens": float64(42),
					"input_tokens_details": map[string]interface{}{
						"cached_tokens": float64(12),
					},
					"output_tokens":         float64(5),
					"output_tokens_details": nil,
					"total_tokens":          float64(47),
				},
			},
		},
	}
}

// MakeChannel creates a channel from a slice
func MakeChannel(items []SseResponseBody) chan SseResponseBody {
	ch := make(chan SseResponseBody, len(items))
	for _, item := range items {
		ch <- item
	}
	return ch
}

// FindPair searches for a flag and returns its value
func FindPair(args []string, flag string) (string, bool) {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// FindPairs finds all values for a repeated flag
func FindPairs(args []string, flag string) []string {
	var values []string
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			values = append(values, args[i+1])
		}
	}
	return values
}
