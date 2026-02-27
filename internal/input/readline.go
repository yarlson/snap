package input

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// ErrInterrupt is returned by ReadLine when the user presses Ctrl+C.
var ErrInterrupt = errors.New("interrupt")

// ReadLine displays prompt on w, reads one line of input from r byte-by-byte,
// and returns the submitted text. It handles Enter (submit), Ctrl+C (return
// ErrInterrupt), Backspace (delete last rune), printable chars (append), and
// ESC sequences (consume and discard). All editing happens at end of line.
//
// Ctrl+U clears the line. Ctrl+W deletes the last word.
//
// Note: Backspace display assumes single-width characters. Wide characters
// (CJK, emoji) may display incorrectly on the terminal after backspace,
// though the internal buffer is correctly modified.
//
// ReadLine is intended for raw-mode terminal input. The caller is responsible
// for entering/exiting raw mode (see WithRawMode).
func ReadLine(r io.Reader, w io.Writer, prompt string) (string, error) {
	fmt.Fprint(w, prompt)

	var line []byte
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
			line = line[:0]

		case keyCtrlW:
			line = deleteWord(line)

		case keyEnter:
			fmt.Fprint(w, "\r\n")
			return string(line), nil

		case keyBackspace:
			if len(line) > 0 {
				_, size := utf8.DecodeLastRune(line)
				line = line[:len(line)-size]
				// Erase the character visually: move cursor back, write space, move back.
				fmt.Fprint(w, "\b \b")
			}

		case keyEsc:
			consumeEscape(r, buf)

		default:
			if (b >= 32 || b == '\t') && len(line) < maxLineLen {
				line = append(line, b)
				//nolint:errcheck // Best-effort single-byte echo to terminal.
				w.Write([]byte{b})
			}
		}
	}
}
