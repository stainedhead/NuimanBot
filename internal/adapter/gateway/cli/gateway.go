package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time" // Added for time.Now()

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// Gateway implements domain.Gateway for the command-line interface.
type Gateway struct {
	Cfg            *config.CLIConfig
	messageHandler domain.MessageHandler
	// stdin/stdout for testing purposes
	Reader io.Reader
	Writer io.Writer
	cancel context.CancelFunc // For stopping the REPL
}

// NewGateway creates a new CLI Gateway instance.
func NewGateway(cfg *config.CLIConfig) *Gateway {
	return &Gateway{
		Cfg:    cfg,
		Reader: os.Stdin,
		Writer: os.Stdout,
	}
}

// Platform returns the platform identifier for CLI.
func (g *Gateway) Platform() domain.Platform {
	return domain.PlatformCLI
}

// Start begins the interactive REPL loop.
func (g *Gateway) Start(ctx context.Context) error {
	ctx, g.cancel = context.WithCancel(ctx)
	scanner := bufio.NewScanner(g.Reader)

	if g.Cfg.DebugMode {
		if _, err := fmt.Fprintln(g.Writer, "CLI Gateway started in debug mode. Type 'exit' or 'quit' to stop."); err != nil {
			// Log the error, but continue since it's a non-critical output
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}
	} else {
		if _, err := fmt.Fprintln(g.Writer, "CLI Gateway started. Type 'exit' or 'quit' to stop."); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}
	}

	for {
		// Prompt for input
		if _, err := fmt.Fprint(g.Writer, "> "); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
		}

		// Perform scan in a way that respects context cancellation
		scanDone := make(chan bool)
		go func() {
			scanDone <- scanner.Scan() // This can block
		}()

		select {
		case scanned := <-scanDone:
			if !scanned { // scanner.Scan() returned false
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("CLI scanner error: %w", err)
				}
				// EOF or user closed stdin
				return nil
			}
			// Process input here
			input := scanner.Text()
			input = strings.TrimSpace(input)

			if input == "" {
				continue
			}

			if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
				return nil
			}

			if g.messageHandler == nil {
				if _, err := fmt.Fprintln(g.Writer, "Error: Message handler not set. Cannot process input."); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
				}
				continue
			}

			incomingMsg := domain.IncomingMessage{
				ID:          "cli-" + fmt.Sprintf("%d", time.Now().UnixNano()), // Unique ID
				Platform:    domain.PlatformCLI,
				PlatformUID: "cli_user", // Placeholder for CLI user ID
				Text:        input,
				Timestamp:   time.Now(),
				Metadata:    nil,
			}

			if err := g.messageHandler(ctx, incomingMsg); err != nil {
				if _, writeErr := fmt.Fprintf(g.Writer, "Error processing message: %v\n", err); writeErr != nil {
					fmt.Fprintf(os.Stderr, "Error writing to CLI output (after message processing error): %v\n", writeErr)
				}
			}
		case <-ctx.Done():
			if _, err := fmt.Fprintln(g.Writer, "CLI Gateway stopping..."); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to CLI output: %v\n", err)
			}
			return nil
		}
	}
}

// Stop gracefully shuts down the gateway.
func (g *Gateway) Stop(ctx context.Context) error {
	if g.cancel != nil {
		g.cancel()
	}
	return nil
}

// Send sends a message to a user (CLI output).
func (g *Gateway) Send(ctx context.Context, msg domain.OutgoingMessage) error {
	_, err := fmt.Fprintf(g.Writer, "Bot: %s\n", msg.Content)
	if err != nil {
		return fmt.Errorf("failed to write to CLI output: %w", err)
	}
	return nil
}

// OnMessage registers a handler for incoming messages.
func (g *Gateway) OnMessage(handler domain.MessageHandler) {
	g.messageHandler = handler
}
