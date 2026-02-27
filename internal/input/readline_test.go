package input

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// Up arrow: ESC [ A
	r := strings.NewReader("a\x1b[Ab\r")
	var w bytes.Buffer

	line, err := ReadLine(r, &w, "snap plan> ")
	require.NoError(t, err)
	assert.Equal(t, "ab", line)

	// No escape bytes should leak to the output writer.
	assert.NotContains(t, w.Bytes(), []byte{0x1b}, "output must not contain ESC bytes")
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
	assert.True(t, strings.HasPrefix(w.String(), "snap plan> "), "output should start with prompt")
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

	output := w.String()
	// Prompt + echoed text + \r\n must all be present.
	assert.True(t, strings.HasPrefix(output, "snap plan> hello"), "output should contain prompt followed by echoed text")
	assert.True(t, strings.HasSuffix(output, "\r\n"), "output should end with \\r\\n")
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
	// Left, Right, Up arrows all consumed
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
