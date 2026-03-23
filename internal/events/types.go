package events

import "time"

// Event type constants
const (
	// Library events
	EventLibraryScanStarted   EventType = "library.scan.started"
	EventLibraryScanProgress  EventType = "library.scan.progress"
	EventLibraryScanComplete  EventType = "library.scan.complete"
	EventLibraryScanError     EventType = "library.scan.error"

	// Stream events
	EventStreamStarted        EventType = "stream.started"
	EventStreamEnded          EventType = "stream.ended"
	EventStreamError          EventType = "stream.error"

	// Transcode events
	EventTranscodeStarted     EventType = "transcode.started"
	EventTranscodeProgress    EventType = "transcode.progress"
	EventTranscodeComplete    EventType = "transcode.complete"
	EventTranscodeError       EventType = "transcode.error"

	// Device events
	EventDeviceConnected      EventType = "device.connected"
	EventDeviceDisconnected   EventType = "device.disconnected"
)

// Topic constants
const (
	TopicLibraries  = "libraries"
	TopicStreams     = "streams"
	TopicTranscodes  = "transcodes"
	TopicDevices     = "devices"
	TopicAll         = "all"
)

// EventType represents the type of event
type EventType string

// Event represents a system event
type Event struct {
	Type      EventType       `json:"type"`
	Topic     string          `json:"topic"`
	Payload   interface{}     `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	ID        string          `json:"id"`
}

// EventPayload is a generic payload container
type EventPayload struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// LibraryScanPayload contains library scan event data
type LibraryScanPayload struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name,omitempty"`
	TotalFiles  int    `json:"total_files,omitempty"`
	Processed   int    `json:"processed,omitempty"`
	CurrentFile string `json:"current_file,omitempty"`
	Status      string `json:"status,omitempty"`
	Error       string `json:"error,omitempty"`
}

// StreamPayload contains stream event data
type StreamPayload struct {
	StreamID    string `json:"stream_id"`
	SessionID   string `json:"session_id,omitempty"`
	MediaItemID string `json:"media_item_id"`
	MediaTitle  string `json:"media_title,omitempty"`
	Variant     string `json:"variant,omitempty"`
	BytesServed int64  `json:"bytes_served,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
}

// TranscodePayload contains transcode event data
type TranscodePayload struct {
	TranscodeID    string  `json:"transcode_id"`
	SessionID      string  `json:"session_id,omitempty"`
	MediaItemID    string  `json:"media_item_id"`
	MediaTitle     string  `json:"media_title,omitempty"`
	SourceCodec    string  `json:"source_codec,omitempty"`
	TargetCodec    string  `json:"target_codec,omitempty"`
	Progress       float64 `json:"progress,omitempty"`
	Bitrate        int64   `json:"bitrate,omitempty"`
	FrameRate      float64 `json:"frame_rate,omitempty"`
	Duration       float64 `json:"duration,omitempty"`
	ProcessedTime  float64 `json:"processed_time,omitempty"`
	Status         string  `json:"status,omitempty"`
	Error          string  `json:"error,omitempty"`
}

// DevicePayload contains device event data
type DevicePayload struct {
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name,omitempty"`
	Platform      string `json:"platform,omitempty"`
	Capabilities  interface{} `json:"capabilities,omitempty"`
	TrustScore    float64 `json:"trust_score,omitempty"`
}

// NewEvent creates a new event with the given type, topic, and payload
func NewEvent(eventType EventType, topic string, payload interface{}) *Event {
	return &Event{
		Type:      eventType,
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
		ID:        generateEventID(),
	}
}
