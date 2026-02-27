package input

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/ui"
)

func TestReadLine_BasicSubmit(t *testing.T) {
	r := strings.NewReader("hello\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "hello", line)
}

func TestReadLine_CtrlC_ReturnsErrInterrupt(t *testing.T) {
	r := strings.NewReader("he\x03")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	assert.ErrorIs(t, err, ErrInterrupt)
	assert.Equal(t, "", line)
}

func TestReadLine_Backspace_DeletesLastRune(t *testing.T) {
	r := strings.NewReader("abc\x7f\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "ab", line)
}

func TestReadLine_Backspace_EmptyLine_Noop(t *testing.T) {
	r := strings.NewReader("\x7f\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "", line)
}

func TestReadLine_EscapeSequence_Consumed(t *testing.T) {
	// Up arrow: ESC [ A — should be silently consumed.
	r := strings.NewReader("a\x1b[Ab\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "ab", line)

	// Up arrow should not leak as visible text in output.
	stripped := ui.StripColors(w.String())
	assert.NotContains(t, stripped, "[A", "up arrow should not leak as visible text")
}

func TestReadLine_EmptyEnter(t *testing.T) {
	r := strings.NewReader("\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "", line)
}

func TestReadLine_LongLine_Truncated(t *testing.T) {
	// 4097 'a' bytes + \r — should truncate to 4096
	input := strings.Repeat("a", 4097) + "\r"
	r := strings.NewReader(input)
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, 4096, len(line))
}

func TestReadLine_PromptWrittenToOutput(t *testing.T) {
	r := strings.NewReader("\r")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)

	stripped := ui.StripColors(w.String())
	assert.True(t, strings.HasPrefix(stripped, "snap plan> "), "output should start with prompt")
}

func TestReadLine_NewlineAfterSubmit(t *testing.T) {
	r := strings.NewReader("hi\r")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Contains(t, w.String(), "\r\n")
}

func TestReadLine_TextEchoedToOutput(t *testing.T) {
	r := strings.NewReader("hello\r")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)

	stripped := ui.StripColors(w.String())
	// Prompt + echoed text + \r\n must all be present.
	assert.True(t, strings.HasPrefix(stripped, "snap plan> hello"), "output should contain prompt followed by echoed text")
	assert.True(t, strings.HasSuffix(w.String(), "\r\n"), "output should end with \\r\\n")
}

func TestReadLine_BackspaceMultiByteUTF8(t *testing.T) {
	// Type "café" then backspace (removes 2-byte é), then "e"
	r := strings.NewReader("caf\xc3\xa9\x7fe\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "cafe", line)
}

func TestReadLine_MultipleEscapeSequences(t *testing.T) {
	// Left moves cursor, Right moves cursor, Up consumed as noop.
	r := strings.NewReader("x\x1b[D\x1b[C\x1b[Ay\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "xy", line)
}

func TestReadLine_EOF_ReturnsError(t *testing.T) {
	// Empty reader — immediate EOF
	r := strings.NewReader("")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "> ")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrInterrupt)
}

func TestReadLine_CtrlU_ClearsLine(t *testing.T) {
	// Type "hello", Ctrl+U to clear, then "world"
	r := strings.NewReader("hello\x15world\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "world", line)
}

func TestReadLine_CtrlW_DeletesWord(t *testing.T) {
	// Type "hello world", Ctrl+W to delete "world", then "snap"
	r := strings.NewReader("hello world\x17snap\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "hello snap", line)
}

func TestReadLine_CtrlW_EmptyLine_Noop(t *testing.T) {
	// Ctrl+W on empty line should do nothing
	r := strings.NewReader("\x17\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "", line)
}

func TestReadLine_CtrlU_EmptyLine_Noop(t *testing.T) {
	// Ctrl+U on empty line should do nothing
	r := strings.NewReader("\x15\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "", line)
}

// --- Cursor movement tests (TASK2) ---

func TestReadLine_ArrowLeft_MovesCursor(t *testing.T) {
	// Type "abc", left 2, type "x" → "axbc"
	r := strings.NewReader("abc\x1b[D\x1b[Dx\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "axbc", line)
}

func TestReadLine_ArrowLeft_AtStart_Noop(t *testing.T) {
	// Arrow left on empty line is no-op, then type "a"
	r := strings.NewReader("\x1b[D\x1b[Da\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "a", line)
}

func TestReadLine_ArrowRight_AtEnd_Noop(t *testing.T) {
	// Type "ab", arrow right at end is no-op
	r := strings.NewReader("ab\x1b[C\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "ab", line)
}

func TestReadLine_ArrowRight_MovesForward(t *testing.T) {
	// Type "ab", left then right returns to end, type "x"
	r := strings.NewReader("ab\x1b[D\x1b[Cx\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "abx", line)
}

func TestReadLine_InsertMidLine(t *testing.T) {
	// Type "ab", left 1, type "x" → "axb"
	r := strings.NewReader("ab\x1b[Dx\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "axb", line)
}

func TestReadLine_InsertMidLine_Classic(t *testing.T) {
	// AC-2.3 classic test: type "hello", arrow left 3, type "X" → "heXllo"
	r := strings.NewReader("hello\x1b[D\x1b[D\x1b[DX\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "heXllo", line)
}

func TestReadLine_BackspaceMidLine(t *testing.T) {
	// Type "abc", left 1, backspace → "ac" (deletes 'b')
	r := strings.NewReader("abc\x1b[D\x7f\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "ac", line)
}

func TestReadLine_BackspaceAtStart_Noop(t *testing.T) {
	// Type "ab", left 2 (at start), backspace is no-op
	r := strings.NewReader("ab\x1b[D\x1b[D\x7f\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "ab", line)
}

func TestReadLine_CtrlW_MidLine(t *testing.T) {
	// Type "hello world", left 5 (cursor before 'w'), Ctrl+W deletes "hello "
	r := strings.NewReader("hello world\x1b[D\x1b[D\x1b[D\x1b[D\x1b[D\x17\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "world", line)
}

func TestReadLine_UTF8_CursorMovement(t *testing.T) {
	// Type "café", left 2, type "x" → "caxfé"
	r := strings.NewReader("caf\xc3\xa9\x1b[D\x1b[Dx\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "caxf\xc3\xa9", line) // "caxfé"
}

func TestReadLine_UnknownEscape_WithArrows(t *testing.T) {
	// Type "a", up arrow (consumed), "b", left arrow, "c" → "acb"
	r := strings.NewReader("a\x1b[Ab\x1b[Dc\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "> ")
	require.NoError(t, err)
	assert.Equal(t, "acb", line)
}

func TestReadLine_StyledPrompt(t *testing.T) {
	r := strings.NewReader("\r")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)

	output := w.String()
	// With colors enabled (via TestMain), output should start with ANSI codes.
	expectedPrefix := ui.ResolveStyle(ui.WeightBold) + ui.ResolveColor(ui.ColorSecondary)
	assert.True(t, strings.HasPrefix(output, expectedPrefix),
		"prompt should start with bold + secondary color ANSI codes")
	assert.Contains(t, output, "snap plan> ")
}

func TestReadLine_StyledPrompt_NoColor(t *testing.T) {
	// AC-2.6: When NO_COLOR is set, prompt is plain text (no ANSI codes).
	t.Setenv("NO_COLOR", "1")
	ui.ResetColorMode()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		ui.ResetColorMode()
	})

	r := strings.NewReader("\r")
	var w bytes.Buffer

	_, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)

	output := w.String()
	assert.True(t, strings.HasPrefix(output, "snap plan> "),
		"prompt should be plain text without ANSI codes when NO_COLOR is set")
	assert.NotContains(t, output, "\033[",
		"output should contain no ANSI escape codes when NO_COLOR is set")
}
