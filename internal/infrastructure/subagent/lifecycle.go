package subagent

import (
	"context"
	"errors"
	"fmt"
	"nuimanbot/internal/domain"
	"sync"
	"time"
)

// MonitoringHook is a callback for subagent status changes
type MonitoringHook func(subagentID string, status domain.SubagentStatus)

// runningSubagent tracks a running subagent execution
type runningSubagent struct {
	context    domain.SubagentContext
	result     *domain.SubagentResult
	cancelFunc context.CancelFunc
	startTime  time.Time
	mu         sync.RWMutex
}

// LifecycleManager manages the lifecycle of subagent executions
type LifecycleManager struct {
	executor       domain.SubagentExecutor
	running        map[string]*runningSubagent
	mu             sync.RWMutex
	monitoringHook MonitoringHook
	hookMu         sync.RWMutex
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(executor domain.SubagentExecutor) *LifecycleManager {
	return &LifecycleManager{
		executor: executor,
		running:  make(map[string]*runningSubagent),
	}
}

// Start begins execution of a subagent in the background
func (m *LifecycleManager) Start(ctx context.Context, subagentCtx domain.SubagentContext) error {
	// Validate subagent context
	if err := subagentCtx.Validate(); err != nil {
		return fmt.Errorf("invalid subagent context: %w", err)
	}

	m.mu.Lock()
	// Check if already running
	if _, exists := m.running[subagentCtx.ID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("subagent %s is already running", subagentCtx.ID)
	}

	// Create cancellable context with timeout
	execCtx, cancel := context.WithTimeout(ctx, subagentCtx.ResourceLimits.Timeout)

	// Create running entry
	running := &runningSubagent{
		context:    subagentCtx,
		result:     &domain.SubagentResult{SubagentID: subagentCtx.ID, Status: domain.SubagentStatusRunning},
		cancelFunc: cancel,
		startTime:  time.Now(),
	}

	m.running[subagentCtx.ID] = running
	m.mu.Unlock()

	// Call monitoring hook
	m.callHook(subagentCtx.ID, domain.SubagentStatusRunning)

	// Execute in background
	go func() {
		defer cancel()

		result, err := m.executor.Execute(execCtx, subagentCtx)

		// Update result
		running.mu.Lock()
		if err != nil {
			running.result = &domain.SubagentResult{
				SubagentID:   subagentCtx.ID,
				Status:       domain.SubagentStatusError,
				ErrorMessage: err.Error(),
			}
		} else {
			running.result = result
		}
		running.mu.Unlock()

		// Call monitoring hook with final status
		if running.result != nil {
			m.callHook(subagentCtx.ID, running.result.Status)
		}
	}()

	return nil
}

// Cancel stops a running subagent
func (m *LifecycleManager) Cancel(ctx context.Context, subagentID string) error {
	m.mu.RLock()
	running, exists := m.running[subagentID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("subagent %s not found", subagentID)
	}

	// Cancel the execution context
	running.cancelFunc()

	// Update status
	running.mu.Lock()
	if running.result.Status == domain.SubagentStatusRunning {
		running.result.Status = domain.SubagentStatusCancelled
		running.result.ErrorMessage = "cancelled by user"
	}
	running.mu.Unlock()

	// Call monitoring hook
	m.callHook(subagentID, domain.SubagentStatusCancelled)

	return nil
}

// GetStatus retrieves the current status of a subagent
func (m *LifecycleManager) GetStatus(ctx context.Context, subagentID string) (*domain.SubagentResult, error) {
	m.mu.RLock()
	running, exists := m.running[subagentID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("subagent %s not found", subagentID)
	}

	running.mu.RLock()
	defer running.mu.RUnlock()

	// Return a copy of the result
	result := *running.result
	return &result, nil
}

// ListRunning returns IDs of all currently running subagents
func (m *LifecycleManager) ListRunning(ctx context.Context) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var running []string
	for id, r := range m.running {
		r.mu.RLock()
		if r.result.Status == domain.SubagentStatusRunning {
			running = append(running, id)
		}
		r.mu.RUnlock()
	}

	return running
}

// SetMonitoringHook sets a callback for status changes
func (m *LifecycleManager) SetMonitoringHook(hook MonitoringHook) {
	m.hookMu.Lock()
	defer m.hookMu.Unlock()
	m.monitoringHook = hook
}

// Shutdown gracefully stops all running subagents
func (m *LifecycleManager) Shutdown(ctx context.Context) error {
	m.mu.RLock()
	runningIDs := make([]string, 0, len(m.running))
	for id := range m.running {
		runningIDs = append(runningIDs, id)
	}
	m.mu.RUnlock()

	// Cancel all running subagents
	for _, id := range runningIDs {
		if err := m.Cancel(ctx, id); err != nil {
			// Log but continue
			continue
		}
	}

	// Wait for all to finish or timeout
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(5 * time.Second)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			running := m.ListRunning(context.Background())
			if len(running) == 0 {
				return nil
			}
			if time.Now().After(deadline) {
				return errors.New("shutdown timeout: some subagents still running")
			}
		}
	}
}

// callHook calls the monitoring hook if set
func (m *LifecycleManager) callHook(subagentID string, status domain.SubagentStatus) {
	m.hookMu.RLock()
	hook := m.monitoringHook
	m.hookMu.RUnlock()

	if hook != nil {
		hook(subagentID, status)
	}
}
