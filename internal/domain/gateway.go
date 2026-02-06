package domain

import "context"

// MessageHandler is a function signature for handling incoming messages.
type MessageHandler func(ctx context.Context, msg IncomingMessage) error

// Gateway defines the interface for a platform gateway (e.g., CLI, Slack, Telegram).
type Gateway interface {
	// Platform returns the unique identifier for the gateway's platform.
	Platform() Platform

	// Start begins the gateway's operation (e.g., listening for connections, starting a REPL).
	Start(ctx context.Context) error

	// Stop gracefully shuts down the gateway.
	Stop(ctx context.Context) error

	// Send sends an outgoing message to a user on the platform.
	Send(ctx context.Context, msg OutgoingMessage) error

	// OnMessage registers a handler function to be called when an incoming message is received.
	OnMessage(handler MessageHandler)
}
