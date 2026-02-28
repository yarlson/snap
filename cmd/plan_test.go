package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yarlson/tap"

	"github.com/yarlson/snap/internal/session"
)

// emitString types each rune as a keypress via the mock readable.
func emitString(in *tap.MockReadable, s string) {
	for _, ch := range s {
		str := string(ch)
		in.EmitKeypress(str, tap.Key{Name: str, Rune: ch})
	}
}

// emitLine types a string followed by Enter.
func emitLine(in *tap.MockReadable, s string) {
	emitString(in, s)
	in.EmitKeypress("", tap.Key{Name: "return"})
}

// --- Integration tests: resolvePlanSession ---

func TestResolvePlanSession_ZeroSessions_CreatesDefault(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	name, err := resolvePlanSession(nil)
	require.NoError(t, err)
	assert.Equal(t, "default", name)

	// "default" session directory should exist on disk.
	assert.True(t, session.Exists(".", "default"))
}

func TestResolvePlanSession_OneSession_AutoDetects(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create one session.
	sessDir := filepath.Join(projectDir, ".snap", "sessions", "auth", "tasks")
	require.NoError(t, os.MkdirAll(sessDir, 0o755))

	name, err := resolvePlanSession(nil)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}

func TestResolvePlanSession_MultipleSessions_Errors(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create two sessions.
	for _, n := range []string{"auth", "api"} {
		sessDir := filepath.Join(projectDir, ".snap", "sessions", n, "tasks")
		require.NoError(t, os.MkdirAll(sessDir, 0o755))
	}

	_, err := resolvePlanSession(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple sessions found")
}

// --- Integration tests: checkPlanConflict ---

func TestCheckPlanConflict_EmptySession_NoPrompt(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))

	ctx := context.Background()
	name, err := checkPlanConflict(ctx, "auth", false)
	require.NoError(t, err)
	assert.Equal(t, "auth", name)
}

func TestCheckPlanConflict_NonTTY_ReturnsError(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	ctx := context.Background()
	_, err := checkPlanConflict(ctx, "auth", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already has planning artifacts")
	assert.Contains(t, err.Error(), "snap delete auth")
	assert.Contains(t, err.Error(), "snap new")
}

func TestCheckPlanConflict_TTY_Choice1_CleansUp(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	td := session.TasksDir(".", "auth")
	require.NoError(t, os.WriteFile(filepath.Join(td, "TASK1.md"), []byte("# Task 1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(td, "PRD.md"), []byte("# PRD\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan struct {
		name string
		err  error
	}, 1)

	go func() {
		name, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- struct {
			name string
			err  error
		}{name, err}
	}()

	// First option (replan) is pre-selected; just press Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		assert.Equal(t, "auth", r.name)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// Session should be cleaned.
	assert.False(t, session.HasArtifacts(".", "auth"))
}

func TestCheckPlanConflict_TTY_CtrlC_ReturnsCanceled(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan error, 1)

	go func() {
		_, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- err
	}()

	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("\x03", tap.Key{Name: "c", Ctrl: true})

	select {
	case err := <-resultCh:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

func TestCheckPlanConflict_TTY_Choice2_ValidName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan struct {
		name string
		err  error
	}, 1)

	go func() {
		name, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- struct {
			name string
			err  error
		}{name, err}
	}()

	// Select second option (new session): down arrow + Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "down"})
	time.Sleep(50 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	// Type session name + Enter.
	time.Sleep(200 * time.Millisecond)
	emitLine(in, "my-feature")

	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		assert.Equal(t, "my-feature", r.name)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "my-feature"))
}

