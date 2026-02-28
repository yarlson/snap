package input_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/input"
	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/ui"
)

// mockStepInfo implements input.StepInfo for testing.
type mockStepInfo struct {
	current int
	total   int
	name    string
}

func (m *mockStepInfo) Get() (current, total int, name string) {
	return m.current, m.total, m.name
}

// syncWriter wraps a buffer with a mutex for thread-safe writes.
type syncWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.Write(p)
}

func TestReader_EnqueuesLines(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("first prompt\nsecond prompt\n")

	reader := input.NewReader(r, q)
	reader.Start()

	// Wait for reader to process input (EOF closes naturally).
	assert.Eventually(t, func() bool {
		return q.Len() == 2
	}, time.Second, 10*time.Millisecond)

	first, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "first prompt", first)

	second, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "second prompt", second)
}

func TestReader_SkipsEmptyLines(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("real prompt\n\n\n")

	reader := input.NewReader(r, q)
	reader.Start()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	// Only the non-empty line should be enqueued.
	assert.Equal(t, 1, q.Len())
}

func TestReader_TrimsWhitespace(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("  padded prompt  \n")

	reader := input.NewReader(r, q)
	reader.Start()

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "padded prompt", prompt)
}

func TestReader_StopsOnEOF(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("prompt\n")

	reader := input.NewReader(r, q)
	reader.Start()

	// Reader should finish after EOF.
	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)
}

func TestReader_StopsOnClose(t *testing.T) {
	q := queue.New()
	pr, pw := io.Pipe()

	reader := input.NewReader(pr, q)
	reader.Start()

	// Write a prompt then close.
	_, err := pw.Write([]byte("prompt\n"))
	require.NoError(t, err)
	pw.Close()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, q.Len())
}

func TestReader_ConcurrentReadsWhileEnqueuing(t *testing.T) {
	q := queue.New()
	pr, pw := io.Pipe()

	reader := input.NewReader(pr, q)
	reader.Start()

	// Write prompts one at a time.
	for range 5 {
		_, err := pw.Write([]byte("prompt\n"))
		require.NoError(t, err)
	}
	pw.Close()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 5, q.Len())
}

func TestReader_ShowsQueuedConfirmation(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("fix nil pointer\n")
	var buf strings.Builder

	info := &mockStepInfo{current: 3, total: 9, name: "Validate implementation"}
	reader := input.NewReader(r, q,
		input.WithOutput(&buf),
		input.WithStepInfo(info),
	)
	reader.Start()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	output := ui.StripColors(buf.String())
	assert.Contains(t, output, "📌 Queued", "Should show queued box title")
	assert.Contains(t, output, "fix nil pointer", "Should show prompt text")
	assert.Contains(t, output, "⏳ Waiting for Step 3/9: Validate implementation",
		"Should show waiting indicator")
	assert.Contains(t, output, "📋 1 prompt in queue", "Should show queue count")
}

func TestReader_ShowsQueueStatusOnEmptyEnter(t *testing.T) {
	q := queue.New()
	// Queue a prompt first, then send empty line.
	q.Enqueue("existing prompt")
	r := strings.NewReader("\n")
	var buf strings.Builder

	reader := input.NewReader(r, q,
		input.WithOutput(&buf),
	)
	reader.Start()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	output := ui.StripColors(buf.String())
	assert.Contains(t, output, "📋 Queue (1 prompt pending):", "Should show queue status")
	assert.Contains(t, output, "1. existing prompt", "Should list queued prompt")
}

func TestReader_ShowsEmptyQueueStatusOnEmptyEnter(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("\n")
	var buf strings.Builder

	reader := input.NewReader(r, q,
		input.WithOutput(&buf),
	)
	reader.Start()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	output := ui.StripColors(buf.String())
	assert.Contains(t, output, "📋 Queue empty — no prompts pending", "Should show empty queue message")
}

func TestReader_NoOutputWithoutOptions(t *testing.T) {
	q := queue.New()
	r := strings.NewReader("prompt\n\n")

	// No WithOutput or WithStepInfo — should not panic or output anything.
	reader := input.NewReader(r, q)
	reader.Start()

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 1, q.Len())
}

func TestReader_WithTerminal_EnqueuesOnCR(t *testing.T) {
	q := queue.New()
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	// WithTerminal uses raw reader (CR-delimited, byte-by-byte).
	// Pipe is not a real terminal so raw mode won't be set, but
	// the byte-by-byte read loop still runs.
	reader := input.NewReader(pr, q,
		input.WithTerminal(pr),
	)
	reader.Start()

	// Raw reader uses CR (\r) as line terminator.
	_, err = pw.WriteString("raw prompt\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "raw prompt", prompt)

	reader.Stop()
	pw.Close()
}

func TestReader_WithTerminal_ShowsQueueUI(t *testing.T) {
	q := queue.New()
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var buf bytes.Buffer
	var bufMu sync.Mutex
	bufWriter := &syncWriter{buf: &buf, mu: &bufMu}
	info := &mockStepInfo{current: 2, total: 10, name: "Code review"}

	reader := input.NewReader(pr, q,
		input.WithTerminal(pr),
		input.WithOutput(bufWriter),
		input.WithStepInfo(info),
	)
	reader.Start()

	_, err = pw.WriteString("fix tests\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	bufMu.Lock()
	output := ui.StripColors(buf.String())
	bufMu.Unlock()

	assert.Contains(t, output, "📌 Queued", "Should show queued box")
	assert.Contains(t, output, "fix tests", "Should show prompt text")
	assert.Contains(t, output, "Step 2/10: Code review", "Should show step context")

	reader.Stop()
	pw.Close()
}

func TestReader_WithTerminal_EmptyEnterShowsStatus(t *testing.T) {
	q := queue.New()
	q.Enqueue("pending prompt")
	pr, pw, err := os.Pipe()
	require.NoError(t, err)
	defer pr.Close()

	var buf strings.Builder

	reader := input.NewReader(pr, q,
		input.WithTerminal(pr),
		input.WithOutput(&buf),
	)
	reader.Start()

	// Send CR with no text, then a real prompt to confirm processing order.
	_, err = pw.WriteString("\rcheck\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 2
	}, time.Second, 10*time.Millisecond)

	output := ui.StripColors(buf.String())
	assert.Contains(t, output, "📋 Queue (1 prompt pending):", "Should show queue status")

	reader.Stop()
	pw.Close()
}

func TestReader_WithTerminal_StopCleansUp(t *testing.T) {
	q := queue.New()
	pr, pw, err := os.Pipe()
	require.NoError(t, err)

	reader := input.NewReader(pr, q,
		input.WithTerminal(pr),
	)
	reader.Start()

	// Stop should signal the reader to exit.
	reader.Stop()
	pw.Close() // Unblock the blocked Read so the goroutine can exit.

	assert.Eventually(t, func() bool {
		return reader.Done()
	}, time.Second, 10*time.Millisecond)

	pr.Close()
}
