// Package input reads lines from stdin and enqueues them as user prompts.
package input

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yarlson/snap/internal/queue"
	"github.com/yarlson/snap/internal/ui"
)

// StepInfo provides current step context for queue display.
type StepInfo interface {
	Get() (current, total int, name string)
}

// Reader reads lines from an io.Reader and enqueues non-empty lines.
// When backed by a terminal (via WithTerminal), it uses raw mode to suppress
// echo and prevent garbled output during streaming. When a Mode is provided
// (via WithMode), typing activates modal input with output buffering for a
// clean composing experience.
type Reader struct {
	source    io.Reader
	terminal  *os.File // set via WithTerminal for raw mode reading.
	output    io.Writer
	queue     *queue.Queue
	stepInfo  StepInfo
	inputMode *Mode
	mu        sync.Mutex
	raw       *rawReader
	done      atomic.Bool
	stop      chan struct{}
}

// NewReader creates a Reader that reads from source and enqueues into q.
// If output or stepInfo is nil, queue UI feedback is disabled.
func NewReader(source io.Reader, q *queue.Queue, opts ...ReaderOption) *Reader {
	r := &Reader{
		source: source,
		queue:  q,
		stop:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ReaderOption configures optional Reader behavior.
type ReaderOption func(*Reader)

// WithOutput sets the writer for queue UI display.
func WithOutput(w io.Writer) ReaderOption {
	return func(r *Reader) {
		r.output = w
	}
}

// WithStepInfo provides current step context for queue display.
func WithStepInfo(info StepInfo) ReaderOption {
	return func(r *Reader) {
		r.stepInfo = info
	}
}

// WithTerminal enables raw terminal mode for the given file. When set,
// the reader suppresses terminal echo and reads character-by-character,
// preventing garbled output during streaming.
func WithTerminal(f *os.File) ReaderOption {
	return func(r *Reader) {
		r.terminal = f
	}
}

// WithMode enables modal input. When set, the first printable keystroke
// pauses output and shows an input prompt; Enter submits, Escape cancels.
func WithMode(m *Mode) ReaderOption {
	return func(r *Reader) {
		r.inputMode = m
	}
}

// Start begins reading lines in a background goroutine. Returns immediately.
func (r *Reader) Start() {
	if r.terminal != nil {
		go r.rawLoop()
	} else {
		go r.readLoop()
	}
}

// Stop signals the reader to stop. The goroutine will exit after the current
// blocking read completes (e.g., on next newline or EOF). For os.Stdin, the
// goroutine may not exit until the process ends because Scan blocks on input.
// When using raw terminal mode, Stop restores terminal settings immediately
// even if the goroutine remains blocked on the current read.
func (r *Reader) Stop() {
	select {
	case <-r.stop:
		// Already closed.
	default:
		close(r.stop)
	}

	r.mu.Lock()
	raw := r.raw
	r.mu.Unlock()
	if raw != nil {
		raw.restore()
	}
}

// Done returns true if the reader has stopped (EOF or error).
func (r *Reader) Done() bool {
	return r.done.Load()
}

// rawLoop runs the raw terminal reader for echo-suppressed input.
func (r *Reader) rawLoop() {
	defer r.done.Store(true)

	rr := newRawReader(r.terminal, r.queue, r.stop)
	r.mu.Lock()
	r.raw = rr
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if r.raw == rr {
			r.raw = nil
		}
		r.mu.Unlock()
	}()
	rr.onEnqueue = r.showQueuedConfirmation
	rr.onEmptyEnter = r.showQueueStatus
	rr.inputMode = r.inputMode

	if err := rr.run(); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "raw reader stopped: %v\n", err)
	}
}

// readLoop runs the line-based scanner for pipe/non-terminal input.
func (r *Reader) readLoop() {
	defer r.done.Store(true)

	scanner := bufio.NewScanner(r.source)
	for scanner.Scan() {
		select {
		case <-r.stop:
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			r.showQueueStatus()
			continue
		}
		if !r.queue.Enqueue(line) {
			fmt.Fprintf(os.Stderr, "prompt queue full (max %d), dropping input\n", queue.MaxPrompts)
			continue
		}
		r.showQueuedConfirmation(line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stdin reader stopped: %v\n", err)
	}
}

// showQueuedConfirmation prints the boxed acknowledgment for a queued prompt.
// Note: queueLen may be stale if DrainQueue runs concurrently between Enqueue
// and Len — this is acceptable for a UI hint.
func (r *Reader) showQueuedConfirmation(prompt string) {
	if r.output == nil || r.stepInfo == nil {
		return
	}
	current, total, name := r.stepInfo.Get()
	queueLen := r.queue.Len()
	// QueuedPrompt also calls StripColors internally as defense-in-depth.
	//nolint:gosec // G705: prompt is sanitized by StripColors before output
	fmt.Fprint(r.output, ui.QueuedPrompt(ui.StripColors(prompt), current, total, name, queueLen))
}

// showQueueStatus prints the current queue contents on empty Enter.
func (r *Reader) showQueueStatus() {
	if r.output == nil {
		return
	}
	prompts := r.queue.All()
	fmt.Fprint(r.output, ui.QueueStatus(prompts))
}
