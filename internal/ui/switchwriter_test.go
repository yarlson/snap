package ui_test

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yarlson/snap/internal/ui"
)

func TestSwitchWriter_PassthroughByDefault(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	_, err := sw.Write([]byte("hello"))
	require.NoError(t, err)

	assert.Equal(t, "hello", buf.String())
}

func TestSwitchWriter_BuffersWhenPaused(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Pause()
	_, err := sw.Write([]byte("buffered"))
	require.NoError(t, err)

	assert.Empty(t, buf.String(), "Output should be buffered, not written through")
}

func TestSwitchWriter_FlushWritesBufferedContent(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Pause()
	_, err := sw.Write([]byte("first "))
	require.NoError(t, err)
	_, err = sw.Write([]byte("second"))
	require.NoError(t, err)

	assert.Empty(t, buf.String())

	sw.Resume()

	assert.Equal(t, "first second", buf.String())
}

func TestSwitchWriter_ResumeRestoresPassthrough(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Pause()
	//nolint:errcheck // Intentionally ignoring - testing buffer behavior, not error handling.
	sw.Write([]byte("buffered "))
	sw.Resume()

	_, err := sw.Write([]byte("live"))
	require.NoError(t, err)

	assert.Equal(t, "buffered live", buf.String())
}

func TestSwitchWriter_MultiplePauseResumeCycles(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	//nolint:errcheck // Intentionally ignoring - testing pause/resume cycle, not error handling.
	sw.Write([]byte("A"))
	sw.Pause()
	//nolint:errcheck // Intentionally ignoring - testing pause/resume cycle, not error handling.
	sw.Write([]byte("B"))
	sw.Resume()
	//nolint:errcheck // Intentionally ignoring - testing pause/resume cycle, not error handling.
	sw.Write([]byte("C"))
	sw.Pause()
	//nolint:errcheck // Intentionally ignoring - testing pause/resume cycle, not error handling.
	sw.Write([]byte("D"))
	sw.Resume()

	assert.Equal(t, "ABCD", buf.String())
}

func TestSwitchWriter_ResumeOnAlreadyActiveIsNoop(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Resume() // Should not panic or misbehave.
	_, err := sw.Write([]byte("ok"))
	require.NoError(t, err)

	assert.Equal(t, "ok", buf.String())
}

func TestSwitchWriter_PauseWhenAlreadyPausedIsNoop(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Pause()
	sw.Pause() // Should not panic or lose buffer.
	//nolint:errcheck // Intentionally ignoring - testing double-pause, not error handling.
	sw.Write([]byte("data"))
	sw.Resume()

	assert.Equal(t, "data", buf.String())
}

func TestSwitchWriter_ThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			//nolint:errcheck // Intentionally ignoring - testing thread safety, not error handling.
			sw.Write([]byte("x"))
		}()
	}

	// Interleave pause/resume from another goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			sw.Pause()
			sw.Resume()
		}
	}()

	wg.Wait()

	// All 100 "x" writes should appear somewhere (buffered or direct).
	// Just verify no panic and no data loss.
	assert.Equal(t, 100, len(buf.String()))
}

func TestSwitchWriter_IsPaused(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	assert.False(t, sw.IsPaused())
	sw.Pause()
	assert.True(t, sw.IsPaused())
	sw.Resume()
	assert.False(t, sw.IsPaused())
}

func TestSwitchWriter_ImplementsIOWriter(_ *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	// Must satisfy io.Writer interface.
	var _ io.Writer = sw
}

func TestSwitchWriter_Direct_BypassesBuffer(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	sw.Pause()
	_, err := sw.Direct([]byte("direct write"))
	require.NoError(t, err)

	assert.Equal(t, "direct write", buf.String(), "Direct should bypass buffer even when paused")
}

func TestSwitchWriter_Direct_WorksWhenActive(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf)

	_, err := sw.Direct([]byte("direct"))
	require.NoError(t, err)

	assert.Equal(t, "direct", buf.String())
}

func TestSwitchWriter_NormalizesLF_WhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf, ui.WithLFToCRLF())

	_, err := sw.Write([]byte("line1\nline2\n"))
	require.NoError(t, err)

	assert.Equal(t, "line1\r\nline2\r\n", buf.String())
}

func TestSwitchWriter_NormalizesBufferedContentOnResume_WhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf, ui.WithLFToCRLF())

	sw.Pause()
	_, err := sw.Write([]byte("buffered\noutput\n"))
	require.NoError(t, err)
	assert.Empty(t, buf.String())

	sw.Resume()

	assert.Equal(t, "buffered\r\noutput\r\n", buf.String())
}

func TestSwitchWriter_DirectNormalizesLF_WhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf, ui.WithLFToCRLF())

	_, err := sw.Direct([]byte("direct\nwrite\n"))
	require.NoError(t, err)

	assert.Equal(t, "direct\r\nwrite\r\n", buf.String())
}

func TestSwitchWriter_DoesNotDoubleNormalizeCRLF_WhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf, ui.WithLFToCRLF())

	_, err := sw.Write([]byte("line1\r\nline2\n"))
	require.NoError(t, err)

	assert.Equal(t, "line1\r\nline2\r\n", buf.String())
}

func TestSwitchWriter_CRLFSplitAcrossWrites_WhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	sw := ui.NewSwitchWriter(&buf, ui.WithLFToCRLF())

	// First write ends with \r, second starts with \n — should not double up.
	_, err := sw.Write([]byte("line1\r"))
	require.NoError(t, err)
	_, err = sw.Write([]byte("\nline2\n"))
	require.NoError(t, err)

	assert.Equal(t, "line1\r\nline2\r\n", buf.String())
}