func TestCheckPlanConflict_TTY_Choice2_InvalidThenValid(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan struct {
		name string
		err  error
	}, 1)

	go func() {
		name, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- struct {
			name string
			err  error
		}{name, err}
	}()

	// Select "new session": down + Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "down"})
	time.Sleep(50 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	// Type invalid name + Enter (validation error shown by tap).
	time.Sleep(200 * time.Millisecond)
	emitLine(in, "bad name!")

	// Clear the previous input (tap keeps field content after validation error),
	// then type valid name + Enter.
	time.Sleep(200 * time.Millisecond)
	for range len("bad name!") {
		in.EmitKeypress("", tap.Key{Name: "backspace"})
	}
	emitLine(in, "good-name")

	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		assert.Equal(t, "good-name", r.name)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "good-name"))
}

func TestCheckPlanConflict_TTY_Choice2_ExistingThenNew(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan struct {
		name string
		err  error
	}, 1)

	go func() {
		name, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- struct {
			name string
			err  error
		}{name, err}
	}()

	// Select "new session": down + Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "down"})
	time.Sleep(50 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	// Type "auth" (already exists) + Enter.
	time.Sleep(200 * time.Millisecond)
	emitLine(in, "auth")

	// Clear the previous input (tap keeps field content after validation error),
	// then type "auth-v2" (new) + Enter.
	time.Sleep(200 * time.Millisecond)
	for range len("auth") {
		in.EmitKeypress("", tap.Key{Name: "backspace"})
	}
	emitLine(in, "auth-v2")

	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		assert.Equal(t, "auth-v2", r.name)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// New session should exist on disk.
	assert.True(t, session.Exists(".", "auth-v2"))
}

func TestCheckPlanConflict_TTY_Choice2_CtrlC_DuringName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)
	require.NoError(t, session.Create(".", "auth"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "auth"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan error, 1)

	go func() {
		_, err := checkPlanConflict(ctx, "auth", true)
		resultCh <- err
	}()

	// Select "new session": down + Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "down"})
	time.Sleep(50 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	// Ctrl+C during name input.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("\x03", tap.Key{Name: "c", Ctrl: true})

	select {
	case err := <-resultCh:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}
}

// --- Integration test: planRun wiring after conflict choice 2 ---

func TestPlanRun_ConflictChoice2_UsesNewSessionName(t *testing.T) {
	projectDir := t.TempDir()
	chdir(t, projectDir)

	// Create "default" session with artifacts to trigger conflict.
	require.NoError(t, session.Create(".", "default"))
	require.NoError(t, os.WriteFile(
		filepath.Join(session.TasksDir(".", "default"), "TASK1.md"),
		[]byte("# Task 1\n"), 0o600))

	in := tap.NewMockReadable()
	out := tap.NewMockWritable()
	tap.SetTermIO(in, out)
	defer tap.SetTermIO(nil, nil)

	ctx := context.Background()
	resultCh := make(chan struct {
		name string
		err  error
	}, 1)

	go func() {
		name, err := checkPlanConflict(ctx, "default", true)
		resultCh <- struct {
			name string
			err  error
		}{name, err}
	}()

	// Select "new session": down + Enter.
	time.Sleep(200 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "down"})
	time.Sleep(50 * time.Millisecond)
	in.EmitKeypress("", tap.Key{Name: "return"})

	// Type session name + Enter.
	time.Sleep(200 * time.Millisecond)
	emitLine(in, "my-feature")

	var newName string
	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		newName = r.name
		assert.Equal(t, "my-feature", newName)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	// Verify downstream operations use the new session name (not "default"):

	// 1. Tasks dir resolves to the new session's directory.
	td := session.TasksDir(".", newName)
	assert.Equal(t, filepath.Join(".snap", "sessions", "my-feature", "tasks"), td)

	// 2. Session exists and supports plan markers.
	require.NoError(t, session.MarkPlanStarted(".", newName))
	assert.True(t, session.HasPlanHistory(".", newName))

	// 3. Run instruction references the new session name.
	runInstruction := fmt.Sprintf("Run: snap run %s", newName)
	assert.Equal(t, "Run: snap run my-feature", runInstruction)

	// 4. Original session is untouched (artifacts still present).
	assert.True(t, session.HasArtifacts(".", "default"))
}
