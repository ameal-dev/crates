package ai

// StreamChunkMsg carries a text delta from the streaming AI response.
type StreamChunkMsg struct {
	Content string
}

// StreamDoneMsg signals the streaming response is complete, with the full accumulated text.
type StreamDoneMsg struct {
	FullContent string
}

// StreamErrorMsg carries an error from the streaming AI response.
type StreamErrorMsg struct {
	Err error
}

// StreamFallbackMsg notifies the UI that the primary model was unavailable
// and streaming is retrying with a fallback model.
type StreamFallbackMsg struct {
	FromModel string
	ToModel   string
}

// StreamConnectingMsg notifies the UI that a connection attempt is starting.
type StreamConnectingMsg struct {
	Model string
}
