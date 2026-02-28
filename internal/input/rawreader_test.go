package input

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/ui"
)

func TestRawReader_EnqueuesOnEnter(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("hello\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_BackspaceRemovesCharacter(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("helloo\x7f\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_EmptyEnterDoesNotEnqueue(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("\rhello\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_EmptyEnterCallsCallback(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	called := false
	rr.onEmptyEnter = func() {
		called = true
	}

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("\rdone\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.True(t, called, "onEmptyEnter callback should have been called")

	close(stop)
	w.Close()
}

func TestRawReader_EnqueueCallsCallback(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	var enqueuedPrompt string
	var enqueueMu sync.Mutex
	rr.onEnqueue = func(text string) {
		enqueueMu.Lock()
		enqueuedPrompt = text
		enqueueMu.Unlock()
	}

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("fix bug\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	enqueueMu.Lock()
	defer enqueueMu.Unlock()
	assert.Equal(t, "fix bug", enqueuedPrompt)

	close(stop)
	w.Close()
}

func TestRawReader_MultipleLines(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("first\rsecond\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 2
	}, time.Second, 10*time.Millisecond)

	first, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "first", first)

	second, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "second", second)

	close(stop)
	w.Close()
}

func TestRawReader_CtrlCStops(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString(string(rune(keyCtrlC)))
	require.NoError(t, err)

	select {
	case err := <-done:
		assert.NoError(t, err, "Ctrl+C should return nil error")
	case <-time.After(time.Second):
		t.Fatal("rawReader did not stop on Ctrl+C")
	}

	w.Close()
}

func TestRawReader_StopChannelStops(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	close(stop)
	w.Close()
	r.Close()

	select {
	case <-done:
		// Stopped successfully.
	case <-time.After(time.Second):
		t.Fatal("rawReader did not stop on channel close")
	}
}

func TestRawReader_BackspaceOnEmptyLine(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("\x7f\x7fhello\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_IgnoresControlCharacters(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("he\x01\x02llo\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_QueueFullDropsInput(t *testing.T) {
	q := queue.New()
	for range queue.MaxPrompts {
		q.Enqueue("filler")
	}

	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("overflow\r")
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, queue.MaxPrompts, q.Len())

	close(stop)
	w.Close()
}

func TestRawReader_TabIsAccepted(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("hello\tworld\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello\tworld", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_NilCallbacksSafe(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	_, err = w.WriteString("\rhello\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	close(stop)
	w.Close()
}

func TestRawReader_BackspaceRemovesMultiByteUTF8(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Type "café" then backspace (should remove the é, which is 2 bytes), then "e".
	_, err = w.WriteString("caf\xc3\xa9\x7fe\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "cafe", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_EscapeSequenceDiscarded(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Type "hi", then Up arrow (ESC [ A), then "lo", then Enter.
	// The escape sequence should be discarded, leaving "hilo".
	_, err = w.WriteString("hi\x1b[Alo\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hilo", prompt)

	close(stop)
	w.Close()
}

// --- Modal input mode tests ---

func newModalRawReader(t *testing.T) (*rawReader, *os.File, *queue.Queue, *ui.SwitchWriter, *bytes.Buffer) {
	t.Helper()
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)
	im := NewMode(sw)

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)
	rr.inputMode = im

	t.Cleanup(func() {
		close(stop)
		w.Close()
		r.Close()
	})

	return rr, w, q, sw, &buf
}

func TestRawReaderModal_EnqueuesOnEnter(t *testing.T) {
	rr, w, q, _, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	_, err := w.WriteString("fix bug\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "fix bug", prompt)
}

func TestRawReaderModal_PausesOutputDuringComposing(t *testing.T) {
	rr, w, _, sw, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Type a character — should enter composing and pause output.
	_, err := w.WriteString("h")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return sw.IsPaused()
	}, time.Second, 10*time.Millisecond)
}

func TestRawReaderModal_ResumesAfterSubmit(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	_, err := w.WriteString("go\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.False(t, sw.IsPaused(), "Output should resume after submit")
}

func TestRawReaderModal_EscapeCancelsInput(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Type "abc", then Escape (bare ESC, not a CSI sequence — just ESC alone).
	// Escape without a following '[' is treated as cancel.
	_, err := w.WriteString("abc\x1b")
	require.NoError(t, err)

	// Wait for composing to start then cancel.
	assert.Eventually(t, func() bool {
		return !sw.IsPaused()
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, 0, q.Len(), "Nothing should be enqueued on cancel")
}

func TestRawReaderModal_CtrlCInComposingCancels(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	done := make(chan error, 1)
	go func() { done <- rr.run() }()

	// Type "x" then Ctrl+C — should cancel input, not stop reader.
	_, err := w.WriteString("x\x03")
	require.NoError(t, err)

	// Reader should still be running. Verify by enqueuing another prompt.
	time.Sleep(50 * time.Millisecond)
	_, err = w.WriteString("y\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.False(t, sw.IsPaused())

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "y", prompt)
}

func TestRawReaderModal_CtrlCInIdleStopsReader(t *testing.T) {
	rr, w, _, _, _ := newModalRawReader(t)

	done := make(chan error, 1)
	go func() { done <- rr.run() }()

	_, err := w.WriteString(string(rune(keyCtrlC)))
	require.NoError(t, err)

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("rawReader did not stop on Ctrl+C in idle")
	}
}

func TestRawReaderModal_BackspaceWorks(t *testing.T) {
	rr, w, q, _, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	_, err := w.WriteString("helloo\x7f\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)
}

func TestRawReaderModal_EmptyEnterInIdleCallsCallback(t *testing.T) {
	rr, w, _, _, _ := newModalRawReader(t)

	called := false
	rr.onEmptyEnter = func() {
		called = true
	}

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Enter with no preceding characters (idle mode).
	_, err := w.WriteString("\r")
	require.NoError(t, err)

	// Follow up to ensure processing order.
	_, err = w.WriteString("next\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return rr.queue.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.True(t, called, "onEmptyEnter should be called on Enter in idle mode")
}

func TestRawReaderModal_ShowsInputPrompt(t *testing.T) {
	rr, w, q, _, buf := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	_, err := w.WriteString("hi\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	output := ui.StripColors(buf.String())
	assert.Contains(t, output, "❯ h", "Should show input prompt with first char")
}

// --- Ctrl+U (clear line) tests ---

func TestRawReader_CtrlU_ClearsLine(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Type "hello", Ctrl+U (clear), then "world", Enter.
	_, err = w.WriteString("hello\x15world\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "world", prompt)

	close(stop)
	w.Close()
}

func TestRawReaderModal_CtrlU_ClearsLine(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Type "hello", Ctrl+U (clear line), then "fix\r".
	_, err := w.WriteString("hello\x15fix\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "fix", prompt)
	assert.False(t, sw.IsPaused(), "Output should resume after submit")
}

func TestRawReaderModal_CtrlU_OnEmptyLineCancels(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	done := make(chan error, 1)
	go func() { done <- rr.run() }()

	// Type "a", backspace (removes 'a', line empty), then Ctrl+U → should cancel composing.
	_, err := w.WriteString("a\x7f\x15")
	require.NoError(t, err)

	// Reader should still be running — verify by submitting a new prompt.
	time.Sleep(50 * time.Millisecond)
	_, err = w.WriteString("ok\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.False(t, sw.IsPaused())
	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "ok", prompt)
}

// --- Ctrl+W (delete word) tests ---

func TestRawReader_CtrlW_DeletesWord(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Type "hello world", Ctrl+W (delete "world"), type "there", Enter.
	_, err = w.WriteString("hello world\x17there\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello there", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_CtrlW_DeletesOnlyWord(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Type "hello", Ctrl+W (deletes "hello"), type "world", Enter.
	_, err = w.WriteString("hello\x17world\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "world", prompt)

	close(stop)
	w.Close()
}

func TestRawReader_CtrlW_OnEmptyLineIsNoop(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Ctrl+W on empty line, then "hello", Enter.
	_, err = w.WriteString("\x17hello\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello", prompt)

	close(stop)
	w.Close()
}

func TestRawReaderModal_CtrlW_DeletesWord(t *testing.T) {
	rr, w, q, _, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Type "hello world", Ctrl+W (delete "world"), type "fix", Enter.
	_, err := w.WriteString("hello world\x17fix\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "hello fix", prompt)
}

func TestRawReaderModal_CtrlW_DeletesOnlyWordCancelsIfEmpty(t *testing.T) {
	rr, w, q, sw, _ := newModalRawReader(t)

	done := make(chan error, 1)
	go func() { done <- rr.run() }()

	// Type "hello", Ctrl+W (deletes "hello", line empty → cancel composing).
	_, err := w.WriteString("hello\x17")
	require.NoError(t, err)

	// Reader should still be running.
	time.Sleep(50 * time.Millisecond)
	_, err = w.WriteString("ok\r")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, time.Second, 10*time.Millisecond)

	assert.False(t, sw.IsPaused())
	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, "ok", prompt)
}

// --- Long line handling tests ---

func TestRawReader_LongLineIsTruncated(t *testing.T) {
	q := queue.New()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)

	done := make(chan error, 1)
	go func() {
		done <- rr.run()
	}()

	// Write a line exceeding maxLineLen, then Enter.
	longInput := make([]byte, maxLineLen+100, maxLineLen+101)
	for i := range longInput {
		longInput[i] = 'a'
	}
	longInput = append(longInput, '\r')
	_, err = w.Write(longInput)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, 5*time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, maxLineLen, len(prompt), "Prompt should be truncated to maxLineLen")

	close(stop)
	w.Close()
}

func TestRawReaderModal_LongLineIsTruncated(t *testing.T) {
	rr, w, q, _, _ := newModalRawReader(t)

	//nolint:errcheck // Background goroutine; error checked via queue assertions.
	go func() { rr.run() }()

	// Write a line exceeding maxLineLen, then Enter.
	// The modal path processes each byte individually and triggers redrawLine()
	// once the line exceeds terminal width, so this needs a generous timeout
	// (especially under the race detector on slow CI runners).
	longInput := make([]byte, maxLineLen+100, maxLineLen+101)
	for i := range longInput {
		longInput[i] = 'b'
	}
	longInput = append(longInput, '\r')
	_, err := w.Write(longInput)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return q.Len() == 1
	}, 5*time.Second, 10*time.Millisecond)

	prompt, ok := q.Dequeue()
	require.True(t, ok)
	assert.Equal(t, maxLineLen, len(prompt), "Modal prompt should be truncated to maxLineLen")
}

// --- Panic recovery test ---

// panicReader is an io.Reader that panics on Read, used to test panic recovery.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("reader panic test") }

func TestRawReader_PanicRecoveryRestoresTerminal(t *testing.T) {
	// When run() wraps with panic recovery, panics during readLoop should
	// still restore the terminal and re-panic. Since pipes aren't real
	// terminals, we just verify the recovery + re-panic mechanism works
	// by injecting a reader that panics on Read().
	q := queue.New()
	r, _, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	stop := make(chan struct{})
	rr := newRawReader(r, q, stop)
	// Replace the source with a panicking reader so the readLoop panics.
	rr.source = panicReader{}

	assert.Panics(t, func() {
		//nolint:errcheck // Testing panic recovery; error return is irrelevant.
		rr.run()
	}, "Should re-panic after terminal restore")
}
