package queue_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/queue"
)

func TestQueue_EnqueueDequeue(t *testing.T) {
	q := queue.New()

	require.True(t, q.Enqueue("first prompt"))
	require.True(t, q.Enqueue("second prompt"))

	assert.Equal(t, 2, q.Len())

	first, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "first prompt", first)

	second, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "second prompt", second)

	_, ok = q.Dequeue()
	assert.False(t, ok)
}

func TestQueue_DequeueEmpty(t *testing.T) {
	q := queue.New()

	prompt, ok := q.Dequeue()
	assert.False(t, ok)
	assert.Empty(t, prompt)
}

func TestQueue_Len(t *testing.T) {
	q := queue.New()
	assert.Equal(t, 0, q.Len())

	q.Enqueue("one")
	assert.Equal(t, 1, q.Len())

	q.Enqueue("two")
	assert.Equal(t, 2, q.Len())

	q.Dequeue()
	assert.Equal(t, 1, q.Len())
}

func TestQueue_DrainAll(t *testing.T) {
	q := queue.New()

	q.Enqueue("first")
	q.Enqueue("second")
	q.Enqueue("third")

	prompts := q.DrainAll()
	assert.Equal(t, []string{"first", "second", "third"}, prompts)
	assert.Equal(t, 0, q.Len())
}

func TestQueue_DrainAllEmpty(t *testing.T) {
	q := queue.New()

	prompts := q.DrainAll()
	assert.Nil(t, prompts)
	assert.Equal(t, 0, q.Len())
}

func TestQueue_All(t *testing.T) {
	q := queue.New()

	q.Enqueue("first")
	q.Enqueue("second")

	// All returns a copy without removing items.
	prompts := q.All()
	assert.Equal(t, []string{"first", "second"}, prompts)
	assert.Equal(t, 2, q.Len(), "All should not remove items from queue")
}

func TestQueue_AllEmpty(t *testing.T) {
	q := queue.New()
	prompts := q.All()
	assert.Nil(t, prompts)
}

func TestQueue_ConcurrentAccess(t *testing.T) {
	q := queue.New()
	const goroutines = queue.MaxPrompts

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Concurrent enqueue.
	for range goroutines {
		go func() {
			defer wg.Done()
			q.Enqueue("prompt")
			_ = q.Len()
		}()
	}

	wg.Wait()
	assert.Equal(t, goroutines, q.Len())

	// Concurrent dequeue.
	wg.Add(goroutines)
	dequeued := make(chan string, goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if prompt, ok := q.Dequeue(); ok {
				dequeued <- prompt
			}
		}()
	}

	wg.Wait()
	close(dequeued)

	count := 0
	for range dequeued {
		count++
	}
	assert.Equal(t, goroutines, count)
	assert.Equal(t, 0, q.Len())
}

func TestQueue_FIFOOrder(t *testing.T) {
	q := queue.New()

	for i := range 10 {
		q.Enqueue(string(rune('a' + i)))
	}

	for i := range 10 {
		prompt, ok := q.Dequeue()
		require.True(t, ok)
		assert.Equal(t, string(rune('a'+i)), prompt)
	}
}

func TestQueue_EnqueueFull(t *testing.T) {
	q := queue.New()

	// Fill to capacity.
	for range queue.MaxPrompts {
		require.True(t, q.Enqueue("prompt"))
	}

	// Next enqueue should fail.
	assert.False(t, q.Enqueue("overflow"))
	assert.Equal(t, queue.MaxPrompts, q.Len())

	// Draining frees space.
	q.DrainAll()
	assert.True(t, q.Enqueue("after drain"))
	assert.Equal(t, 1, q.Len())
}
