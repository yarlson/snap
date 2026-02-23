package ui

import (
	"bytes"
	"io"
	"sync"
)

// SwitchWriter wraps an io.Writer and toggles between pass-through and buffered
// modes. When paused, writes accumulate in an internal buffer. When resumed,
// buffered content flushes to the underlying writer and subsequent writes pass
// through directly. Thread-safe for concurrent writes and pause/resume.
type SwitchWriter struct {
	mu       sync.Mutex
	dest     io.Writer
	buf      bytes.Buffer
	paused   bool
	lfToCRLF bool
	lastCR   bool // tracks trailing \r across normalizeNewlines calls
}

// SwitchWriterOption configures optional behavior on a SwitchWriter.
type SwitchWriterOption func(*SwitchWriter)

// WithLFToCRLF normalizes bare LF bytes to CRLF on writes.
// This keeps cursor column behavior correct when terminal output processing
// is disabled (for example while stdin is in raw mode).
func WithLFToCRLF() SwitchWriterOption {
	return func(sw *SwitchWriter) {
		sw.lfToCRLF = true
	}
}

// NewSwitchWriter creates a SwitchWriter that writes to dest in pass-through mode.
func NewSwitchWriter(dest io.Writer, opts ...SwitchWriterOption) *SwitchWriter {
	sw := &SwitchWriter{dest: dest}
	for _, opt := range opts {
		opt(sw)
	}
	return sw
}

// Write implements io.Writer. When active, writes directly to dest. When paused,
// writes accumulate in the internal buffer.
func (sw *SwitchWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	normalized := sw.normalizeNewlines(p)
	if sw.paused {
		if _, err := sw.buf.Write(normalized); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	if err := writeAll(sw.dest, normalized); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Direct writes directly to the underlying writer, bypassing the buffer even
// when paused. Used for input prompt rendering that must always be visible.
func (sw *SwitchWriter) Direct(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if err := writeAll(sw.dest, sw.normalizeNewlines(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Pause switches to buffered mode. Subsequent writes accumulate in the buffer
// until Resume is called. Calling Pause when already paused is a no-op.
func (sw *SwitchWriter) Pause() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.paused = true
}

// Resume flushes buffered content to dest and switches back to pass-through mode.
// Calling Resume when already active is a no-op.
func (sw *SwitchWriter) Resume() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if !sw.paused {
		return
	}

	if sw.buf.Len() > 0 {
		//nolint:errcheck // Best-effort flush; buffer content is transient UI output.
		_ = writeAll(sw.dest, sw.buf.Bytes())
		sw.buf.Reset()
	}

	sw.paused = false
}

// IsPaused reports whether the writer is currently in buffered mode.
func (sw *SwitchWriter) IsPaused() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.paused
}

func (sw *SwitchWriter) normalizeNewlines(p []byte) []byte {
	if !sw.lfToCRLF || !bytes.Contains(p, []byte{'\n'}) {
		if len(p) > 0 {
			sw.lastCR = p[len(p)-1] == '\r'
		}
		return p
	}

	normalized := make([]byte, 0, len(p)+bytes.Count(p, []byte{'\n'}))
	prevCR := sw.lastCR
	for _, b := range p {
		if b == '\n' && !prevCR {
			normalized = append(normalized, '\r', '\n')
		} else {
			normalized = append(normalized, b)
		}
		prevCR = b == '\r'
	}
	sw.lastCR = prevCR

	return normalized
}

func writeAll(w io.Writer, p []byte) error {
	remaining := p
	for len(remaining) > 0 {
		n, err := w.Write(remaining)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		remaining = remaining[n:]
	}
	return nil
}
