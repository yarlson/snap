package workflow

import "sync"

// StepContext holds the currently-running step info for display in queue UI.
// It is safe for concurrent reads/writes from different goroutines.
type StepContext struct {
	mu      sync.RWMutex
	current int
	total   int
	name    string
}

// NewStepContext creates a StepContext with initial values.
func NewStepContext() *StepContext {
	return &StepContext{}
}

// Set updates the current step context.
func (c *StepContext) Set(current, total int, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = current
	c.total = total
	c.name = name
}

// Get returns the current step info.
func (c *StepContext) Get() (current, total int, name string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current, c.total, c.name
}
