package plan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/tap"
)

// requireNonEmpty is the Validate function that will be used in TASK2
// to distinguish empty submit (rejected, user stays in prompt) from
// Ctrl+C/Escape (bypasses Validate, returns "").
func requireNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("enter a message, or /done to finish")
	}
	return nil
}

// TestTapText runs sequential subtests proving the tap.Text integration
// strategy for the planner's interactive input loop.
//
// These tests use tap.SetTermIO (global state) so they must NOT run
// in parallel.
func TestTapText(t *testing.T) {
	t.Run("SubmitWithContent", func(t *testing.T) {
		in := tap.NewMockReadable()
		out := tap.NewMockWritable()
		tap.SetTermIO(in, out)
		defer tap.SetTermIO(nil, nil)

		resultCh := make(chan string, 1)
		go func() {
			resultCh <- tap.Text(context.Background(), tap.TextOptions{
				Message:  "test>",
				Validate: requireNonEmpty,
			})
		}()

		time.Sleep(50 * time.Millisecond)
		for _, ch := range "hello" {
			s := string(ch)
			in.EmitKeypress(s, tap.Key{Name: s})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})

		select {
		case result := <-resultCh:
			assert.Equal(t, "hello", result)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tap.Text to return")
		}
	})

	t.Run("ValidateRejectsEmpty", func(t *testing.T) {
		in := tap.NewMockReadable()
		out := tap.NewMockWritable()
		tap.SetTermIO(in, out)
		defer tap.SetTermIO(nil, nil)

		resultCh := make(chan string, 1)
		go func() {
			resultCh <- tap.Text(context.Background(), tap.TextOptions{
				Message:  "test>",
				Validate: requireNonEmpty,
			})
		}()

		// Press Enter on empty input — Validate rejects, user stays in prompt.
		time.Sleep(50 * time.Millisecond)
		in.EmitKeypress("", tap.Key{Name: "return"})

		// Prompt should still be active. Type content and submit.
		time.Sleep(50 * time.Millisecond)
		for _, ch := range "hello" {
			s := string(ch)
			in.EmitKeypress(s, tap.Key{Name: s})
		}
		in.EmitKeypress("", tap.Key{Name: "return"})

		select {
		case result := <-resultCh:
			assert.Equal(t, "hello", result)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tap.Text to return")
		}
	})

	t.Run("CtrlC_ReturnsEmpty", func(t *testing.T) {
		in := tap.NewMockReadable()
		out := tap.NewMockWritable()
		tap.SetTermIO(in, out)
		defer tap.SetTermIO(nil, nil)

		resultCh := make(chan string, 1)
		go func() {
			resultCh <- tap.Text(context.Background(), tap.TextOptions{
				Message:  "test>",
				Validate: requireNonEmpty,
			})
		}()

		time.Sleep(50 * time.Millisecond)
		in.EmitKeypress("\x03", tap.Key{Name: "c", Ctrl: true})

		select {
		case result := <-resultCh:
			assert.Equal(t, "", result)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tap.Text to return")
		}
	})

	t.Run("Escape_ReturnsEmpty", func(t *testing.T) {
		in := tap.NewMockReadable()
		out := tap.NewMockWritable()
		tap.SetTermIO(in, out)
		defer tap.SetTermIO(nil, nil)

		resultCh := make(chan string, 1)
		go func() {
			resultCh <- tap.Text(context.Background(), tap.TextOptions{
				Message:  "test>",
				Validate: requireNonEmpty,
			})
		}()

		time.Sleep(50 * time.Millisecond)
		in.EmitKeypress("", tap.Key{Name: "escape"})

		select {
		case result := <-resultCh:
			assert.Equal(t, "", result)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for tap.Text to return")
		}
	})

	t.Run("ContextCancel", func(t *testing.T) {
		in := tap.NewMockReadable()
		out := tap.NewMockWritable()
		tap.SetTermIO(in, out)
		defer tap.SetTermIO(nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel before calling Text

		result := tap.Text(ctx, tap.TextOptions{
			Message:  "test>",
			Validate: requireNonEmpty,
		})

		assert.Equal(t, "", result)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	})
}
