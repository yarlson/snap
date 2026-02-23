package input_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yarlson/snap/internal/input"
)

func TestIsTerminal_PipeIsNotTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	assert.False(t, input.IsTerminal(r), "pipe read end should not be a terminal")
	assert.False(t, input.IsTerminal(w), "pipe write end should not be a terminal")
}

func TestIsTerminal_RegularFileIsNotTerminal(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	assert.False(t, input.IsTerminal(f), "regular file should not be a terminal")
}

func TestIsTerminal_ClosedFileReturnsFalse(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	f.Close()

	assert.False(t, input.IsTerminal(f), "closed file should not be a terminal")
}
