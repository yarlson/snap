package input

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/yarlson/snap/internal/queue"
)

var (
	termIsTerminal = term.IsTerminal
	termMakeRaw    = term.MakeRaw
	termRestore    = term.Restore
)

const (
	keyEnter     = '\r'
	keyBackspace = 127
	keyCtrlC     = 3
	keyCtrlU     = 21
	keyCtrlW     = 23
	keyEsc       = 0x1b

	// maxLineLen is the maximum number of bytes allowed in a single input line.
	// Characters beyond this limit are silently dropped.
	maxLineLen = 4096
)

// rawReader reads byte-by-byte from a reader, assembling lines and enqueuing
// them on Enter. When backed by a terminal, echo is suppressed via raw mode.
// When inputMode is set, delegates composing/display to Mode for modal
// input with output buffering.
type rawReader struct {
	source    io.Reader
	queue     *queue.Queue
	stop      chan struct{}
	mu        sync.Mutex
	oldState  *term.State
	fd        int // terminal file descriptor; -1 if not a terminal.
	inputMode *Mode

	// Callbacks wired by Reader for queue UI display.
	onEnqueue    func(string)
	onEmptyEnter func()
}

// newRawReader creates a rawReader. If f is a terminal, raw mode will be
// enabled on run(). Otherwise it reads in byte mode without raw mode (useful
// for testing with pipes).
func newRawReader(f *os.File, q *queue.Queue, stop chan struct{}) *rawReader {
	return &rawReader{
		source: f,
		queue:  q,
		stop:   stop,
		fd:     int(f.Fd()),
	}
}

// run enables raw mode if backed by a terminal, then reads byte-by-byte.
// Returns when the stop channel is closed, Ctrl+C is pressed, EOF, or error.
// Terminal is restored before returning. A signal handler ensures the terminal
// is restored even if the process receives SIGINT or SIGTERM. A deferred panic
// handler restores the terminal before re-panicking to prevent leaving the
// terminal in raw mode on unexpected crashes.
func (rr *rawReader) run() error {
	if termIsTerminal(rr.fd) {
		oldState, err := termMakeRaw(rr.fd)
		if err != nil {
			return fmt.Errorf("failed to set raw terminal mode: %w", err)
		}
		rr.mu.Lock()
		rr.oldState = oldState
		rr.mu.Unlock()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		defer rr.restore()
	}

	// Restore terminal on panic before re-panicking.
	defer func() {
		if r := recover(); r != nil {
			rr.restore()
			panic(r)
		}
	}()

	return rr.readLoop()
}

// readLoop reads byte-by-byte, assembles lines, and enqueues on Enter.
func (rr *rawReader) readLoop() error {
	if rr.inputMode != nil {
		return rr.readLoopModal()
	}
	return rr.readLoopPlain()
}

// readLoopModal reads bytes and delegates to Mode for modal input.
func (rr *rawReader) readLoopModal() error {
	buf := make([]byte, 1)
	for {
		select {
		case <-rr.stop:
			return nil
		default:
		}

		n, err := rr.source.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}

		b := buf[0]
		switch b {
		case keyCtrlC:
			if rr.inputMode.IsComposing() {
				rr.inputMode.Cancel()
				continue
			}
			return nil

		case keyCtrlU:
			rr.inputMode.ClearLine()

		case keyCtrlW:
			rr.inputMode.DeleteWord()

		case keyEnter:
			if rr.inputMode.IsComposing() {
				text := rr.inputMode.Submit()
				if text == "" {
					continue
				}
				if !rr.queue.Enqueue(text) {
					fmt.Fprintf(os.Stderr, "prompt queue full (max %d), dropping input\n", queue.MaxPrompts)
					continue
				}
				if rr.onEnqueue != nil {
					rr.onEnqueue(text)
				}
			} else if rr.onEmptyEnter != nil {
				rr.onEmptyEnter()
			}

		case keyEsc:
			if rr.inputMode.IsComposing() {
				rr.inputMode.Cancel()
			} else {
				rr.consumeEscape(buf)
			}

		case keyBackspace:
			rr.inputMode.HandleBackspace()

		default:
			if b >= 32 || b == '\t' {
				rr.inputMode.HandleByte(b)
			}
		}
	}
}

// readLoopPlain reads bytes without modal input (original behavior).
func (rr *rawReader) readLoopPlain() error {
	var line []byte
	buf := make([]byte, 1)

	for {
		select {
		case <-rr.stop:
			return nil
		default:
		}

		n, err := rr.source.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}

		b := buf[0]
		switch b {
		case keyCtrlC:
			return nil

		case keyCtrlU:
			line = line[:0]

		case keyCtrlW:
			line = deleteWord(line)

		case keyEnter:
			text := string(line)
			line = line[:0]
			if text == "" {
				if rr.onEmptyEnter != nil {
					rr.onEmptyEnter()
				}
				continue
			}
			if !rr.queue.Enqueue(text) {
				fmt.Fprintf(os.Stderr, "prompt queue full (max %d), dropping input\n", queue.MaxPrompts)
				continue
			}
			if rr.onEnqueue != nil {
				rr.onEnqueue(text)
			}

		case keyEsc:
			rr.consumeEscape(buf)

		case keyBackspace:
			if len(line) > 0 {
				_, size := utf8.DecodeLastRune(line)
				line = line[:len(line)-size]
			}

		default:
			if (b >= 32 || b == '\t') && len(line) < maxLineLen {
				line = append(line, b)
			}
		}
	}
}

// deleteWord removes the last word from a byte slice, mimicking Ctrl+W behavior.
// It first skips trailing whitespace, then removes non-whitespace characters.
func deleteWord(line []byte) []byte {
	i := len(line)
	// Skip trailing whitespace.
	for i > 0 && isSpace(line[i-1]) {
		i--
	}
	// Delete back to the previous whitespace.
	for i > 0 && !isSpace(line[i-1]) {
		i--
	}
	return line[:i]
}

// isSpace reports whether b is an ASCII whitespace character relevant to word
// boundaries (space or tab).
func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

// consumeEscape reads and discards the remainder of an ANSI escape sequence.
// This prevents arrow keys, Home, End, etc. from injecting garbage characters
// into the line buffer when the terminal is in raw mode.
func (rr *rawReader) consumeEscape(buf []byte) {
	b, ok := rr.readByte(buf)
	if !ok {
		return
	}
	if b != '[' {
		// Not a CSI sequence (e.g., Alt+key sends ESC followed by key).
		return
	}
	// CSI sequence: ESC [ (parameter bytes 0x30-0x3F)* (intermediate bytes 0x20-0x2F)* (final byte 0x40-0x7E)
	for {
		b, ok = rr.readByte(buf)
		if !ok {
			return
		}
		if b >= 0x40 && b <= 0x7E {
			return // final byte — sequence complete
		}
	}
}

// readByte reads a single byte from the source into buf and returns it.
func (rr *rawReader) readByte(buf []byte) (byte, bool) {
	n, err := rr.source.Read(buf)
	if err != nil || n == 0 {
		return 0, false
	}
	return buf[0], true
}

// restore returns the terminal to its original state. Safe to call multiple
// times; subsequent calls after the first are no-ops.
func (rr *rawReader) restore() {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if rr.oldState != nil {
		//nolint:errcheck // Best-effort terminal restore; nothing to do on failure.
		termRestore(rr.fd, rr.oldState)
		rr.oldState = nil
	}
}
