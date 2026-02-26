package input

import (
	"os"
	"testing"

	"github.com/yarlson/snap/internal/ui"
)

// TestMain resets color mode so that tests expecting ANSI output get
// predictable results regardless of terminal state (e.g. in CI).
func TestMain(m *testing.M) {
	ui.ResetColorMode()
	os.Exit(m.Run())
}
