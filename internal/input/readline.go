package input

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/yarlson/snap/internal/ui"
)

// ErrInterrupt is returned by ReadLine when the user presses Ctrl+C.
var ErrInterrupt = errors.New("interrupt")

// ReadLine displays a styled prompt on w, reads one line of input from r
// byte-by-byte, and returns the submitted text. It supports full cursor
// movement and editing at any position within the line:
//
//   - Arrow left/right move the cursor within the line
//   - Backspace deletes the character before the cursor (at any position)
//   - Printable characters insert at the cursor position
//   - Enter submits the full line regardless of cursor position
//   - Ctrl+C returns ErrInterrupt
//   - Ctrl+U clears the entire line
//   - Ctrl+W deletes the word before the cursor
//   - Unknown escape sequences (up/down arrow, Home, End, etc.) are consumed
//
// The prompt is styled with ColorSecondary + WeightBold, respecting NO_COLOR.
//
// Note: Display assumes single-width characters. Wide characters (CJK, emoji)
// may render incorrectly, though the internal buffer is correctly modified.
//
// ReadLine is intended for raw-mode terminal input. The caller is responsible
// for entering/exiting raw mode (see WithRawMode).
func ReadLine(r io.Reader, w io.Writer, prompt string) (string, error) {
	styledPrompt := ui.ResolveStyle(ui.WeightBold) +
		ui.ResolveColor(ui.ColorSecondary) + prompt +
		ui.ResolveStyle(ui.WeightNormal)
	fmt.Fprint(w, styledPrompt)

	var line []byte
	var pending []byte
	var pendingLen int
	pos := 0 // cursor position as rune offset
	buf := make([]byte, 1)

	for {
		b, ok := readByte(r, buf)
		if !ok {
			return string(line), io.ErrUnexpectedEOF
		}

		switch b {
		case keyCtrlC:
			return "", ErrInterrupt

		case keyCtrlU:
			if len(line) > 0 {
				line = line[:0]
				pos = 0
				pending = nil
				fmt.Fprintf(w, "\r\x1b[K%s", styledPrompt)
			}

		case keyCtrlW:
			if pos > 0 {
				bytePos := runeToByteOffset(line, pos)
				i := bytePos
				for i > 0 && isSpace(line[i-1]) {
					i--
				}
				for i > 0 && !isSpace(line[i-1]) {
					i--
				}
				deletedRunes := utf8.RuneCount(line[i:bytePos])
				line = append(line[:i], line[bytePos:]...)
				pos -= deletedRunes
				pending = nil
				rlRedrawLine(w, styledPrompt, line, pos)
			}

		case keyEnter:
			fmt.Fprint(w, "\r\n")
			return string(line), nil

		case keyBackspace:
			pending = nil
			if pos > 0 {
				runeCount := utf8.RuneCount(line)
				if pos == runeCount {
					// End of line: simple visual backspace.
					_, size := utf8.DecodeLastRune(line)
					line = line[:len(line)-size]
					pos--
					fmt.Fprint(w, "\b \b")
				} else {
					// Mid-line: remove rune before pos, redraw.
					prevOff := runeToByteOffset(line, pos-1)
					curOff := runeToByteOffset(line, pos)
					line = append(line[:prevOff], line[curOff:]...)
					pos--
					rlRedrawLine(w, styledPrompt, line, pos)
				}
			}

		case keyEsc:
			pending = nil
			rlHandleEscape(r, w, buf, line, &pos)

		default:
			if b < 32 && b != '\t' {
				continue
			}
			if len(line) >= maxLineLen {
				continue
			}

			runeBytes := assembleRune(b, &pending, &pendingLen)
			if runeBytes == nil {
				continue
			}

			if len(line)+len(runeBytes) > maxLineLen {
				continue
			}

			runeCount := utf8.RuneCount(line)
			if pos == runeCount {
				// Append at end of line: simple echo.
				line = append(line, runeBytes...)
				pos++
				//nolint:errcheck // Best-effort echo to terminal.
				w.Write(runeBytes)
			} else {
				// Insert mid-line: splice and redraw.
				bytePos := runeToByteOffset(line, pos)
				line = insertBytesAt(line, bytePos, runeBytes)
				pos++
				rlRedrawLine(w, styledPrompt, line, pos)
			}
		}
	}
}

// rlHandleEscape processes an escape sequence after the initial ESC byte.
// Arrow left (ESC[D) and arrow right (ESC[C) move the cursor. All other
// escape sequences are silently consumed to prevent garbage output.
func rlHandleEscape(r io.Reader, w io.Writer, buf, line []byte, pos *int) {
	b, ok := readByte(r, buf)
	if !ok {
		return
	}
	if b != '[' {
		return // Not a CSI sequence.
	}

	hasParams := false
	for {
		b, ok = readByte(r, buf)
		if !ok {
			return
		}
		if b >= 0x40 && b <= 0x7E {
			// Final byte — only handle simple (no params) arrow keys.
			if !hasParams {
				switch b {
				case 'C': // arrow right
					if *pos < utf8.RuneCount(line) {
						*pos++
						fmt.Fprint(w, "\x1b[C")
					}
				case 'D': // arrow left
					if *pos > 0 {
						*pos--
						fmt.Fprint(w, "\b")
					}
				}
			}
			return
		}
		hasParams = true
	}
}

// assembleRune collects bytes of a multi-byte UTF-8 character, returning
// the complete rune bytes when ready, or nil when still accumulating.
// For single-byte ASCII, returns immediately.
func assembleRune(b byte, pending *[]byte, pendingLen *int) []byte {
	if b < 0x80 {
		*pending = nil
		return []byte{b}
	}
	if b >= 0xC0 {
		// Lead byte of a multi-byte sequence.
		*pending = []byte{b}
		switch {
		case b < 0xE0:
			*pendingLen = 2
		case b < 0xF0:
			*pendingLen = 3
		default:
			*pendingLen = 4
		}
		return nil
	}
	// Continuation byte (0x80..0xBF).
	if len(*pending) > 0 {
		*pending = append(*pending, b)
		if len(*pending) == *pendingLen {
			result := make([]byte, len(*pending))
			copy(result, *pending)
			*pending = nil
			return result
		}
		return nil
	}
	// Orphan continuation byte — discard.
	return nil
}

// runeToByteOffset returns the byte offset of the n-th rune in b.
// If n exceeds the rune count, returns len(b).
func runeToByteOffset(b []byte, n int) int {
	off := 0
	for i := 0; i < n && off < len(b); i++ {
		_, size := utf8.DecodeRune(b[off:])
		off += size
	}
	return off
}

// insertBytesAt inserts data into line at the given byte offset.
func insertBytesAt(line []byte, off int, data []byte) []byte {
	line = append(line, make([]byte, len(data))...)
	copy(line[off+len(data):], line[off:len(line)-len(data)])
	copy(line[off:], data)
	return line
}

// rlRedrawLine clears the terminal line and redraws the styled prompt with
// current text, positioning the cursor at the given rune offset.
func rlRedrawLine(w io.Writer, styledPrompt string, line []byte, pos int) {
	runeCount := utf8.RuneCount(line)
	fmt.Fprintf(w, "\r\x1b[K%s%s", styledPrompt, string(line))
	if tail := runeCount - pos; tail > 0 {
		fmt.Fprintf(w, "\x1b[%dD", tail)
	}
}
