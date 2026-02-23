package input

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yarlson/snap/internal/ui"
)

// promptPrefix is the styled input prompt rendered via design tokens.
var promptPrefix = fmt.Sprintf("%s%s❯ %s",
	ui.ResolveColor(ui.ColorSecondary),
	ui.ResolveStyle(ui.WeightBold),
	ui.ResolveStyle(ui.WeightNormal),
)

// promptPrefixLen is the visible character width of the prompt prefix ("❯ ").
const promptPrefixLen = 2 // "❯" (1 column) + " " (1 column)

// Mode is a state machine that manages modal input. In idle state, output
// streams freely and keystrokes are invisible. On the first printable byte,
// it transitions to composing: output pauses (via SwitchWriter), a styled
// "❯ " prompt appears, and subsequent bytes echo visibly. Enter submits the
// line, Escape cancels it, and both flush buffered output and resume streaming.
//
// Long lines that exceed terminal width are truncated for display with an
// ellipsis prefix (…) while the full text is kept in the buffer. Terminal
// width is tracked via SetTermWidth to handle SIGWINCH events.
//
// Mode is NOT thread-safe; all methods must be called from a single goroutine
// (the rawReader goroutine).
type Mode struct {
	sw        *ui.SwitchWriter
	composing bool
	line      []byte
	termWidth int // Updated via SetTermWidth; 0 means unknown width
}

// NewMode creates a Mode that controls the given SwitchWriter.
func NewMode(sw *ui.SwitchWriter) *Mode {
	return &Mode{sw: sw, termWidth: 80} // Default terminal width
}

// IsComposing reports whether the user is actively typing input.
func (m *Mode) IsComposing() bool {
	return m.composing
}

// Line returns the current input line as a string.
func (m *Mode) Line() string {
	return string(m.line)
}

// SetTermWidth updates the terminal width. This is called when SIGWINCH is received.
func (m *Mode) SetTermWidth(w int) {
	if w > 0 {
		m.termWidth = w
	}
}

// getDisplayText returns the text to display, truncating if necessary to fit terminal width.
// If the line is longer than available space, returns "…" + tail of line.
// Available space is termWidth minus prompt prefix length (2 for "❯ ").
func (m *Mode) getDisplayText() string {
	const ellipsis = "…"
	const ellipsisWidth = 1 // "…" is 1 character wide
	const promptWidth = promptPrefixLen

	if m.termWidth == 0 {
		// If terminal width is unknown, don't truncate.
		return string(m.line)
	}

	availableWidth := m.termWidth - promptWidth
	if availableWidth <= 0 {
		// Terminal too narrow; show only what fits.
		return string(m.line)
	}

	lineWidth := utf8.RuneCount(m.line)
	if lineWidth <= availableWidth {
		return string(m.line)
	}

	// Line is too long; show tail with ellipsis prefix.
	tailWidth := availableWidth - ellipsisWidth
	if tailWidth <= 0 {
		// No room for tail; just show ellipsis.
		return ellipsis
	}

	// Find the start of the tail that fits.
	tailStart := lineWidth - tailWidth
	if tailStart < 0 {
		tailStart = 0
	}

	// Convert rune offset to byte offset.
	byteOffset := 0
	runeCount := 0
	for i := 0; i < len(m.line); i++ {
		if (m.line[i] & 0xC0) != 0x80 {
			runeCount++
		}
		if runeCount == tailStart {
			byteOffset = i
			break
		}
	}

	return ellipsis + string(m.line[byteOffset:])
}

// HandleByte processes a single byte of input. If idle, the first printable
// byte activates composing mode. In composing mode, bytes are appended to the
// input line and echoed via Direct write on the SwitchWriter. Bytes beyond
// maxLineLen are silently dropped.
//
// When a byte is added and the line would exceed the terminal width, the entire
// line is redrawn with proper truncation instead of just echoing the new byte.
func (m *Mode) HandleByte(b byte) {
	if !m.composing {
		m.activate(b)
		return
	}
	if len(m.line) >= maxLineLen {
		return
	}
	m.line = append(m.line, b)

	// Check if line is now long enough to require truncation display.
	// If so, redraw the entire line; otherwise just echo the byte.
	lineWidth := utf8.RuneCount(m.line)
	availableWidth := m.termWidth - promptPrefixLen
	if lineWidth > availableWidth && m.termWidth > 0 {
		// Line is now too long for terminal; redraw with truncation.
		m.redrawLine()
	} else {
		// Line still fits; just echo the byte.
		m.echo(string([]byte{b}))
	}
}

