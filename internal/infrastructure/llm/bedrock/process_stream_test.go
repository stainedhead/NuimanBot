package bedrock

import (
	"fmt"
	"testing"

	"nuimanbot/internal/config"
	"nuimanbot/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// mockConverseStreamOutputReader implements bedrockruntime.ConverseStreamOutputReader for testing.
type mockConverseStreamOutputReader struct {
	events chan types.ConverseStreamOutput
	err    error
}

func (m *mockConverseStreamOutputReader) Events() <-chan types.ConverseStreamOutput {
	return m.events
}

func (m *mockConverseStreamOutputReader) Close() error {
	return nil
}

func (m *mockConverseStreamOutputReader) Err() error {
	return m.err
}

// newStreamTestClient creates a test bedrock client used for processStreamEvents tests.
func newStreamTestClient() *Client {
	cfg := &config.BedrockProviderConfig{
		AWSRegion: "us-east-1",
	}
	awsCfg := aws.Config{Region: "us-east-1"}
	return NewClientWithConfig(cfg, awsCfg)
}

// TestProcessStreamEvents_ContentDelta tests processing text delta events.
func TestProcessStreamEvents_ContentDelta(t *testing.T) {
	client := newStreamTestClient()
	chunkChan := make(chan domain.StreamChunk, 10)

	events := make(chan types.ConverseStreamOutput, 3)

	idx := int32(0)
	text := "Hello, world!"
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{
		Value: types.ContentBlockDeltaEvent{
			ContentBlockIndex: &idx,
			Delta: &types.ContentBlockDeltaMemberText{
				Value: text,
			},
		},
	}

	events <- &types.ConverseStreamOutputMemberMessageStop{
		Value: types.MessageStopEvent{},
	}
	close(events)

	reader := &mockConverseStreamOutputReader{events: events}
	stream := bedrockruntime.NewConverseStreamEventStream(func(es *bedrockruntime.ConverseStreamEventStream) {
		es.Reader = reader
	})

	go client.processStreamEvents(stream, chunkChan)

	var chunks []domain.StreamChunk
	for c := range chunkChan {
		chunks = append(chunks, c)
	}

	if len(chunks) != 2 {
		t.Fatalf("Expected 2 chunks (delta + done), got %d", len(chunks))
	}

	if chunks[0].Delta != text {
		t.Errorf("Expected delta %q, got %q", text, chunks[0].Delta)
	}
	if chunks[0].Done {
		t.Error("Expected Done=false for delta chunk")
	}
	if !chunks[1].Done {
		t.Error("Expected Done=true for stop chunk")
	}
}

// TestProcessStreamEvents_MetadataEvent tests that metadata events are ignored.
func TestProcessStreamEvents_MetadataEvent(t *testing.T) {
	client := newStreamTestClient()
	chunkChan := make(chan domain.StreamChunk, 10)

	events := make(chan types.ConverseStreamOutput, 3)

	events <- &types.ConverseStreamOutputMemberMetadata{
		Value: types.ConverseStreamMetadataEvent{},
	}
	events <- &types.ConverseStreamOutputMemberMessageStop{
		Value: types.MessageStopEvent{},
	}
	close(events)

	reader := &mockConverseStreamOutputReader{events: events}
	stream := bedrockruntime.NewConverseStreamEventStream(func(es *bedrockruntime.ConverseStreamEventStream) {
		es.Reader = reader
	})

	go client.processStreamEvents(stream, chunkChan)

	var chunks []domain.StreamChunk
	for c := range chunkChan {
		chunks = append(chunks, c)
	}

	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk (done only), got %d", len(chunks))
	}
	if !chunks[0].Done {
		t.Error("Expected Done=true")
	}
}

// TestProcessStreamEvents_StreamError tests processing of a stream error.
func TestProcessStreamEvents_StreamError(t *testing.T) {
	client := newStreamTestClient()
	chunkChan := make(chan domain.StreamChunk, 10)

	events := make(chan types.ConverseStreamOutput)
	close(events)

	streamErr := fmt.Errorf("stream connection lost")
	reader := &mockConverseStreamOutputReader{events: events, err: streamErr}
	stream := bedrockruntime.NewConverseStreamEventStream(func(es *bedrockruntime.ConverseStreamEventStream) {
		es.Reader = reader
	})

	go client.processStreamEvents(stream, chunkChan)

	var chunks []domain.StreamChunk
	for c := range chunkChan {
		chunks = append(chunks, c)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected at least one chunk with error")
	}

	lastChunk := chunks[len(chunks)-1]
	if lastChunk.Error == nil {
		t.Error("Expected error in last chunk")
	}
	if !lastChunk.Done {
		t.Error("Expected Done=true on error chunk")
	}
}

// TestProcessStreamEvents_ContentDeltaNilDelta tests content delta with nil delta.
func TestProcessStreamEvents_ContentDeltaNilDelta(t *testing.T) {
	client := newStreamTestClient()
	chunkChan := make(chan domain.StreamChunk, 10)

	events := make(chan types.ConverseStreamOutput, 3)

	idx := int32(0)
	events <- &types.ConverseStreamOutputMemberContentBlockDelta{
		Value: types.ContentBlockDeltaEvent{
			ContentBlockIndex: &idx,
			Delta:             nil, // nil delta - should be skipped
		},
	}

	events <- &types.ConverseStreamOutputMemberMessageStop{
		Value: types.MessageStopEvent{},
	}
	close(events)

	reader := &mockConverseStreamOutputReader{events: events}
	stream := bedrockruntime.NewConverseStreamEventStream(func(es *bedrockruntime.ConverseStreamEventStream) {
		es.Reader = reader
	})

	go client.processStreamEvents(stream, chunkChan)

	var chunks []domain.StreamChunk
	for c := range chunkChan {
		chunks = append(chunks, c)
	}

	// Should only have the Done chunk (nil delta was skipped)
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk (done only), got %d", len(chunks))
	}
	if !chunks[0].Done {
		t.Error("Expected Done=true")
	}
}

// TestProcessStreamEvents_EmptyStream tests processing an immediately closed stream with no error.
func TestProcessStreamEvents_EmptyStream(t *testing.T) {
	client := newStreamTestClient()
	chunkChan := make(chan domain.StreamChunk, 10)

	events := make(chan types.ConverseStreamOutput)
	close(events)

	reader := &mockConverseStreamOutputReader{events: events, err: nil}
	stream := bedrockruntime.NewConverseStreamEventStream(func(es *bedrockruntime.ConverseStreamEventStream) {
		es.Reader = reader
	})

	go client.processStreamEvents(stream, chunkChan)

	var chunks []domain.StreamChunk
	for c := range chunkChan {
		chunks = append(chunks, c)
	}

	// Empty stream with no error should produce no chunks
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty stream, got %d", len(chunks))
	}
}
