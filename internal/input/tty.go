package input

import "os"

// IsTerminal reports whether f is connected to a terminal (TTY).
// Returns false for pipes, redirected input, or if Stat fails.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
