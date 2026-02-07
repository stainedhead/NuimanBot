package tool

import (
	"context"
	"fmt"
	"sync"

	"nuimanbot/internal/domain"
)

// InMemoryRegistry is a simple in-memory implementation of the ToolRegistry.
type InMemoryRegistry struct {
	mu    sync.RWMutex
	tools map[string]domain.Tool
}

// NewInMemoryRegistry creates a new InMemoryRegistry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		tools: make(map[string]domain.Tool),
	}
}

// Register adds a tool to the registry.
func (r *InMemoryRegistry) Register(tool domain.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool == nil {
		return fmt.Errorf("cannot register nil tool")
	}

	if _, exists := r.tools[tool.Name()]; exists {
		return fmt.Errorf("tool with name '%s' already registered", tool.Name())
	}
	r.tools[tool.Name()] = tool
	return nil
}

// Get retrieves a tool by its unique name.
func (r *InMemoryRegistry) Get(name string) (domain.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool, nil
}

// List returns all registered tools.
func (r *InMemoryRegistry) List() []domain.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]domain.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ListForUser returns tools available to a specific user.
// For this simple implementation, it returns all tools.
func (r *InMemoryRegistry) ListForUser(ctx context.Context, userID string) ([]domain.Tool, error) {
	return r.List(), nil
}
