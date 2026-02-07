package sqlite

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"nuimanbot/internal/domain"
)

// messageItem represents a message to be batched.
type messageItem struct {
	conversationID string
	userID         string
	platform       domain.Platform
	message        domain.StoredMessage
}

// MessageBatcher batches message writes for improved performance.
// It buffers messages and flushes them either when the buffer reaches maxSize
// or when the flush interval elapses.
type MessageBatcher struct {
	repo          *MessageRepository
	buffer        []messageItem
	maxSize       int
	flushInterval time.Duration
	ticker        *time.Ticker
	stopCh        chan struct{}
	flushCh       chan struct{}
	mu            sync.Mutex
	wg            sync.WaitGroup
}

// NewMessageBatcher creates a new message batcher.
// maxSize is the maximum number of messages to buffer before flushing.
// flushInterval is the maximum time to wait before flushing.
func NewMessageBatcher(repo *MessageRepository, maxSize int, flushInterval time.Duration) *MessageBatcher {
	b := &MessageBatcher{
		repo:          repo,
		buffer:        make([]messageItem, 0, maxSize),
		maxSize:       maxSize,
		flushInterval: flushInterval,
		ticker:        time.NewTicker(flushInterval),
		stopCh:        make(chan struct{}),
		flushCh:       make(chan struct{}, 1),
	}

	// Start background flusher
	b.wg.Add(1)
	go b.run()

	return b
}

// Add adds a message to the batch buffer.
// If the buffer reaches maxSize, it triggers an immediate flush.
func (b *MessageBatcher) Add(ctx context.Context, conversationID, userID string, platform domain.Platform, msg domain.StoredMessage) error {
	b.mu.Lock()
	b.buffer = append(b.buffer, messageItem{
		conversationID: conversationID,
		userID:         userID,
		platform:       platform,
		message:        msg,
	})
	shouldFlush := len(b.buffer) >= b.maxSize
	b.mu.Unlock()

	if shouldFlush {
		select {
		case b.flushCh <- struct{}{}:
		default:
			// Flush already scheduled
		}
	}

	return nil
}

// Flush immediately flushes all buffered messages.
func (b *MessageBatcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	items := make([]messageItem, len(b.buffer))
	copy(items, b.buffer)
	b.buffer = b.buffer[:0]
	b.mu.Unlock()

	return b.flushItems(ctx, items)
}

// Stop stops the batcher and flushes any remaining messages.
func (b *MessageBatcher) Stop() {
	close(b.stopCh)
	b.wg.Wait()

	// Final flush
	if err := b.Flush(context.Background()); err != nil {
		slog.Warn("Failed to flush messages on shutdown", "error", err)
	}
}

// run is the background goroutine that handles periodic flushes.
func (b *MessageBatcher) run() {
	defer b.wg.Done()

	for {
		select {
		case <-b.stopCh:
			return

		case <-b.ticker.C:
			// Time-based flush
			b.doFlush()

		case <-b.flushCh:
			// Size-based flush
			b.doFlush()
		}
	}
}

// doFlush performs the actual flush operation.
func (b *MessageBatcher) doFlush() {
	ctx := context.Background()

	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}

	items := make([]messageItem, len(b.buffer))
	copy(items, b.buffer)
	b.buffer = b.buffer[:0]
	b.mu.Unlock()

	if err := b.flushItems(ctx, items); err != nil {
		slog.Error("Failed to flush messages", "error", err, "count", len(items))
		// Messages are lost on error - in production, could implement retry logic or dead letter queue
	}
}

// flushItems writes all buffered items to the repository.
// Each message is saved in its own transaction (handled by SaveMessage).
// Note: For true batch atomicity, this could be refactored to use a single
// transaction, but that would require changes to SaveMessage's interface.
func (b *MessageBatcher) flushItems(ctx context.Context, items []messageItem) error {
	if len(items) == 0 {
		return nil
	}

	// Save each message (each SaveMessage call handles its own transaction)
	for _, item := range items {
		if err := b.repo.SaveMessage(ctx, item.conversationID, item.userID, item.platform, item.message); err != nil {
			// Return first error - remaining messages in batch are lost
			// In production, could implement retry queue or dead letter queue
			return err
		}
	}

	return nil
}