// HandleBackspace removes the last UTF-8 rune from the input line. If the line
// becomes empty, a second backspace cancels composing mode entirely.
func (m *Mode) HandleBackspace() {
	if !m.composing {
		return
	}
	if len(m.line) == 0 {
		m.clearPrompt()
		m.composing = false
		m.sw.Resume()
		return
	}
	_, size := utf8.DecodeLastRune(m.line)
	if size == 0 {
		return
	}
	m.line = m.line[:len(m.line)-size]
	// Erase the character visually: move cursor back, write space, move back.
	m.echo("\b \b")
}

// Submit finalizes the current input and returns the text. Clears the prompt
// line, resets to idle, and resumes output streaming. Returns empty string
// if not composing.
func (m *Mode) Submit() string {
	if !m.composing {
		return ""
	}
	text := string(m.line)
	m.clearPrompt()
	m.line = m.line[:0]
	m.composing = false
	m.sw.Resume()
	return text
}

// Cancel discards the current input, clears the prompt, resets to idle,
// and resumes output streaming. Shows a brief "cancelled" message before clearing.
func (m *Mode) Cancel() {
	if !m.composing {
		return
	}
	// Show cancel flash message with dim color
	cancelMsg := fmt.Sprintf("%s%s> cancelled%s\r",
		ui.ResolveColor(ui.ColorDim),
		ui.ResolveStyle(ui.WeightNormal),
		ui.ResolveStyle(ui.WeightNormal), // Reset to normal style
	)
	m.echo(cancelMsg)
	// Brief delay for user feedback
	time.Sleep(50 * time.Millisecond)
	m.clearPrompt()
	m.line = m.line[:0]
	m.composing = false
	m.sw.Resume()
}

// ClearLine removes all text from the input line (Ctrl+U). If the line is
// already empty, cancels composing mode. If not composing, this is a no-op.
func (m *Mode) ClearLine() {
	if !m.composing {
		return
	}
	if len(m.line) == 0 {
		m.clearPrompt()
		m.composing = false
		m.sw.Resume()
		return
	}
	m.line = m.line[:0]
	m.redrawLine()
}

// DeleteWord removes the last word from the input line (Ctrl+W). Skips trailing
// whitespace, then deletes back to the previous whitespace boundary. If the line
// becomes empty, cancels composing mode. If not composing, this is a no-op.
func (m *Mode) DeleteWord() {
	if !m.composing {
		return
	}
	m.line = deleteWord(m.line)
	if len(m.line) == 0 {
		m.clearPrompt()
		m.composing = false
		m.sw.Resume()
		return
	}
	m.redrawLine()
}

// activate transitions from idle to composing mode: pauses output, renders the
// styled input prompt with the first character, and stores the byte.
func (m *Mode) activate(b byte) {
	m.composing = true
	m.line = append(m.line[:0], b)
	m.sw.Pause()
	displayText := m.getDisplayText()
	m.echo(fmt.Sprintf("\r%s%s", promptPrefix, displayText))
}

// echo writes text directly to the underlying writer (bypassing the buffer).
func (m *Mode) echo(s string) {
	//nolint:errcheck // Best-effort echo; terminal write failures are not recoverable.
	m.sw.Direct([]byte(s))
}

// clearPrompt erases the input prompt from the terminal line.
// Accounts for display truncation if line is longer than terminal width.
func (m *Mode) clearPrompt() {
	displayText := m.getDisplayText()
	width := promptPrefixLen + utf8.RuneCount([]byte(displayText))
	m.echo("\r" + strings.Repeat(" ", width) + "\r")
}

// redrawLine clears the current terminal line and redraws the prompt with current text.
// If the line is too long for the terminal, displays truncated text with ellipsis prefix.
func (m *Mode) redrawLine() {
	// \r returns to column 0, \x1b[K clears from cursor to end of line.
	displayText := m.getDisplayText()
	m.echo("\r\x1b[K" + promptPrefix + displayText)
}
