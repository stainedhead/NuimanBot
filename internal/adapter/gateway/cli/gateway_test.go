package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os" // Added this import
	"strings"
	"sync"
	"testing"
	"time"

	"nuimanbot/internal/adapter/gateway/cli"
	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"
)

// Helper to create a new Gateway with mocked reader/writer for testing
func newTestGateway(cfg *config.CLIConfig, inputString string, output io.Writer) (g *cli.Gateway, r, w *os.File) {
	var err error // Declare err explicitly
	r, w, err = os.Pipe()
	if err != nil {
		panic(err) // Should not happen in test setup
	}

	g = cli.NewGateway(cfg)
	g.Reader = r
	g.Writer = output

	// Write initial input, if any
	if inputString != "" {
		if _, err := w.WriteString(inputString); err != nil {
			panic(fmt.Errorf("failed to write initial input to pipe: %w", err))
		}
	}
	return g, r, w
}

func TestNewGateway(t *testing.T) {
	cfg := &config.CLIConfig{DebugMode: true}
	g := cli.NewGateway(cfg)

	if g == nil {
		t.Fatal("NewGateway returned nil")
	}
	if g.Platform() != domain.PlatformCLI {
		t.Errorf("NewGateway platform mismatch: got %s, want %s", g.Platform(), domain.PlatformCLI)
	}
	if g.Cfg != cfg {
		t.Error("NewGateway did not set config correctly")
	}
}

func TestPlatform(t *testing.T) {
	cfg := &config.CLIConfig{}
	g := cli.NewGateway(cfg)

	expected := domain.PlatformCLI
	actual := g.Platform()

	if actual != expected {
		t.Errorf("Platform() = %s; want %s", actual, expected)
	}
}

func TestStartStop(t *testing.T) {
	cfg := &config.CLIConfig{}
	output := new(bytes.Buffer)
	g, readerPipe, writerPipe := newTestGateway(cfg, "", output) // No initial input for this test

	defer func() {
		if err := readerPipe.Close(); err != nil {
			t.Logf("Error closing reader pipe: %v", err)
		}
		if err := writerPipe.Close(); err != nil {
			t.Logf("Error closing writer pipe: %v", err)
		}
	}()
	// No input buffer needed for this test scenario

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure the test context is cancelled
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := g.Start(ctx)
		if err != nil {
			t.Errorf("Start returned an error: %v", err)
		}
	}()

	// Wait for the "started" message to appear, indicating the REPL loop is running.
	// This helps ensure the goroutine is fully active before we try to stop it.
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		time.Sleep(100 * time.Millisecond) // Poll every 100ms
		if strings.Contains(output.String(), "CLI Gateway started.") {
			break
		}
		if i == maxAttempts-1 {
			t.Fatal("Timeout waiting for 'CLI Gateway started.' message")
		}
	}

	err := g.Stop(ctx)
	if err != nil {
		t.Errorf("Stop returned an error: %v", err)
	}

	// Give time for processing
	time.Sleep(100 * time.Millisecond)

	wg.Wait() // Wait for g.Start goroutine to finish

	if !strings.Contains(output.String(), "CLI Gateway stopping...") {
		t.Errorf("Expected output to contain 'CLI Gateway stopping...', got: %s", output.String())
	}
}

func TestStart_ExitQuit(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"Exit command", "exit"},
		{"Quit command", "quit"},
		{"Exit command mixed case", "eXiT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.CLIConfig{}
			inputString := fmt.Sprintf("%s\n", tt.cmd)
			output := new(bytes.Buffer)
			g, readerPipe, writerPipe := newTestGateway(cfg, inputString, output)

			defer func() {
				if err := readerPipe.Close(); err != nil {
					t.Logf("Error closing reader pipe: %v", err)
				}
				if err := writerPipe.Close(); err != nil {
					t.Logf("Error closing writer pipe: %v", err)
				}
			}()

			ctx := context.Background()
			err := g.Start(ctx) // Should exit gracefully
			if err != nil {
				t.Errorf("Start returned an error for command %s: %v", tt.cmd, err)
			}

			if !strings.Contains(output.String(), "CLI Gateway started.") {
				t.Errorf("Expected welcome message, got: %s", output.String())
			}
		})
	}
}

func TestStart_NoMessageHandler(t *testing.T) {
	cfg := &config.CLIConfig{}
	inputString := "hello\nexit\n" // Input a message, then exit
	output := new(bytes.Buffer)
	g, readerPipe, writerPipe := newTestGateway(cfg, inputString, output)

	defer func() {
		if err := readerPipe.Close(); err != nil {
			t.Logf("Error closing reader pipe: %v", err)
		}
		if err := writerPipe.Close(); err != nil {
			t.Logf("Error closing writer pipe: %v", err)
		}
	}()

	ctx := context.Background()
	err := g.Start(ctx)
	if err != nil {
		t.Errorf("Start returned an unexpected error: %v", err)
	}

	expectedOutput := "Error: Message handler not set. Cannot process input."
	if !strings.Contains(output.String(), expectedOutput) {
		t.Errorf("Expected output to contain '%s', got: %s", expectedOutput, output.String())
	}
}

func TestStart_InputProcessing(t *testing.T) {
	cfg := &config.CLIConfig{}
	inputString := "hello world\nexit\n"
	output := new(bytes.Buffer)
	g, readerPipe, writerPipe := newTestGateway(cfg, inputString, output)

	defer func() {
		if err := readerPipe.Close(); err != nil {
			t.Logf("Error closing reader pipe: %v", err)
		}
		if err := writerPipe.Close(); err != nil {
			t.Logf("Error closing writer pipe: %v", err)
		}
	}()

	messageReceived := make(chan domain.IncomingMessage, 1)
	g.OnMessage(func(ctx context.Context, msg domain.IncomingMessage) error {
		messageReceived <- msg
		return nil
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := g.Start(ctx)
		if err != nil {
			t.Errorf("Start returned an error: %v", err)
		}
	}()

	select {
	case msg := <-messageReceived:
		if msg.Text != "hello world" {
			t.Errorf("Expected message text 'hello world', got: %s", msg.Text)
		}
		if msg.Platform != domain.PlatformCLI {
			t.Errorf("Expected platform CLI, got: %s", msg.Platform)
		}
		if msg.PlatformUID != "cli_user" {
			t.Errorf("Expected platform UID 'cli_user', got: %s", msg.PlatformUID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message to be processed")
	}

	if err := g.Stop(ctx); err != nil {
		t.Errorf("Error stopping gateway: %v", err)
	}
	wg.Wait()
}

func TestSend(t *testing.T) {
	cfg := &config.CLIConfig{}
	output := new(bytes.Buffer)
	g, readerPipe, writerPipe := newTestGateway(cfg, "", output) // No input needed for this test

	defer func() {
		if err := readerPipe.Close(); err != nil {
			t.Logf("Error closing reader pipe: %v", err)
		}
		if err := writerPipe.Close(); err != nil {
			t.Logf("Error closing writer pipe: %v", err)
		}
	}()

	ctx := context.Background()
	msg := domain.OutgoingMessage{
		RecipientID: "cli_user",
		Content:     "This is a test message.",
		Format:      "text",
	}

	err := g.Send(ctx, msg)
	if err != nil {
		t.Errorf("Send returned an error: %v", err)
	}

	expectedOutput := fmt.Sprintf("Bot: %s\n", msg.Content)
	if output.String() != expectedOutput {
		t.Errorf("Send output mismatch: got '%s', want '%s'", output.String(), expectedOutput)
	}
}
