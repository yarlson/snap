package input

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// WithRawMode enters raw terminal mode on fd, executes fn, and guarantees the
// terminal is restored on return — whether fn returns normally, returns an
// error, or panics. A signal handler restores the terminal on SIGINT/SIGTERM.
func WithRawMode(fd int, fn func() error) error {
	oldState, err := termMakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}

	// Signal handler: restore terminal on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	restore := func() {
		//nolint:errcheck // Best-effort terminal restore; nothing to do on failure.
		termRestore(fd, oldState)
	}

	// Restore terminal on panic before re-panicking.
	defer func() {
		if r := recover(); r != nil {
			restore()
			panic(r)
		}
	}()

	defer restore()

	return fn()
}
