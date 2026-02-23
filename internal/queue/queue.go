// Package queue provides a thread-safe FIFO prompt queue.
package queue

import "sync"

// MaxPrompts is the maximum number of prompts that can be queued.
// Beyond this limit, new prompts are dropped to prevent runaway resource usage.
const MaxPrompts = 100

// Queue is a thread-safe FIFO queue for user prompts.
type Queue struct {
	mu    sync.Mutex
	items []string
}

// New creates an empty prompt queue.
func New() *Queue {
	return &Queue{}
}

// Enqueue adds a prompt to the back of the queue.
// Returns false if the queue is full (at MaxPrompts capacity).
func (q *Queue) Enqueue(prompt string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= MaxPrompts {
		return false
	}
	q.items = append(q.items, prompt)
	return true
}

// Dequeue removes and returns the front prompt. Returns false if empty.
func (q *Queue) Dequeue() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return "", false
	}
	prompt := q.items[0]
	q.items[0] = "" // clear reference for GC
	q.items = q.items[1:]
	return prompt, true
}

// Len returns the number of queued prompts.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// DrainAll removes and returns all queued prompts in FIFO order. Returns nil if empty.
func (q *Queue) DrainAll() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	prompts := q.items
	q.items = nil
	return prompts
}

// All returns a copy of all queued prompts without removing them. Returns nil if empty.
func (q *Queue) All() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	result := make([]string, len(q.items))
	copy(result, q.items)
	return result
}
